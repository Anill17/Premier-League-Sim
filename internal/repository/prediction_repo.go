package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/insider/league-api/internal/domain"
)

// PredictionRepo persists championship predictions per (season, week).
type PredictionRepo struct {
	pool *pgxpool.Pool
}

func NewPredictionRepo(pool *pgxpool.Pool) *PredictionRepo {
	return &PredictionRepo{pool: pool}
}

var _ domain.PredictionRepository = (*PredictionRepo)(nil)

// GetLatest returns the most recent set of predictions for a season.
// "Most recent" is defined as MAX(week) for the season.
func (r *PredictionRepo) GetLatest(
	ctx context.Context,
	seasonID int,
) ([]domain.PredictionWithTeam, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT p.id, p.season_id, p.week, p.team_id,
		        p.championship_probability, p.created_at, t.name
		   FROM predictions p
		   JOIN teams t ON t.id = p.team_id
		  WHERE p.season_id = $1
		    AND p.week = (
		        SELECT MAX(week) FROM predictions WHERE season_id = $1
		    )
		  ORDER BY p.championship_probability DESC`,
		seasonID,
	)
	if err != nil {
		return nil, fmt.Errorf("query latest predictions: %w", err)
	}
	return collectPredictionsWithTeam(rows)
}

// GetByWeek returns the prediction snapshot for a specific week.
func (r *PredictionRepo) GetByWeek(
	ctx context.Context,
	seasonID, week int,
) ([]domain.PredictionWithTeam, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT p.id, p.season_id, p.week, p.team_id,
		        p.championship_probability, p.created_at, t.name
		   FROM predictions p
		   JOIN teams t ON t.id = p.team_id
		  WHERE p.season_id = $1 AND p.week = $2
		  ORDER BY p.championship_probability DESC`,
		seasonID, week,
	)
	if err != nil {
		return nil, fmt.Errorf("query predictions for week %d: %w", week, err)
	}
	return collectPredictionsWithTeam(rows)
}

// DeleteForWeek wipes the existing snapshot for (season, week). Used
// before inserting a fresh snapshot when EditFixture re-runs the
// predictor for the current week.
func (r *PredictionRepo) DeleteForWeek(
	ctx context.Context,
	tx pgx.Tx,
	seasonID, week int,
) error {
	_, err := tx.Exec(ctx,
		`DELETE FROM predictions WHERE season_id = $1 AND week = $2`,
		seasonID, week,
	)
	if err != nil {
		return fmt.Errorf("delete predictions: %w", err)
	}
	return nil
}

// DeleteForSeason removes every prediction row for a season.
func (r *PredictionRepo) DeleteForSeason(
	ctx context.Context,
	tx pgx.Tx,
	seasonID int,
) error {
	if _, err := tx.Exec(ctx,
		`DELETE FROM predictions WHERE season_id = $1`,
		seasonID,
	); err != nil {
		return fmt.Errorf("delete predictions for season %d: %w", seasonID, err)
	}
	return nil
}

// BulkInsert writes a fresh snapshot for a (season, week) tuple.
func (r *PredictionRepo) BulkInsert(
	ctx context.Context,
	tx pgx.Tx,
	predictions []domain.Prediction,
) error {
	if len(predictions) == 0 {
		return nil
	}
	batch := &pgx.Batch{}
	for _, p := range predictions {
		batch.Queue(
			`INSERT INTO predictions
			    (season_id, week, team_id, championship_probability)
			 VALUES ($1, $2, $3, $4)`,
			p.SeasonID, p.Week, p.TeamID, p.ChampionshipProbability,
		)
	}
	br := tx.SendBatch(ctx, batch)
	defer br.Close()
	for range predictions {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("bulk insert predictions: %w", err)
		}
	}
	return nil
}

func collectPredictionsWithTeam(rows pgx.Rows) ([]domain.PredictionWithTeam, error) {
	defer rows.Close()
	var out []domain.PredictionWithTeam
	for rows.Next() {
		var p domain.PredictionWithTeam
		if err := rows.Scan(
			&p.ID, &p.SeasonID, &p.Week, &p.TeamID,
			&p.ChampionshipProbability, &p.CreatedAt, &p.TeamName,
		); err != nil {
			return nil, fmt.Errorf("scan prediction: %w", err)
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate predictions: %w", err)
	}
	return out, nil
}
