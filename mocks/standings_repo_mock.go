package mocks

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/Anill17/league-api/internal/domain"
)

type StandingsRepoMock struct {
	GetBySeasonFunc         func(ctx context.Context, seasonID int) ([]domain.Standing, error)
	GetBySeasonWithTeamFunc func(ctx context.Context, seasonID int) ([]domain.StandingWithTeam, error)
	InitForSeasonFunc       func(ctx context.Context, tx pgx.Tx, seasonID int, teamIDs []int) error
	UpsertFunc              func(ctx context.Context, tx pgx.Tx, s *domain.Standing) error
	BulkReplaceFunc         func(ctx context.Context, tx pgx.Tx, seasonID int, rows []domain.Standing) error
	DeleteForSeasonFunc     func(ctx context.Context, tx pgx.Tx, seasonID int) error

	GetBySeasonCalls         int
	GetBySeasonWithTeamCalls int
	InitForSeasonCalls       int
	UpsertCalls              int
	BulkReplaceCalls         int
	DeleteForSeasonCalls     int

	// LastBulkReplace records the rows passed to BulkReplace for
	// post-hoc assertions.
	LastBulkReplace []domain.Standing
}

var _ domain.StandingsRepository = (*StandingsRepoMock)(nil)

func (m *StandingsRepoMock) GetBySeason(ctx context.Context, seasonID int) ([]domain.Standing, error) {
	m.GetBySeasonCalls++
	if m.GetBySeasonFunc != nil {
		return m.GetBySeasonFunc(ctx, seasonID)
	}
	return nil, nil
}

func (m *StandingsRepoMock) GetBySeasonWithTeam(ctx context.Context, seasonID int) ([]domain.StandingWithTeam, error) {
	m.GetBySeasonWithTeamCalls++
	if m.GetBySeasonWithTeamFunc != nil {
		return m.GetBySeasonWithTeamFunc(ctx, seasonID)
	}
	return nil, nil
}

func (m *StandingsRepoMock) InitForSeason(ctx context.Context, tx pgx.Tx, seasonID int, teamIDs []int) error {
	m.InitForSeasonCalls++
	if m.InitForSeasonFunc != nil {
		return m.InitForSeasonFunc(ctx, tx, seasonID, teamIDs)
	}
	return nil
}

func (m *StandingsRepoMock) Upsert(ctx context.Context, tx pgx.Tx, s *domain.Standing) error {
	m.UpsertCalls++
	if m.UpsertFunc != nil {
		return m.UpsertFunc(ctx, tx, s)
	}
	return nil
}

func (m *StandingsRepoMock) BulkReplace(ctx context.Context, tx pgx.Tx, seasonID int, rows []domain.Standing) error {
	m.BulkReplaceCalls++
	m.LastBulkReplace = append([]domain.Standing(nil), rows...)
	if m.BulkReplaceFunc != nil {
		return m.BulkReplaceFunc(ctx, tx, seasonID, rows)
	}
	return nil
}

func (m *StandingsRepoMock) DeleteForSeason(ctx context.Context, tx pgx.Tx, seasonID int) error {
	m.DeleteForSeasonCalls++
	if m.DeleteForSeasonFunc != nil {
		return m.DeleteForSeasonFunc(ctx, tx, seasonID)
	}
	return nil
}
