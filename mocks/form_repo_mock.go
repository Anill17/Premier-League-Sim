package mocks

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/insider/league-api/internal/domain"
)

type FormRepoMock struct {
	GetBySeasonAndTeamFunc func(ctx context.Context, seasonID, teamID int) (*domain.TeamForm, error)
	GetBySeasonFunc        func(ctx context.Context, seasonID int) ([]domain.TeamForm, error)
	InitForSeasonFunc      func(ctx context.Context, tx pgx.Tx, seasonID int, teamIDs []int) error
	UpsertFunc             func(ctx context.Context, tx pgx.Tx, f *domain.TeamForm) error
	DeleteForSeasonFunc    func(ctx context.Context, tx pgx.Tx, seasonID int) error

	GetBySeasonAndTeamCalls int
	GetBySeasonCalls        int
	InitForSeasonCalls      int
	UpsertCalls             int
	DeleteForSeasonCalls    int
}

var _ domain.FormRepository = (*FormRepoMock)(nil)

func (m *FormRepoMock) GetBySeasonAndTeam(ctx context.Context, seasonID, teamID int) (*domain.TeamForm, error) {
	m.GetBySeasonAndTeamCalls++
	if m.GetBySeasonAndTeamFunc != nil {
		return m.GetBySeasonAndTeamFunc(ctx, seasonID, teamID)
	}
	return nil, nil
}

func (m *FormRepoMock) GetBySeason(ctx context.Context, seasonID int) ([]domain.TeamForm, error) {
	m.GetBySeasonCalls++
	if m.GetBySeasonFunc != nil {
		return m.GetBySeasonFunc(ctx, seasonID)
	}
	return nil, nil
}

func (m *FormRepoMock) InitForSeason(ctx context.Context, tx pgx.Tx, seasonID int, teamIDs []int) error {
	m.InitForSeasonCalls++
	if m.InitForSeasonFunc != nil {
		return m.InitForSeasonFunc(ctx, tx, seasonID, teamIDs)
	}
	return nil
}

func (m *FormRepoMock) Upsert(ctx context.Context, tx pgx.Tx, f *domain.TeamForm) error {
	m.UpsertCalls++
	if m.UpsertFunc != nil {
		return m.UpsertFunc(ctx, tx, f)
	}
	return nil
}

func (m *FormRepoMock) DeleteForSeason(ctx context.Context, tx pgx.Tx, seasonID int) error {
	m.DeleteForSeasonCalls++
	if m.DeleteForSeasonFunc != nil {
		return m.DeleteForSeasonFunc(ctx, tx, seasonID)
	}
	return nil
}
