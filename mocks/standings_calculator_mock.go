package mocks

import (
	"github.com/Anill17/league-api/internal/domain"
)

type StandingsCalculatorMock struct {
	CalculateFunc func(seasonID int, teams []domain.Team, playedFixtures []domain.Fixture) []domain.Standing

	CalculateCalls int
}

var _ domain.StandingsCalculator = (*StandingsCalculatorMock)(nil)

func (m *StandingsCalculatorMock) Calculate(
	seasonID int,
	teams []domain.Team,
	playedFixtures []domain.Fixture,
) []domain.Standing {
	m.CalculateCalls++
	if m.CalculateFunc != nil {
		return m.CalculateFunc(seasonID, teams, playedFixtures)
	}
	return nil
}
