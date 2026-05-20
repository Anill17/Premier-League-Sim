package service

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Anill17/league-api/internal/domain"
	"github.com/Anill17/league-api/pkg/poisson"
)

// =====================================================================
// PoissonSimulator — default MatchSimulator implementation.
// =====================================================================

const (
	// baseRate is the league-wide expected goals base used in the xG
	// formula (see project rules section 8).
	baseRate = 1.4
	// maxGoalsPerTeam caps each team's score in a single match,
	// matching pkg/validator's MaxGoals.
	maxGoalsPerTeam = 6
)

// PoissonSimulator turns a fixture into a MatchResult using a Poisson
// xG model. It optionally takes a seeded *rand.Rand for deterministic
// runs — tests must always supply one.
type PoissonSimulator struct {
	rng *rand.Rand
}

// NewPoissonSimulator returns a simulator backed by the process-global
// random source. Use NewPoissonSimulatorWithRand for deterministic
// behaviour in tests.
func NewPoissonSimulator() *PoissonSimulator {
	return &PoissonSimulator{}
}

// NewPoissonSimulatorWithRand wires a caller-owned *rand.Rand into the
// simulator so test runs are reproducible.
func NewPoissonSimulatorWithRand(r *rand.Rand) *PoissonSimulator {
	return &PoissonSimulator{rng: r}
}

var _ domain.MatchSimulator = (*PoissonSimulator)(nil)

// Simulate computes xG for both sides and draws goals from a Poisson
// distribution. Per the project rules the score is capped at 6 per
// team to keep results plausible.
func (p *PoissonSimulator) Simulate(
	home, away domain.Team,
	homeForm, awayForm domain.TeamForm,
) domain.MatchResult {
	homeXG := baseRate *
		(float64(home.Attack) / float64(away.Defense)) *
		(1.0 + float64(home.HomeAdvantage)/100.0) *
		FormMultiplier(homeForm)

	awayXG := baseRate *
		(float64(away.Attack) / float64(home.Defense)) *
		FormMultiplier(awayForm)

	hg := poisson.Sample(homeXG, p.rng)
	ag := poisson.Sample(awayXG, p.rng)
	if hg > maxGoalsPerTeam {
		hg = maxGoalsPerTeam
	}
	if ag > maxGoalsPerTeam {
		ag = maxGoalsPerTeam
	}
	return domain.MatchResult{
		HomeGoals: hg,
		AwayGoals: ag,
		HomeXG:    homeXG,
		AwayXG:    awayXG,
	}
}

// =====================================================================
// SimulationService — SimulateNextWeek and PlayAll.
// =====================================================================

// SimulationService is the choreographer of weekly progression.
// All multi-step DB writes occur inside a transaction managed by tx.
type SimulationService struct {
	fixtures    domain.FixtureRepository
	standings   domain.StandingsRepository
	form        domain.FormRepository
	predictions domain.PredictionRepository
	seasons     domain.SeasonRepository
	teams       domain.TeamRepository
	simulator   domain.MatchSimulator
	predictor   domain.ChampionshipPredictor
	calculator  domain.StandingsCalculator
	formSvc     *FormService
	tx          domain.TxManager
}

// NewSimulationService is the only place this struct should be
// constructed. Composition root: cmd/server/main.go.
func NewSimulationService(
	fixtures domain.FixtureRepository,
	standings domain.StandingsRepository,
	form domain.FormRepository,
	predictions domain.PredictionRepository,
	seasons domain.SeasonRepository,
	teams domain.TeamRepository,
	simulator domain.MatchSimulator,
	predictor domain.ChampionshipPredictor,
	calculator domain.StandingsCalculator,
	formSvc *FormService,
	tx domain.TxManager,
) *SimulationService {
	return &SimulationService{
		fixtures:    fixtures,
		standings:   standings,
		form:        form,
		predictions: predictions,
		seasons:     seasons,
		teams:       teams,
		simulator:   simulator,
		predictor:   predictor,
		calculator:  calculator,
		formSvc:     formSvc,
		tx:          tx,
	}
}

var _ domain.SimulationService = (*SimulationService)(nil)

// Sentinel errors used by handlers to pick the right HTTP status.
var (
	ErrSeasonComplete    = errors.New("simulation: season already complete")
	ErrWeekAlreadyPlayed = errors.New("simulation: next week already played")
)

// TotalWeeks is the fixed length of one season.
const TotalWeeks = 6

// SimulateNextWeek plays the next unplayed week atomically.
//
// The transaction touches: fixtures (lock + update for the week),
// team_form (upsert), standings (bulk replace), seasons
// (update current_week) and predictions (delete + insert).
//
// Everything that goes into the predictor (teams, forms, remaining
// fixtures, standings) is computed in memory from data preloaded
// outside the tx, so the predictor sees an up-to-date snapshot even
// though our writes are not yet committed.
func (s *SimulationService) SimulateNextWeek(
	ctx context.Context,
	seasonID int,
) (*domain.WeekResult, error) {
	season, err := s.seasons.GetByID(ctx, seasonID)
	if err != nil {
		return nil, err
	}
	if season.IsComplete || season.CurrentWeek >= TotalWeeks {
		return nil, ErrSeasonComplete
	}
	nextWeek := season.CurrentWeek + 1

	teams, err := s.teams.List(ctx)
	if err != nil {
		return nil, err
	}
	teamByID := indexTeams(teams)

	forms, err := s.form.GetBySeason(ctx, seasonID)
	if err != nil {
		return nil, err
	}
	formByTeam := indexForms(forms)

	allFixtures, err := s.fixtures.GetBySeason(ctx, seasonID)
	if err != nil {
		return nil, err
	}

	var played []domain.PlayedMatch

	err = s.tx.WithTransaction(ctx, func(tx pgx.Tx) error {
		locked, err := s.fixtures.LockForWeek(ctx, tx, seasonID, nextWeek)
		if err != nil {
			return err
		}
		if len(locked) == 0 {
			return ErrWeekAlreadyPlayed
		}

		now := time.Now().UTC()
		thisWeek := make([]domain.Fixture, 0, len(locked))
		played = make([]domain.PlayedMatch, 0, len(locked))

		for i := range locked {
			fix := locked[i]
			home := teamByID[fix.HomeTeamID]
			away := teamByID[fix.AwayTeamID]
			homeForm := formByTeam[fix.HomeTeamID]
			awayForm := formByTeam[fix.AwayTeamID]

			res := s.simulator.Simulate(home, away, homeForm, awayForm)

			hg, ag := res.HomeGoals, res.AwayGoals
			fix.HomeGoals = &hg
			fix.AwayGoals = &ag
			fix.Played = true
			fix.PlayedAt = &now
			if err := s.fixtures.Update(ctx, tx, &fix); err != nil {
				return err
			}

			homeRes, awayRes := ResultFromScore(hg, ag)
			if err := s.formSvc.RecordResult(ctx, tx, &homeForm, homeRes); err != nil {
				return err
			}
			if err := s.formSvc.RecordResult(ctx, tx, &awayForm, awayRes); err != nil {
				return err
			}
			formByTeam[fix.HomeTeamID] = homeForm
			formByTeam[fix.AwayTeamID] = awayForm

			thisWeek = append(thisWeek, fix)
			played = append(played, domain.PlayedMatch{
				Fixture: domain.FixtureWithTeams{
					Fixture:      fix,
					HomeTeamName: home.Name,
					AwayTeamName: away.Name,
				},
				Result: res,
			})
		}

		// Build the union of (everything previously played) + (just played).
		playedSnapshot := mergePlayed(allFixtures, thisWeek)
		newStandings := s.calculator.Calculate(seasonID, teams, playedSnapshot)
		if err := s.standings.BulkReplace(ctx, tx, seasonID, newStandings); err != nil {
			return err
		}

		if err := s.seasons.UpdateCurrentWeek(ctx, tx, seasonID, nextWeek); err != nil {
			return err
		}

		if nextWeek >= PredictionStartWeek {
			remaining := remainingAfter(allFixtures, thisWeek)
			formsSlice := flattenForms(formByTeam)
			preds := s.predictor.Predict(
				ctx, seasonID, nextWeek,
				newStandings, remaining, teams, formsSlice,
			)
			if err := s.predictions.DeleteForWeek(ctx, tx, seasonID, nextWeek); err != nil {
				return err
			}
			if err := s.predictions.BulkInsert(ctx, tx, preds); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("simulate week %d: %w", nextWeek, err)
	}

	standingsView, err := s.standings.GetBySeasonWithTeam(ctx, seasonID)
	if err != nil {
		return nil, err
	}
	var predsView []domain.PredictionWithTeam
	if nextWeek >= PredictionStartWeek {
		predsView, err = s.predictions.GetByWeek(ctx, seasonID, nextWeek)
		if err != nil {
			return nil, err
		}
	}

	return &domain.WeekResult{
		Week:        nextWeek,
		Matches:     played,
		Standings:   standingsView,
		Predictions: predsView,
	}, nil
}

// PlayAll simulates every remaining week in its own transaction.
// The project rules forbid batching all weeks into a single tx, so
// each iteration is independent and a mid-stream failure leaves the
// preceding weeks safely committed.
func (s *SimulationService) PlayAll(
	ctx context.Context,
	seasonID int,
) (*domain.PlayAllResult, error) {
	out := &domain.PlayAllResult{}

	for {
		season, err := s.seasons.GetByID(ctx, seasonID)
		if err != nil {
			return nil, err
		}
		if season.IsComplete || season.CurrentWeek >= TotalWeeks {
			break
		}

		week, err := s.SimulateNextWeek(ctx, seasonID)
		if err != nil {
			return nil, err
		}
		out.Weeks = append(out.Weeks, *week)
	}

	if len(out.Weeks) > 0 {
		out.Standings = out.Weeks[len(out.Weeks)-1].Standings
		out.Predictions = out.Weeks[len(out.Weeks)-1].Predictions
	}
	return out, nil
}

// =====================================================================
// Helpers
// =====================================================================

func indexTeams(teams []domain.Team) map[int]domain.Team {
	m := make(map[int]domain.Team, len(teams))
	for _, t := range teams {
		m[t.ID] = t
	}
	return m
}

func indexForms(forms []domain.TeamForm) map[int]domain.TeamForm {
	m := make(map[int]domain.TeamForm, len(forms))
	for _, f := range forms {
		m[f.TeamID] = f
	}
	return m
}

func flattenForms(forms map[int]domain.TeamForm) []domain.TeamForm {
	out := make([]domain.TeamForm, 0, len(forms))
	for _, f := range forms {
		out = append(out, f)
	}
	return out
}

// mergePlayed merges the previously-played fixtures (from allFixtures)
// with the freshly simulated batch, returning a played-only set.
func mergePlayed(allFixtures, thisWeek []domain.Fixture) []domain.Fixture {
	updates := make(map[int]domain.Fixture, len(thisWeek))
	for _, f := range thisWeek {
		updates[f.ID] = f
	}
	out := make([]domain.Fixture, 0, len(allFixtures))
	for _, f := range allFixtures {
		if upd, ok := updates[f.ID]; ok {
			out = append(out, upd)
			continue
		}
		if f.Played {
			out = append(out, f)
		}
	}
	return out
}

// remainingAfter returns the fixtures that are still unplayed after
// the thisWeek batch is applied.
func remainingAfter(allFixtures, thisWeek []domain.Fixture) []domain.Fixture {
	done := make(map[int]bool, len(thisWeek))
	for _, f := range thisWeek {
		done[f.ID] = true
	}
	out := make([]domain.Fixture, 0, len(allFixtures))
	for _, f := range allFixtures {
		if done[f.ID] {
			continue
		}
		if !f.Played {
			out = append(out, f)
		}
	}
	return out
}
