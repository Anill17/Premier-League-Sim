package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/insider/league-api/internal/domain"
)

// StandingsRepo persists league-table rows. The canonical sort order
// (points DESC, goal_difference DESC, goals_for DESC) is centralised
// here in standingsOrderBy — never inlined elsewhere.
type StandingsRepo struct {
	pool *pgxpool.Pool
}

func NewStandingsRepo(pool *pgxpool.Pool) *StandingsRepo {
	return &StandingsRepo{pool: pool}
}

var _ domain.StandingsRepository = (*StandingsRepo)(nil)

// standingsOrderBy is the canonical league-table ordering clause.
// Head-to-head (the final PL tiebreaker) is applied in Go after the
// initial SQL sort because it requires inspecting played fixtures —
// see service/standings_service.go.
const standingsOrderBy = `ORDER BY points DESC, goal_difference DESC, goals_for DESC`

const selectStandingColumns = `
	id, season_id, team_id, played, won, drawn, lost,
	goals_for, goals_against, goal_difference, points
`

// GetBySeason returns the standings rows in canonical SQL order
// (without the head-to-head tiebreaker applied).
func (r *StandingsRepo) GetBySeason(
	ctx context.Context,
	seasonID int,
) ([]domain.Standing, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+selectStandingColumns+`
		   FROM standings
		  WHERE season_id = $1
		  `+standingsOrderBy,
		seasonID,
	)
	if err != nil {
		return nil, fmt.Errorf("query standings: %w", err)
	}
	defer rows.Close()

	var out []domain.Standing
	for rows.Next() {
		s, err := scanStanding(rows)
		if err != nil {
			return nil, fmt.Errorf("scan standing: %w", err)
		}
		out = append(out, *s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate standings: %w", err)
	}
	return out, nil
}

// GetBySeasonWithTeam joins standings with teams so handlers can render
// a human-friendly table without a second round-trip.
func (r *StandingsRepo) GetBySeasonWithTeam(
	ctx context.Context,
	seasonID int,
) ([]domain.StandingWithTeam, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT s.id, s.season_id, s.team_id, s.played, s.won, s.drawn, s.lost,
		        s.goals_for, s.goals_against, s.goal_difference, s.points,
		        t.name
		   FROM standings s
		   JOIN teams t ON t.id = s.team_id
		  WHERE s.season_id = $1
		  `+standingsOrderBy,
		seasonID,
	)
	if err != nil {
		return nil, fmt.Errorf("query standings with team: %w", err)
	}
	defer rows.Close()

	var out []domain.StandingWithTeam
	for rows.Next() {
		var row domain.StandingWithTeam
		if err := rows.Scan(
			&row.ID, &row.SeasonID, &row.TeamID, &row.Played, &row.Won,
			&row.Drawn, &row.Lost, &row.GoalsFor, &row.GoalsAgainst,
			&row.GoalDifference, &row.Points, &row.TeamName,
		); err != nil {
			return nil, fmt.Errorf("scan standing with team: %w", err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate standings with team: %w", err)
	}
	return out, nil
}

// InitForSeason creates one zeroed standings row per team for a fresh
// season.
func (r *StandingsRepo) InitForSeason(
	ctx context.Context,
	tx pgx.Tx,
	seasonID int,
	teamIDs []int,
) error {
	if len(teamIDs) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, tid := range teamIDs {
		batch.Queue(
			`INSERT INTO standings (season_id, team_id) VALUES ($1, $2)
			 ON CONFLICT (season_id, team_id) DO NOTHING`,
			seasonID, tid,
		)
	}
	br := tx.SendBatch(ctx, batch)
	defer br.Close()
	for range teamIDs {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("init standings: %w", err)
		}
	}
	return nil
}

// Upsert writes a single row, inserting on first sight.
func (r *StandingsRepo) Upsert(
	ctx context.Context,
	tx pgx.Tx,
	s *domain.Standing,
) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO standings
		    (season_id, team_id, played, won, drawn, lost,
		     goals_for, goals_against, points)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 ON CONFLICT (season_id, team_id) DO UPDATE
		    SET played        = EXCLUDED.played,
		        won           = EXCLUDED.won,
		        drawn         = EXCLUDED.drawn,
		        lost          = EXCLUDED.lost,
		        goals_for     = EXCLUDED.goals_for,
		        goals_against = EXCLUDED.goals_against,
		        points        = EXCLUDED.points`,
		s.SeasonID, s.TeamID, s.Played, s.Won, s.Drawn, s.Lost,
		s.GoalsFor, s.GoalsAgainst, s.Points,
	)
	if err != nil {
		return fmt.Errorf("upsert standing (season=%d team=%d): %w",
			s.SeasonID, s.TeamID, err)
	}
	return nil
}

// DeleteForSeason removes every standings row for a season.
func (r *StandingsRepo) DeleteForSeason(
	ctx context.Context,
	tx pgx.Tx,
	seasonID int,
) error {
	if _, err := tx.Exec(ctx,
		`DELETE FROM standings WHERE season_id = $1`,
		seasonID,
	); err != nil {
		return fmt.Errorf("delete standings for season %d: %w", seasonID, err)
	}
	return nil
}

// BulkReplace overwrites every standings row for a season in one tx.
// Used by EditFixture's "recalculate from scratch" flow.
func (r *StandingsRepo) BulkReplace(
	ctx context.Context,
	tx pgx.Tx,
	seasonID int,
	rows []domain.Standing,
) error {
	batch := &pgx.Batch{}
	for _, s := range rows {
		batch.Queue(
			`INSERT INTO standings
			    (season_id, team_id, played, won, drawn, lost,
			     goals_for, goals_against, points)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			 ON CONFLICT (season_id, team_id) DO UPDATE
			    SET played        = EXCLUDED.played,
			        won           = EXCLUDED.won,
			        drawn         = EXCLUDED.drawn,
			        lost          = EXCLUDED.lost,
			        goals_for     = EXCLUDED.goals_for,
			        goals_against = EXCLUDED.goals_against,
			        points        = EXCLUDED.points`,
			seasonID, s.TeamID, s.Played, s.Won, s.Drawn, s.Lost,
			s.GoalsFor, s.GoalsAgainst, s.Points,
		)
	}
	br := tx.SendBatch(ctx, batch)
	defer br.Close()
	for range rows {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("bulk replace standings: %w", err)
		}
	}
	return nil
}

func scanStanding(r rowScanner) (*domain.Standing, error) {
	var s domain.Standing
	if err := r.Scan(
		&s.ID, &s.SeasonID, &s.TeamID, &s.Played, &s.Won, &s.Drawn,
		&s.Lost, &s.GoalsFor, &s.GoalsAgainst, &s.GoalDifference, &s.Points,
	); err != nil {
		return nil, err
	}
	return &s, nil
}
