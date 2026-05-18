package service

import (
	"context"
	"sort"
	"time"

	"github.com/insider/league-api/internal/domain"
)

// =====================================================================
// MonteCarloPrediction — default ChampionshipPredictor implementation.
// =====================================================================

// DefaultMonteCarloRuns is the run count mandated by the project rules.
// Tests may override it via NewMonteCarloPredictionWithRuns.
const DefaultMonteCarloRuns = 10000

// MonteCarloPrediction simulates every remaining fixture N times and
// reports how often each team finishes first.
//
// It depends only on a MatchSimulator (which is itself an interface)
// so a faster or stubbed simulator can be plugged in for tests.
type MonteCarloPrediction struct {
	simulator domain.MatchSimulator
	runs      int
}

// NewMonteCarloPrediction wires the default 10,000-run predictor.
func NewMonteCarloPrediction(simulator domain.MatchSimulator) *MonteCarloPrediction {
	return &MonteCarloPrediction{
		simulator: simulator,
		runs:      DefaultMonteCarloRuns,
	}
}

// NewMonteCarloPredictionWithRuns lets tests pin the run count for
// reproducibility / speed. Randomness is supplied by the injected
// simulator, not by the predictor itself.
func NewMonteCarloPredictionWithRuns(
	simulator domain.MatchSimulator,
	runs int,
) *MonteCarloPrediction {
	return &MonteCarloPrediction{
		simulator: simulator,
		runs:      runs,
	}
}

var _ domain.ChampionshipPredictor = (*MonteCarloPrediction)(nil)

// Predict runs the Monte Carlo simulation and returns one Prediction
// per team. Probabilities are expressed in percent (0–100) and across
// the league sum to ~100 (drift from rounding only).
//
// Ties for first place are resolved by splitting the win credit
// evenly across the tied teams, which gives mathematically consistent
// probabilities without baking in a tiebreaker heuristic that may
// differ from the league rules.
func (m *MonteCarloPrediction) Predict(
	ctx context.Context,
	seasonID int,
	week int,
	standings []domain.Standing,
	remaining []domain.Fixture,
	teams []domain.Team,
	forms []domain.TeamForm,
) []domain.Prediction {
	teamByID := indexTeams(teams)
	startStandings := standingsMap(standings, teams)
	startForms := indexForms(forms)

	wins := make(map[int]float64, len(teams))

	for run := 0; run < m.runs; run++ {
		if err := ctx.Err(); err != nil {
			break
		}
		sim := cloneStandingsMap(startStandings)
		forms := cloneFormsMap(startForms)

		for _, fix := range remaining {
			home := teamByID[fix.HomeTeamID]
			away := teamByID[fix.AwayTeamID]
			res := m.simulator.Simulate(home, away, forms[fix.HomeTeamID], forms[fix.AwayTeamID])
			applyMatch(sim[fix.HomeTeamID], sim[fix.AwayTeamID], res.HomeGoals, res.AwayGoals)

			homeRes, awayRes := ResultFromScore(res.HomeGoals, res.AwayGoals)
			updateFormInPlace(forms, fix.HomeTeamID, homeRes)
			updateFormInPlace(forms, fix.AwayTeamID, awayRes)
		}

		leaders := findLeaders(sim)
		credit := 1.0 / float64(len(leaders))
		for _, id := range leaders {
			wins[id] += credit
		}
	}

	now := time.Now().UTC()
	runs := float64(m.runs)
	out := make([]domain.Prediction, 0, len(teams))
	for _, t := range teams {
		prob := (wins[t.ID] / runs) * 100.0
		out = append(out, domain.Prediction{
			SeasonID:                seasonID,
			Week:                    week,
			TeamID:                  t.ID,
			ChampionshipProbability: prob,
			CreatedAt:               now,
		})
	}
	return out
}

// standingsMap returns a team-id-keyed copy of standings, inserting
// zero rows for any team without an entry in the input slice.
func standingsMap(rows []domain.Standing, teams []domain.Team) map[int]*domain.Standing {
	m := make(map[int]*domain.Standing, len(teams))
	for _, t := range teams {
		m[t.ID] = &domain.Standing{TeamID: t.ID}
	}
	for _, r := range rows {
		row := r
		m[r.TeamID] = &row
	}
	return m
}

func cloneStandingsMap(src map[int]*domain.Standing) map[int]*domain.Standing {
	dst := make(map[int]*domain.Standing, len(src))
	for id, s := range src {
		cp := *s
		dst[id] = &cp
	}
	return dst
}

func cloneFormsMap(src map[int]domain.TeamForm) map[int]domain.TeamForm {
	dst := make(map[int]domain.TeamForm, len(src))
	for id, f := range src {
		dst[id] = domain.TeamForm{
			ID:            f.ID,
			SeasonID:      f.SeasonID,
			TeamID:        f.TeamID,
			RecentResults: append([]string(nil), f.RecentResults...),
		}
	}
	return dst
}

func updateFormInPlace(forms map[int]domain.TeamForm, teamID int, result string) {
	f := forms[teamID]
	f.RecentResults = append(f.RecentResults, result)
	if len(f.RecentResults) > domain.MaxRecentResults {
		f.RecentResults = f.RecentResults[len(f.RecentResults)-domain.MaxRecentResults:]
	}
	forms[teamID] = f
}

// findLeaders returns the ids of every team tied for first place after
// sorting by points / GD / GF. Head-to-head is intentionally NOT
// applied here — its complexity buys very little for a Monte Carlo
// that already evens out tiny effects across 10k runs.
func findLeaders(standings map[int]*domain.Standing) []int {
	rows := make([]domain.Standing, 0, len(standings))
	for _, s := range standings {
		s.GoalDifference = s.GoalsFor - s.GoalsAgainst
		rows = append(rows, *s)
	}
	sort.Slice(rows, func(i, j int) bool {
		return primaryLess(rows[i], rows[j])
	})

	top := rows[0]
	leaders := []int{top.TeamID}
	for i := 1; i < len(rows); i++ {
		if primaryEqual(top, rows[i]) {
			leaders = append(leaders, rows[i].TeamID)
		} else {
			break
		}
	}
	return leaders
}

// =====================================================================
// PredictionService — read-side facade.
// =====================================================================

// PredictionService is a thin wrapper over PredictionRepository.
// Write paths run elsewhere (SimulationService, FixtureService).
type PredictionService struct {
	repo domain.PredictionRepository
}

// NewPredictionService wires the repo into the read-side service.
func NewPredictionService(repo domain.PredictionRepository) *PredictionService {
	return &PredictionService{repo: repo}
}

var _ domain.PredictionService = (*PredictionService)(nil)

// GetLatest returns the most recent set of predictions for a season.
func (s *PredictionService) GetLatest(
	ctx context.Context,
	seasonID int,
) ([]domain.PredictionWithTeam, error) {
	return s.repo.GetLatest(ctx, seasonID)
}

// GetByWeek returns the prediction snapshot for a specific week.
func (s *PredictionService) GetByWeek(
	ctx context.Context,
	seasonID, week int,
) ([]domain.PredictionWithTeam, error) {
	return s.repo.GetByWeek(ctx, seasonID, week)
}
