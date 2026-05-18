package service

import (
	"sort"
	"testing"

	"github.com/insider/league-api/internal/domain"
)

// ptr returns &v — used to construct *int fields on Fixture rows.
func ptr[T any](v T) *T { return &v }

// teamSlice is the canonical 4-team test fixture matching 002_seed.sql
// ids so tests read cleanly.
var teamSlice = []domain.Team{
	{ID: 1, Name: "Manchester City"},
	{ID: 2, Name: "Arsenal"},
	{ID: 3, Name: "Liverpool"},
	{ID: 4, Name: "Chelsea"},
}

func TestStandingsCalculator_Empty(t *testing.T) {
	t.Parallel()
	s := NewStandingsService()
	got := s.Calculate(1, teamSlice, nil)
	if len(got) != len(teamSlice) {
		t.Fatalf("expected %d rows, got %d", len(teamSlice), len(got))
	}
	for _, row := range got {
		if row.Played != 0 || row.Points != 0 {
			t.Fatalf("expected zero row, got %+v", row)
		}
	}
}

func TestStandingsCalculator_HomeWinsMatch(t *testing.T) {
	t.Parallel()
	s := NewStandingsService()

	fixtures := []domain.Fixture{
		{
			SeasonID: 1, Week: 1,
			HomeTeamID: 1, AwayTeamID: 2,
			HomeGoals: ptr(3), AwayGoals: ptr(1),
			Played: true,
		},
	}

	got := s.Calculate(1, teamSlice, fixtures)
	row := findStanding(t, got, 1)
	if row.Won != 1 || row.Played != 1 || row.Points != 3 {
		t.Fatalf("home: %+v", row)
	}
	if row.GoalsFor != 3 || row.GoalsAgainst != 1 || row.GoalDifference != 2 {
		t.Fatalf("home goals: %+v", row)
	}
	away := findStanding(t, got, 2)
	if away.Lost != 1 || away.Points != 0 || away.GoalDifference != -2 {
		t.Fatalf("away: %+v", away)
	}
}

func TestStandingsCalculator_Draw(t *testing.T) {
	t.Parallel()
	s := NewStandingsService()
	fixtures := []domain.Fixture{
		{
			SeasonID: 1, Week: 1,
			HomeTeamID: 1, AwayTeamID: 2,
			HomeGoals: ptr(2), AwayGoals: ptr(2),
			Played: true,
		},
	}
	got := s.Calculate(1, teamSlice, fixtures)
	for _, id := range []int{1, 2} {
		row := findStanding(t, got, id)
		if row.Drawn != 1 || row.Points != 1 || row.GoalsFor != 2 || row.GoalDifference != 0 {
			t.Fatalf("team %d draw row: %+v", id, row)
		}
	}
}

// TestStandingsCalculator_TableDriven exercises a small mini-season.
func TestStandingsCalculator_TableDriven(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		fixtures []domain.Fixture
		want     map[int]domain.Standing // expected by team ID
	}{
		{
			name: "all_four_teams_one_round",
			fixtures: []domain.Fixture{
				{SeasonID: 1, Week: 1, HomeTeamID: 1, AwayTeamID: 4, HomeGoals: ptr(3), AwayGoals: ptr(0), Played: true},
				{SeasonID: 1, Week: 1, HomeTeamID: 2, AwayTeamID: 3, HomeGoals: ptr(1), AwayGoals: ptr(1), Played: true},
			},
			want: map[int]domain.Standing{
				1: {Played: 1, Won: 1, Drawn: 0, Lost: 0, GoalsFor: 3, GoalsAgainst: 0, GoalDifference: 3, Points: 3},
				2: {Played: 1, Won: 0, Drawn: 1, Lost: 0, GoalsFor: 1, GoalsAgainst: 1, GoalDifference: 0, Points: 1},
				3: {Played: 1, Won: 0, Drawn: 1, Lost: 0, GoalsFor: 1, GoalsAgainst: 1, GoalDifference: 0, Points: 1},
				4: {Played: 1, Won: 0, Drawn: 0, Lost: 1, GoalsFor: 0, GoalsAgainst: 3, GoalDifference: -3, Points: 0},
			},
		},
		{
			name: "ignore_unplayed_or_partial",
			fixtures: []domain.Fixture{
				{SeasonID: 1, HomeTeamID: 1, AwayTeamID: 2, HomeGoals: ptr(1), AwayGoals: ptr(0), Played: true},
				{SeasonID: 1, HomeTeamID: 3, AwayTeamID: 4, Played: false}, // ignored
				{SeasonID: 1, HomeTeamID: 1, AwayTeamID: 3, Played: true},  // ignored (nil goals)
			},
			want: map[int]domain.Standing{
				1: {Played: 1, Won: 1, GoalsFor: 1, GoalDifference: 1, Points: 3},
				2: {Played: 1, Lost: 1, GoalsAgainst: 1, GoalDifference: -1},
				3: {},
				4: {},
			},
		},
	}

	calc := NewStandingsService()
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := calc.Calculate(1, teamSlice, c.fixtures)
			for _, row := range got {
				want, ok := c.want[row.TeamID]
				if !ok {
					t.Fatalf("unexpected team %d in result", row.TeamID)
				}
				if row.Played != want.Played ||
					row.Won != want.Won ||
					row.Drawn != want.Drawn ||
					row.Lost != want.Lost ||
					row.GoalsFor != want.GoalsFor ||
					row.GoalsAgainst != want.GoalsAgainst ||
					row.GoalDifference != want.GoalDifference ||
					row.Points != want.Points {
					t.Fatalf("team %d: got %+v want %+v", row.TeamID, row, want)
				}
			}
		})
	}
}

// TestSortWithHeadToHead_PrimaryOrder confirms basic points/GD/GF
// sorting works without involving any tied groups.
func TestSortWithHeadToHead_PrimaryOrder(t *testing.T) {
	t.Parallel()
	s := NewStandingsService()
	rows := []domain.StandingWithTeam{
		{Standing: domain.Standing{TeamID: 1, Points: 9, GoalDifference: 5, GoalsFor: 10}, TeamName: "A"},
		{Standing: domain.Standing{TeamID: 2, Points: 6, GoalDifference: 3, GoalsFor: 6}, TeamName: "B"},
		{Standing: domain.Standing{TeamID: 3, Points: 3, GoalDifference: -1, GoalsFor: 2}, TeamName: "C"},
	}
	got := s.SortWithHeadToHead(rows, nil)
	wantOrder := []int{1, 2, 3}
	for i, want := range wantOrder {
		if got[i].TeamID != want {
			t.Fatalf("position %d: got team %d, want %d", i, got[i].TeamID, want)
		}
	}
}

// TestSortWithHeadToHead_BreaksTies builds two teams tied on
// points/GD/GF and one head-to-head fixture that resolves them.
func TestSortWithHeadToHead_BreaksTies(t *testing.T) {
	t.Parallel()
	s := NewStandingsService()
	rows := []domain.StandingWithTeam{
		{Standing: domain.Standing{TeamID: 1, Points: 4, GoalDifference: 1, GoalsFor: 3}},
		{Standing: domain.Standing{TeamID: 2, Points: 4, GoalDifference: 1, GoalsFor: 3}},
	}
	played := []domain.Fixture{
		{HomeTeamID: 1, AwayTeamID: 2, HomeGoals: ptr(2), AwayGoals: ptr(0), Played: true},
	}
	got := s.SortWithHeadToHead(rows, played)
	if got[0].TeamID != 1 {
		t.Fatalf("h2h: expected team 1 first, got %d", got[0].TeamID)
	}
}

// findStanding returns the row for teamID, failing the test if absent.
func findStanding(t *testing.T, rows []domain.Standing, teamID int) domain.Standing {
	t.Helper()
	for _, r := range rows {
		if r.TeamID == teamID {
			return r
		}
	}
	t.Fatalf("team %d not found in %+v", teamID, rows)
	return domain.Standing{}
}

// TestCalculateIsStableOrderIndependent — calculate should yield the
// same per-team values regardless of the fixture ordering.
func TestCalculateIsStableOrderIndependent(t *testing.T) {
	t.Parallel()
	s := NewStandingsService()
	fixtures := []domain.Fixture{
		{HomeTeamID: 1, AwayTeamID: 2, HomeGoals: ptr(2), AwayGoals: ptr(0), Played: true},
		{HomeTeamID: 3, AwayTeamID: 4, HomeGoals: ptr(1), AwayGoals: ptr(1), Played: true},
		{HomeTeamID: 1, AwayTeamID: 3, HomeGoals: ptr(0), AwayGoals: ptr(2), Played: true},
	}
	original := s.Calculate(1, teamSlice, fixtures)

	shuffled := append([]domain.Fixture(nil), fixtures...)
	sort.Slice(shuffled, func(i, j int) bool {
		return shuffled[i].HomeTeamID > shuffled[j].HomeTeamID
	})
	swapped := s.Calculate(1, teamSlice, shuffled)

	indexBy := func(rows []domain.Standing) map[int]domain.Standing {
		m := map[int]domain.Standing{}
		for _, r := range rows {
			m[r.TeamID] = r
		}
		return m
	}
	a, b := indexBy(original), indexBy(swapped)
	for id := range a {
		if a[id] != b[id] {
			t.Fatalf("team %d differs by fixture order: %+v vs %+v",
				id, a[id], b[id])
		}
	}
}
