package mocks

import (
	"github.com/Anill17/league-api/internal/domain"
)

type StandingsSorterMock struct {
	SortWithHeadToHeadFunc func(rows []domain.StandingWithTeam, playedFixtures []domain.Fixture) []domain.StandingWithTeam

	SortWithHeadToHeadCalls int
}

var _ domain.StandingsSorter = (*StandingsSorterMock)(nil)

func (m *StandingsSorterMock) SortWithHeadToHead(
	rows []domain.StandingWithTeam,
	playedFixtures []domain.Fixture,
) []domain.StandingWithTeam {
	m.SortWithHeadToHeadCalls++
	if m.SortWithHeadToHeadFunc != nil {
		return m.SortWithHeadToHeadFunc(rows, playedFixtures)
	}
	return rows
}
