package mocks

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/Anill17/league-api/internal/domain"
)

type SeasonRepoMock struct {
	GetByIDFunc           func(ctx context.Context, id int) (*domain.Season, error)
	CreateFunc            func(ctx context.Context, tx pgx.Tx) (*domain.Season, error)
	UpdateCurrentWeekFunc func(ctx context.Context, tx pgx.Tx, seasonID, week int) error
	MarkCompleteFunc      func(ctx context.Context, tx pgx.Tx, seasonID int) error
	ResetFunc             func(ctx context.Context, tx pgx.Tx, seasonID int) error
	DeleteFunc            func(ctx context.Context, tx pgx.Tx, seasonID int) error

	GetByIDCalls           int
	CreateCalls            int
	UpdateCurrentWeekCalls int
	MarkCompleteCalls      int
	ResetCalls             int
	DeleteCalls            int
}

var _ domain.SeasonRepository = (*SeasonRepoMock)(nil)

func (m *SeasonRepoMock) GetByID(ctx context.Context, id int) (*domain.Season, error) {
	m.GetByIDCalls++
	if m.GetByIDFunc != nil {
		return m.GetByIDFunc(ctx, id)
	}
	return nil, nil
}

func (m *SeasonRepoMock) Create(ctx context.Context, tx pgx.Tx) (*domain.Season, error) {
	m.CreateCalls++
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, tx)
	}
	return nil, nil
}

func (m *SeasonRepoMock) UpdateCurrentWeek(ctx context.Context, tx pgx.Tx, seasonID, week int) error {
	m.UpdateCurrentWeekCalls++
	if m.UpdateCurrentWeekFunc != nil {
		return m.UpdateCurrentWeekFunc(ctx, tx, seasonID, week)
	}
	return nil
}

func (m *SeasonRepoMock) MarkComplete(ctx context.Context, tx pgx.Tx, seasonID int) error {
	m.MarkCompleteCalls++
	if m.MarkCompleteFunc != nil {
		return m.MarkCompleteFunc(ctx, tx, seasonID)
	}
	return nil
}

func (m *SeasonRepoMock) Reset(ctx context.Context, tx pgx.Tx, seasonID int) error {
	m.ResetCalls++
	if m.ResetFunc != nil {
		return m.ResetFunc(ctx, tx, seasonID)
	}
	return nil
}

func (m *SeasonRepoMock) Delete(ctx context.Context, tx pgx.Tx, seasonID int) error {
	m.DeleteCalls++
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, tx, seasonID)
	}
	return nil
}
