// Package domain holds the core business entities and the interfaces
// that describe how the rest of the system collaborates with them.
// Nothing in this package may import from /internal/repository,
// /internal/service, or /internal/handler — it is the inner ring of
// the dependency graph.
package domain

import "time"

// Result values stored in TeamForm.RecentResults.
const (
	ResultWin  = "W"
	ResultDraw = "D"
	ResultLoss = "L"
)

// MaxRecentResults bounds how many recent results we remember per team.
const MaxRecentResults = 3

// Team is a club competing in the league. Attributes drive the Poisson
// xG model; they are stored once in the DB and never hardcoded in Go.
type Team struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	Attack        int    `json:"attack"`
	Defense       int    `json:"defense"`
	Midfield      int    `json:"midfield"`
	HomeAdvantage int    `json:"home_advantage"`
	Strength      int    `json:"strength"`
}

// Season is a single league campaign of 6 weeks.
type Season struct {
	ID          int       `json:"id"`
	CurrentWeek int       `json:"current_week"`
	IsComplete  bool      `json:"is_complete"`
	CreatedAt   time.Time `json:"created_at"`
}

// Fixture is one scheduled match between two teams in a given week.
// HomeGoals / AwayGoals / PlayedAt are pointers so the "not yet played"
// state is representable without ambiguous sentinel values.
type Fixture struct {
	ID         int        `json:"id"`
	SeasonID   int        `json:"season_id"`
	Week       int        `json:"week"`
	HomeTeamID int        `json:"home_team_id"`
	AwayTeamID int        `json:"away_team_id"`
	HomeGoals  *int       `json:"home_goals,omitempty"`
	AwayGoals  *int       `json:"away_goals,omitempty"`
	Played     bool       `json:"played"`
	PlayedAt   *time.Time `json:"played_at,omitempty"`
}

// Standing is a row in the league table for a team in a given season.
// GoalDifference is derived (GoalsFor - GoalsAgainst). The DB stores it
// as a generated column; we mirror that invariant in Go.
type Standing struct {
	ID             int `json:"id"`
	SeasonID       int `json:"season_id"`
	TeamID         int `json:"team_id"`
	Played         int `json:"played"`
	Won            int `json:"won"`
	Drawn          int `json:"drawn"`
	Lost           int `json:"lost"`
	GoalsFor       int `json:"goals_for"`
	GoalsAgainst   int `json:"goals_against"`
	GoalDifference int `json:"goal_difference"`
	Points         int `json:"points"`
}

// TeamForm holds the last MaxRecentResults outcomes for a team in the
// season. It feeds the Poisson xG form multiplier.
type TeamForm struct {
	ID            int      `json:"id"`
	SeasonID      int      `json:"season_id"`
	TeamID        int      `json:"team_id"`
	RecentResults []string `json:"recent_results"`
}

// Prediction is the Monte-Carlo championship probability for a team
// at the end of a given week.
type Prediction struct {
	ID                      int       `json:"id"`
	SeasonID                int       `json:"season_id"`
	Week                    int       `json:"week"`
	TeamID                  int       `json:"team_id"`
	ChampionshipProbability float64   `json:"championship_probability"`
	CreatedAt               time.Time `json:"created_at"`
}

// MatchResult is the in-memory output of the match simulator.
// It is intentionally separate from Fixture so the simulator stays
// pure: it returns a value object without touching persistence.
type MatchResult struct {
	HomeGoals int     `json:"home_goals"`
	AwayGoals int     `json:"away_goals"`
	HomeXG    float64 `json:"home_xg"`
	AwayXG    float64 `json:"away_xg"`
}

// =====================================================================
// DTOs — view-models returned by services / consumed by handlers.
// Keeping these alongside entities (per project rule that ALL structs
// live in this file) so handlers never need to assemble their own.
// =====================================================================

// StandingWithTeam decorates a Standing row with the team name so a
// single response can be rendered without a second lookup client-side.
type StandingWithTeam struct {
	Standing
	TeamName string `json:"team_name"`
}

// FixtureWithTeams decorates a Fixture with both team names.
type FixtureWithTeams struct {
	Fixture
	HomeTeamName string `json:"home_team_name"`
	AwayTeamName string `json:"away_team_name"`
}

// PredictionWithTeam decorates a Prediction with the team name.
type PredictionWithTeam struct {
	Prediction
	TeamName string `json:"team_name"`
}

// SeasonWithFixtures is the payload returned by season creation.
type SeasonWithFixtures struct {
	Season   Season             `json:"season"`
	Fixtures []FixtureWithTeams `json:"fixtures"`
}

// WeekView is the payload for "what happened in week N".
type WeekView struct {
	Week      int                `json:"week"`
	Fixtures  []FixtureWithTeams `json:"fixtures"`
	Standings []StandingWithTeam `json:"standings"`
}

// PlayedMatch pairs the fixture row with the simulator's verdict.
type PlayedMatch struct {
	Fixture FixtureWithTeams `json:"fixture"`
	Result  MatchResult      `json:"result"`
}

// WeekResult is what a /next-week call returns.
type WeekResult struct {
	Week        int                  `json:"week"`
	Matches     []PlayedMatch        `json:"matches"`
	Standings   []StandingWithTeam   `json:"standings"`
	Predictions []PredictionWithTeam `json:"predictions,omitempty"`
}

// PlayAllResult is what a /play-all call returns.
type PlayAllResult struct {
	Weeks       []WeekResult         `json:"weeks"`
	Standings   []StandingWithTeam   `json:"final_standings"`
	Predictions []PredictionWithTeam `json:"final_predictions"`
}

// EditFixtureResult is what a PUT /fixtures/:id call returns.
type EditFixtureResult struct {
	Fixture     FixtureWithTeams     `json:"fixture"`
	Standings   []StandingWithTeam   `json:"standings"`
	Predictions []PredictionWithTeam `json:"predictions"`
}
