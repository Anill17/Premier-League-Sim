// Package service contains the business logic of the league. Each
// service has a single responsibility and depends only on interfaces
// declared in internal/domain.
package service

import (
	"sort"

	"github.com/Anill17/league-api/internal/domain"
)

// StandingsService is the sole implementation of both
// domain.StandingsCalculator and domain.StandingsSorter. By centralising
// the standings logic here we satisfy DRY: SimulateWeek, EditFixture
// and the read path all share this code.
type StandingsService struct{}

// NewStandingsService returns a stateless calculator/sorter.
func NewStandingsService() *StandingsService {
	return &StandingsService{}
}

// Compile-time interface satisfaction checks.
var (
	_ domain.StandingsCalculator = (*StandingsService)(nil)
	_ domain.StandingsSorter     = (*StandingsService)(nil)
)

// Calculate rebuilds the full standings table from the played fixtures.
// The returned slice is in the same order as the supplied teams slice;
// ordering for display is a separate concern (see SortWithHeadToHead).
func (s *StandingsService) Calculate(
	seasonID int,
	teams []domain.Team,
	playedFixtures []domain.Fixture,
) []domain.Standing {
	byTeam := make(map[int]*domain.Standing, len(teams))
	for _, t := range teams {
		byTeam[t.ID] = &domain.Standing{
			SeasonID: seasonID,
			TeamID:   t.ID,
		}
	}

	for _, f := range playedFixtures {
		if !f.Played || f.HomeGoals == nil || f.AwayGoals == nil {
			continue
		}
		home, ok := byTeam[f.HomeTeamID]
		if !ok {
			continue
		}
		away, ok := byTeam[f.AwayTeamID]
		if !ok {
			continue
		}
		applyMatch(home, away, *f.HomeGoals, *f.AwayGoals)
	}

	out := make([]domain.Standing, 0, len(teams))
	for _, t := range teams {
		row := byTeam[t.ID]
		row.GoalDifference = row.GoalsFor - row.GoalsAgainst
		out = append(out, *row)
	}
	return out
}

// SortWithHeadToHead applies head-to-head points (calculated from the
// played fixtures) as the final tiebreaker on a slice already sorted
// by points / GD / GF.
//
// The algorithm:
//  1. Find groups of consecutive rows whose (points, GD, GF) all match.
//  2. Within each group, reorder by head-to-head points DESC.
//
// This mirrors the Premier League tie-breaking sequence and is applied
// in Go rather than SQL because head-to-head depends on the set of
// played fixtures, which is variable.
func (s *StandingsService) SortWithHeadToHead(
	rows []domain.StandingWithTeam,
	playedFixtures []domain.Fixture,
) []domain.StandingWithTeam {
	if len(rows) < 2 {
		return rows
	}

	sort.SliceStable(rows, func(i, j int) bool {
		return primaryLess(rows[i].Standing, rows[j].Standing)
	})

	i := 0
	for i < len(rows) {
		j := i + 1
		for j < len(rows) && primaryEqual(rows[i].Standing, rows[j].Standing) {
			j++
		}
		if j-i > 1 {
			applyHeadToHead(rows[i:j], playedFixtures)
		}
		i = j
	}
	return rows
}

// applyMatch updates two standing rows with the result of one match.
func applyMatch(home, away *domain.Standing, hg, ag int) {
	home.Played++
	away.Played++
	home.GoalsFor += hg
	home.GoalsAgainst += ag
	away.GoalsFor += ag
	away.GoalsAgainst += hg

	switch {
	case hg > ag:
		home.Won++
		home.Points += 3
		away.Lost++
	case hg < ag:
		away.Won++
		away.Points += 3
		home.Lost++
	default:
		home.Drawn++
		away.Drawn++
		home.Points++
		away.Points++
	}
}

// primaryLess implements the canonical points / GD / GF ordering.
func primaryLess(a, b domain.Standing) bool {
	if a.Points != b.Points {
		return a.Points > b.Points
	}
	if a.GoalDifference != b.GoalDifference {
		return a.GoalDifference > b.GoalDifference
	}
	return a.GoalsFor > b.GoalsFor
}

func primaryEqual(a, b domain.Standing) bool {
	return a.Points == b.Points &&
		a.GoalDifference == b.GoalDifference &&
		a.GoalsFor == b.GoalsFor
}

// applyHeadToHead reorders a tied group by head-to-head points.
func applyHeadToHead(group []domain.StandingWithTeam, fixtures []domain.Fixture) {
	involved := make(map[int]bool, len(group))
	for _, r := range group {
		involved[r.TeamID] = true
	}

	h2h := make(map[int]int, len(group))
	for _, f := range fixtures {
		if !f.Played || f.HomeGoals == nil || f.AwayGoals == nil {
			continue
		}
		if !involved[f.HomeTeamID] || !involved[f.AwayTeamID] {
			continue
		}
		hg, ag := *f.HomeGoals, *f.AwayGoals
		switch {
		case hg > ag:
			h2h[f.HomeTeamID] += 3
		case hg < ag:
			h2h[f.AwayTeamID] += 3
		default:
			h2h[f.HomeTeamID]++
			h2h[f.AwayTeamID]++
		}
	}

	sort.SliceStable(group, func(i, j int) bool {
		return h2h[group[i].TeamID] > h2h[group[j].TeamID]
	})
}
