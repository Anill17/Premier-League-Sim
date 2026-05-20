package mocks

import (
	"context"

	"github.com/Anill17/league-api/internal/domain"
)

type ChampionshipPredictorMock struct {
	PredictFunc func(
		ctx context.Context,
		seasonID, week int,
		standings []domain.Standing,
		remaining []domain.Fixture,
		teams []domain.Team,
		forms []domain.TeamForm,
	) []domain.Prediction

	PredictCalls int
}

var _ domain.ChampionshipPredictor = (*ChampionshipPredictorMock)(nil)

func (m *ChampionshipPredictorMock) Predict(
	ctx context.Context,
	seasonID, week int,
	standings []domain.Standing,
	remaining []domain.Fixture,
	teams []domain.Team,
	forms []domain.TeamForm,
) []domain.Prediction {
	m.PredictCalls++
	if m.PredictFunc != nil {
		return m.PredictFunc(ctx, seasonID, week, standings, remaining, teams, forms)
	}
	return nil
}
