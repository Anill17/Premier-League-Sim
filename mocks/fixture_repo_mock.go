package mocks

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/Anill17/league-api/internal/domain"
)

type FixtureRepoMock struct {
	GetByIDFunc            func(ctx context.Context, id int) (*domain.Fixture, error)
	GetBySeasonAndWeekFunc func(ctx context.Context, seasonID, week int) ([]domain.Fixture, error)
	GetBySeasonFunc        func(ctx context.Context, seasonID int) ([]domain.Fixture, error)
	GetPlayedFunc          func(ctx context.Context, seasonID int) ([]domain.Fixture, error)
	GetRemainingFunc       func(ctx context.Context, seasonID int) ([]domain.Fixture, error)
	CreateFunc             func(ctx context.Context, tx pgx.Tx, f *domain.Fixture) error
	UpdateFunc             func(ctx context.Context, tx pgx.Tx, f *domain.Fixture) error
	BulkCreateFunc         func(ctx context.Context, tx pgx.Tx, fixtures []domain.Fixture) error
	DeleteForSeasonFunc    func(ctx context.Context, tx pgx.Tx, seasonID int) error
	LockForWeekFunc        func(ctx context.Context, tx pgx.Tx, seasonID, week int) ([]domain.Fixture, error)
	LockByIDFunc           func(ctx context.Context, tx pgx.Tx, id int) (*domain.Fixture, error)

	GetByIDCalls            int
	GetBySeasonAndWeekCalls int
	GetBySeasonCalls        int
	GetPlayedCalls          int
	GetRemainingCalls       int
	CreateCalls             int
	UpdateCalls             int
	BulkCreateCalls         int
	DeleteForSeasonCalls    int
	LockForWeekCalls        int
	LockByIDCalls           int
}

var _ domain.FixtureRepository = (*FixtureRepoMock)(nil)

func (m *FixtureRepoMock) GetByID(ctx context.Context, id int) (*domain.Fixture, error) {
	m.GetByIDCalls++
	if m.GetByIDFunc != nil {
		return m.GetByIDFunc(ctx, id)
	}
	return nil, nil
}

func (m *FixtureRepoMock) GetBySeasonAndWeek(ctx context.Context, seasonID, week int) ([]domain.Fixture, error) {
	m.GetBySeasonAndWeekCalls++
	if m.GetBySeasonAndWeekFunc != nil {
		return m.GetBySeasonAndWeekFunc(ctx, seasonID, week)
	}
	return nil, nil
}

func (m *FixtureRepoMock) GetBySeason(ctx context.Context, seasonID int) ([]domain.Fixture, error) {
	m.GetBySeasonCalls++
	if m.GetBySeasonFunc != nil {
		return m.GetBySeasonFunc(ctx, seasonID)
	}
	return nil, nil
}

func (m *FixtureRepoMock) GetPlayed(ctx context.Context, seasonID int) ([]domain.Fixture, error) {
	m.GetPlayedCalls++
	if m.GetPlayedFunc != nil {
		return m.GetPlayedFunc(ctx, seasonID)
	}
	return nil, nil
}

func (m *FixtureRepoMock) GetRemaining(ctx context.Context, seasonID int) ([]domain.Fixture, error) {
	m.GetRemainingCalls++
	if m.GetRemainingFunc != nil {
		return m.GetRemainingFunc(ctx, seasonID)
	}
	return nil, nil
}

func (m *FixtureRepoMock) Create(ctx context.Context, tx pgx.Tx, f *domain.Fixture) error {
	m.CreateCalls++
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, tx, f)
	}
	return nil
}

func (m *FixtureRepoMock) Update(ctx context.Context, tx pgx.Tx, f *domain.Fixture) error {
	m.UpdateCalls++
	if m.UpdateFunc != nil {
		return m.UpdateFunc(ctx, tx, f)
	}
	return nil
}

func (m *FixtureRepoMock) BulkCreate(ctx context.Context, tx pgx.Tx, fixtures []domain.Fixture) error {
	m.BulkCreateCalls++
	if m.BulkCreateFunc != nil {
		return m.BulkCreateFunc(ctx, tx, fixtures)
	}
	return nil
}

func (m *FixtureRepoMock) DeleteForSeason(ctx context.Context, tx pgx.Tx, seasonID int) error {
	m.DeleteForSeasonCalls++
	if m.DeleteForSeasonFunc != nil {
		return m.DeleteForSeasonFunc(ctx, tx, seasonID)
	}
	return nil
}

func (m *FixtureRepoMock) LockForWeek(ctx context.Context, tx pgx.Tx, seasonID, week int) ([]domain.Fixture, error) {
	m.LockForWeekCalls++
	if m.LockForWeekFunc != nil {
		return m.LockForWeekFunc(ctx, tx, seasonID, week)
	}
	return nil, nil
}

func (m *FixtureRepoMock) LockByID(ctx context.Context, tx pgx.Tx, id int) (*domain.Fixture, error) {
	m.LockByIDCalls++
	if m.LockByIDFunc != nil {
		return m.LockByIDFunc(ctx, tx, id)
	}
	return nil, nil
}
