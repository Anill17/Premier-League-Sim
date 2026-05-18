package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/insider/league-api/internal/domain"
)

// SeasonRepo handles persistence of seasons. Writes accept pgx.Tx so
// they participate in the caller's transaction (ACID requirement).
type SeasonRepo struct {
	pool *pgxpool.Pool
}

func NewSeasonRepo(pool *pgxpool.Pool) *SeasonRepo {
	return &SeasonRepo{pool: pool}
}

var _ domain.SeasonRepository = (*SeasonRepo)(nil)

const selectSeasonColumns = `id, current_week, is_complete, created_at`

func (r *SeasonRepo) GetByID(ctx context.Context, id int) (*domain.Season, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+selectSeasonColumns+` FROM seasons WHERE id = $1`,
		id,
	)
	s, err := scanSeason(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("season %d: %w", id, ErrNotFound)
		}
		return nil, fmt.Errorf("get season %d: %w", id, err)
	}
	return s, nil
}

// Create inserts a new season row with defaults (current_week=0,
// is_complete=false) and returns the populated Season.
func (r *SeasonRepo) Create(ctx context.Context, tx pgx.Tx) (*domain.Season, error) {
	row := tx.QueryRow(ctx,
		`INSERT INTO seasons DEFAULT VALUES RETURNING `+selectSeasonColumns,
	)
	s, err := scanSeason(row)
	if err != nil {
		return nil, fmt.Errorf("create season: %w", err)
	}
	return s, nil
}

// UpdateCurrentWeek bumps current_week and, when week == 6, also flips
// is_complete to true. Single statement keeps the operation atomic
// within the caller's tx.
func (r *SeasonRepo) UpdateCurrentWeek(
	ctx context.Context,
	tx pgx.Tx,
	seasonID, week int,
) error {
	_, err := tx.Exec(ctx,
		`UPDATE seasons
		    SET current_week = $2,
		        is_complete  = CASE WHEN $2 >= 6 THEN TRUE ELSE is_complete END
		  WHERE id = $1`,
		seasonID, week,
	)
	if err != nil {
		return fmt.Errorf("update season current_week: %w", err)
	}
	return nil
}

func (r *SeasonRepo) MarkComplete(ctx context.Context, tx pgx.Tx, seasonID int) error {
	_, err := tx.Exec(ctx,
		`UPDATE seasons SET is_complete = TRUE WHERE id = $1`,
		seasonID,
	)
	if err != nil {
		return fmt.Errorf("mark season complete: %w", err)
	}
	return nil
}

// Reset clears the season row's progress fields. Dependent tables are
// cleared by the corresponding repo's DeleteForSeason; this method
// only touches the seasons row itself.
func (r *SeasonRepo) Reset(ctx context.Context, tx pgx.Tx, seasonID int) error {
	_, err := tx.Exec(ctx,
		`UPDATE seasons SET current_week = 0, is_complete = FALSE WHERE id = $1`,
		seasonID,
	)
	if err != nil {
		return fmt.Errorf("reset season state: %w", err)
	}
	return nil
}

// Delete removes the season row. ON DELETE CASCADE on fixtures /
// standings / form / predictions takes care of the rest.
func (r *SeasonRepo) Delete(ctx context.Context, tx pgx.Tx, seasonID int) error {
	tag, err := tx.Exec(ctx, `DELETE FROM seasons WHERE id = $1`, seasonID)
	if err != nil {
		return fmt.Errorf("delete season: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("season %d: %w", seasonID, ErrNotFound)
	}
	return nil
}

func scanSeason(r rowScanner) (*domain.Season, error) {
	var s domain.Season
	if err := r.Scan(&s.ID, &s.CurrentWeek, &s.IsComplete, &s.CreatedAt); err != nil {
		return nil, err
	}
	return &s, nil
}
