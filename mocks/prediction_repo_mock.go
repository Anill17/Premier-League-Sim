package mocks

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/Anill17/league-api/internal/domain"
)

type PredictionRepoMock struct {
	GetLatestFunc       func(ctx context.Context, seasonID int) ([]domain.PredictionWithTeam, error)
	GetByWeekFunc       func(ctx context.Context, seasonID, week int) ([]domain.PredictionWithTeam, error)
	DeleteForWeekFunc   func(ctx context.Context, tx pgx.Tx, seasonID, week int) error
	DeleteForSeasonFunc func(ctx context.Context, tx pgx.Tx, seasonID int) error
	BulkInsertFunc      func(ctx context.Context, tx pgx.Tx, predictions []domain.Prediction) error

	GetLatestCalls       int
	GetByWeekCalls       int
	DeleteForWeekCalls   int
	DeleteForSeasonCalls int
	BulkInsertCalls      int

	// LastBulkInsert records the predictions handed to BulkInsert so
	// tests can verify exactly what would have been written.
	LastBulkInsert []domain.Prediction
}

var _ domain.PredictionRepository = (*PredictionRepoMock)(nil)

func (m *PredictionRepoMock) GetLatest(ctx context.Context, seasonID int) ([]domain.PredictionWithTeam, error) {
	m.GetLatestCalls++
	if m.GetLatestFunc != nil {
		return m.GetLatestFunc(ctx, seasonID)
	}
	return nil, nil
}

func (m *PredictionRepoMock) GetByWeek(ctx context.Context, seasonID, week int) ([]domain.PredictionWithTeam, error) {
	m.GetByWeekCalls++
	if m.GetByWeekFunc != nil {
		return m.GetByWeekFunc(ctx, seasonID, week)
	}
	return nil, nil
}

func (m *PredictionRepoMock) DeleteForWeek(ctx context.Context, tx pgx.Tx, seasonID, week int) error {
	m.DeleteForWeekCalls++
	if m.DeleteForWeekFunc != nil {
		return m.DeleteForWeekFunc(ctx, tx, seasonID, week)
	}
	return nil
}

func (m *PredictionRepoMock) DeleteForSeason(ctx context.Context, tx pgx.Tx, seasonID int) error {
	m.DeleteForSeasonCalls++
	if m.DeleteForSeasonFunc != nil {
		return m.DeleteForSeasonFunc(ctx, tx, seasonID)
	}
	return nil
}

func (m *PredictionRepoMock) BulkInsert(ctx context.Context, tx pgx.Tx, predictions []domain.Prediction) error {
	m.BulkInsertCalls++
	m.LastBulkInsert = append([]domain.Prediction(nil), predictions...)
	if m.BulkInsertFunc != nil {
		return m.BulkInsertFunc(ctx, tx, predictions)
	}
	return nil
}
