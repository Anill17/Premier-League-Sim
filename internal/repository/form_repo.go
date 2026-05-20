package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Anill17/league-api/internal/domain"
)

// FormRepo persists team form (last N results) as JSONB.
type FormRepo struct {
	pool *pgxpool.Pool
}

func NewFormRepo(pool *pgxpool.Pool) *FormRepo {
	return &FormRepo{pool: pool}
}

var _ domain.FormRepository = (*FormRepo)(nil)

func (r *FormRepo) GetBySeasonAndTeam(
	ctx context.Context,
	seasonID, teamID int,
) (*domain.TeamForm, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, season_id, team_id, recent_results
		   FROM team_form
		  WHERE season_id = $1 AND team_id = $2`,
		seasonID, teamID,
	)
	f, err := scanForm(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("form (season=%d team=%d): %w",
				seasonID, teamID, ErrNotFound)
		}
		return nil, fmt.Errorf("get team form: %w", err)
	}
	return f, nil
}

func (r *FormRepo) GetBySeason(
	ctx context.Context,
	seasonID int,
) ([]domain.TeamForm, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, season_id, team_id, recent_results
		   FROM team_form
		  WHERE season_id = $1
		  ORDER BY team_id`,
		seasonID,
	)
	if err != nil {
		return nil, fmt.Errorf("query forms: %w", err)
	}
	defer rows.Close()

	var out []domain.TeamForm
	for rows.Next() {
		f, err := scanForm(rows)
		if err != nil {
			return nil, fmt.Errorf("scan form: %w", err)
		}
		out = append(out, *f)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate forms: %w", err)
	}
	return out, nil
}

func (r *FormRepo) InitForSeason(
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
			`INSERT INTO team_form (season_id, team_id, recent_results)
			 VALUES ($1, $2, '[]'::jsonb)
			 ON CONFLICT (season_id, team_id) DO NOTHING`,
			seasonID, tid,
		)
	}
	br := tx.SendBatch(ctx, batch)
	defer br.Close()
	for range teamIDs {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("init form: %w", err)
		}
	}
	return nil
}

// DeleteForSeason removes every team_form row for a season.
func (r *FormRepo) DeleteForSeason(
	ctx context.Context,
	tx pgx.Tx,
	seasonID int,
) error {
	if _, err := tx.Exec(ctx,
		`DELETE FROM team_form WHERE season_id = $1`,
		seasonID,
	); err != nil {
		return fmt.Errorf("delete team_form for season %d: %w", seasonID, err)
	}
	return nil
}

func (r *FormRepo) Upsert(ctx context.Context, tx pgx.Tx, f *domain.TeamForm) error {
	encoded, err := json.Marshal(f.RecentResults)
	if err != nil {
		return fmt.Errorf("marshal recent_results: %w", err)
	}
	_, err = tx.Exec(ctx,
		`INSERT INTO team_form (season_id, team_id, recent_results)
		 VALUES ($1, $2, $3::jsonb)
		 ON CONFLICT (season_id, team_id) DO UPDATE
		    SET recent_results = EXCLUDED.recent_results`,
		f.SeasonID, f.TeamID, string(encoded),
	)
	if err != nil {
		return fmt.Errorf("upsert form (season=%d team=%d): %w",
			f.SeasonID, f.TeamID, err)
	}
	return nil
}

func scanForm(r rowScanner) (*domain.TeamForm, error) {
	var f domain.TeamForm
	var raw []byte
	if err := r.Scan(&f.ID, &f.SeasonID, &f.TeamID, &raw); err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		f.RecentResults = []string{}
		return &f, nil
	}
	if err := json.Unmarshal(raw, &f.RecentResults); err != nil {
		return nil, fmt.Errorf("decode recent_results: %w", err)
	}
	return &f, nil
}
