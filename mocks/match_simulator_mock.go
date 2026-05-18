package mocks

import (
	"github.com/insider/league-api/internal/domain"
)

// MatchSimulatorMock is a fully-controllable MatchSimulator. By default
// it produces a 1-1 draw with the supplied teams; tests override
// SimulateFunc to make every match deterministic.
type MatchSimulatorMock struct {
	SimulateFunc func(home, away domain.Team, homeForm, awayForm domain.TeamForm) domain.MatchResult

	SimulateCalls int
}

var _ domain.MatchSimulator = (*MatchSimulatorMock)(nil)

func (m *MatchSimulatorMock) Simulate(
	home, away domain.Team,
	homeForm, awayForm domain.TeamForm,
) domain.MatchResult {
	m.SimulateCalls++
	if m.SimulateFunc != nil {
		return m.SimulateFunc(home, away, homeForm, awayForm)
	}
	return domain.MatchResult{HomeGoals: 1, AwayGoals: 1, HomeXG: 1.0, AwayXG: 1.0}
}

// HomeWinSimulator is a convenience constructor: every match results in
// a 2-0 win for the home side. Useful for deterministic standings
// assertions in tests.
func HomeWinSimulator() *MatchSimulatorMock {
	return &MatchSimulatorMock{
		SimulateFunc: func(_, _ domain.Team, _, _ domain.TeamForm) domain.MatchResult {
			return domain.MatchResult{HomeGoals: 2, AwayGoals: 0, HomeXG: 2.0, AwayXG: 0.5}
		},
	}
}

// FixedScoreSimulator returns the same scoreline for every match.
func FixedScoreSimulator(home, away int) *MatchSimulatorMock {
	return &MatchSimulatorMock{
		SimulateFunc: func(_, _ domain.Team, _, _ domain.TeamForm) domain.MatchResult {
			return domain.MatchResult{
				HomeGoals: home,
				AwayGoals: away,
				HomeXG:    float64(home),
				AwayXG:    float64(away),
			}
		},
	}
}
