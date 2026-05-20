package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Anill17/league-api/internal/domain"
)

// TeamRepo is the read-only team repository. Teams are seeded once via
// 002_seed.sql and never mutated by application code.
type TeamRepo struct {
	pool *pgxpool.Pool
}

func NewTeamRepo(pool *pgxpool.Pool) *TeamRepo {
	return &TeamRepo{pool: pool}
}

var _ domain.TeamRepository = (*TeamRepo)(nil)

const selectTeamColumns = `
	id, name, attack, defense, midfield, home_advantage, strength
`

// GetByID returns one team. Returns a "team not found" error if no row
// matches the id.
func (r *TeamRepo) GetByID(ctx context.Context, id int) (*domain.Team, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT `+selectTeamColumns+` FROM teams WHERE id = $1`,
		id,
	)
	t, err := scanTeam(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("team %d: %w", id, ErrNotFound)
		}
		return nil, fmt.Errorf("get team %d: %w", id, err)
	}
	return t, nil
}

// List returns every team ordered by id. The league has exactly four
// teams, so we never paginate.
func (r *TeamRepo) List(ctx context.Context) ([]domain.Team, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+selectTeamColumns+` FROM teams ORDER BY id`,
	)
	if err != nil {
		return nil, fmt.Errorf("list teams: %w", err)
	}
	defer rows.Close()

	var out []domain.Team
	for rows.Next() {
		t, err := scanTeam(rows)
		if err != nil {
			return nil, fmt.Errorf("scan team: %w", err)
		}
		out = append(out, *t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate teams: %w", err)
	}
	return out, nil
}

// rowScanner is satisfied by both pgx.Row and pgx.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanTeam(r rowScanner) (*domain.Team, error) {
	var t domain.Team
	if err := r.Scan(
		&t.ID, &t.Name, &t.Attack, &t.Defense,
		&t.Midfield, &t.HomeAdvantage, &t.Strength,
	); err != nil {
		return nil, err
	}
	return &t, nil
}
