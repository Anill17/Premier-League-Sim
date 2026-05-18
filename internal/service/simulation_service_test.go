package service

import (
	"context"
	"errors"
	"math/rand"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/insider/league-api/internal/domain"
	"github.com/insider/league-api/mocks"
)

// TestPoissonSimulatorSeededDeterministic — given a fixed *rand.Rand
// the simulator must produce identical results across runs.
func TestPoissonSimulatorSeededDeterministic(t *testing.T) {
	t.Parallel()

	run := func() domain.MatchResult {
		sim := NewPoissonSimulatorWithRand(rand.New(rand.NewSource(7)))
		return sim.Simulate(
			domain.Team{Attack: 90, Defense: 85, HomeAdvantage: 8},
			domain.Team{Attack: 80, Defense: 75, HomeAdvantage: 6},
			domain.TeamForm{}, domain.TeamForm{},
		)
	}
	a, b := run(), run()
	if a != b {
		t.Fatalf("non-deterministic: %+v vs %+v", a, b)
	}
}

// TestPoissonSimulator_CapsAtSix — even with an absurd xG, the goal
// cap must clamp the result to 6 per side.
func TestPoissonSimulator_CapsAtSix(t *testing.T) {
	t.Parallel()
	sim := NewPoissonSimulatorWithRand(rand.New(rand.NewSource(1)))
	for i := 0; i < 50; i++ {
		res := sim.Simulate(
			domain.Team{Attack: 100, Defense: 1, HomeAdvantage: 10},
			domain.Team{Attack: 100, Defense: 1, HomeAdvantage: 10},
			domain.TeamForm{}, domain.TeamForm{},
		)
		if res.HomeGoals > maxGoalsPerTeam || res.AwayGoals > maxGoalsPerTeam {
			t.Fatalf("uncapped result: %+v", res)
		}
	}
}

// TestPoissonSimulator_HomeAdvantageMatters — the home team's xG must
// scale up with HomeAdvantage, all other things equal.
func TestPoissonSimulator_HomeAdvantageMatters(t *testing.T) {
	t.Parallel()
	low := NewPoissonSimulatorWithRand(rand.New(rand.NewSource(1)))
	high := NewPoissonSimulatorWithRand(rand.New(rand.NewSource(1)))

	resLow := low.Simulate(
		domain.Team{Attack: 80, Defense: 80, HomeAdvantage: 1},
		domain.Team{Attack: 80, Defense: 80, HomeAdvantage: 1},
		domain.TeamForm{}, domain.TeamForm{},
	)
	resHigh := high.Simulate(
		domain.Team{Attack: 80, Defense: 80, HomeAdvantage: 10},
		domain.Team{Attack: 80, Defense: 80, HomeAdvantage: 1},
		domain.TeamForm{}, domain.TeamForm{},
	)
	if resHigh.HomeXG <= resLow.HomeXG {
		t.Fatalf("homeAdv increase did not raise xG: low=%v high=%v",
			resLow.HomeXG, resHigh.HomeXG)
	}
}

// TestSimulationService_SimulateNextWeek_HappyPath — happy path with
// a deterministic simulator (2-0 home wins). Verifies that fixtures,
// forms, standings and season state are all written exactly once.
func TestSimulationService_SimulateNextWeek_HappyPath(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	seasonID := 1

	allFixtures := []domain.Fixture{
		{ID: 1, SeasonID: seasonID, Week: 1, HomeTeamID: 1, AwayTeamID: 4},
		{ID: 2, SeasonID: seasonID, Week: 1, HomeTeamID: 2, AwayTeamID: 3},
		{ID: 3, SeasonID: seasonID, Week: 2, HomeTeamID: 1, AwayTeamID: 3},
		{ID: 4, SeasonID: seasonID, Week: 2, HomeTeamID: 2, AwayTeamID: 4},
	}

	fixRepo := &mocks.FixtureRepoMock{
		GetBySeasonFunc: func(_ context.Context, _ int) ([]domain.Fixture, error) {
			return allFixtures, nil
		},
		LockForWeekFunc: func(_ context.Context, _ pgx.Tx, _, week int) ([]domain.Fixture, error) {
			var out []domain.Fixture
			for _, f := range allFixtures {
				if f.Week == week {
					out = append(out, f)
				}
			}
			return out, nil
		},
	}
	standingsRepo := &mocks.StandingsRepoMock{
		GetBySeasonWithTeamFunc: func(_ context.Context, _ int) ([]domain.StandingWithTeam, error) {
			return []domain.StandingWithTeam{}, nil
		},
	}
	formRepo := &mocks.FormRepoMock{
		GetBySeasonFunc: func(_ context.Context, _ int) ([]domain.TeamForm, error) {
			return []domain.TeamForm{
				{SeasonID: seasonID, TeamID: 1},
				{SeasonID: seasonID, TeamID: 2},
				{SeasonID: seasonID, TeamID: 3},
				{SeasonID: seasonID, TeamID: 4},
			}, nil
		},
	}
	predRepo := &mocks.PredictionRepoMock{}
	seasonRepo := &mocks.SeasonRepoMock{
		GetByIDFunc: func(_ context.Context, _ int) (*domain.Season, error) {
			return &domain.Season{ID: seasonID, CurrentWeek: 0}, nil
		},
	}
	teamRepo := &mocks.TeamRepoMock{
		ListFunc: func(_ context.Context) ([]domain.Team, error) { return teamSlice, nil },
	}

	svc := NewSimulationService(
		fixRepo, standingsRepo, formRepo, predRepo, seasonRepo, teamRepo,
		mocks.HomeWinSimulator(), &mocks.ChampionshipPredictorMock{},
		NewStandingsService(), NewFormService(formRepo),
		&mocks.TxManagerMock{},
	)

	out, err := svc.SimulateNextWeek(ctx, seasonID)
	if err != nil {
		t.Fatalf("SimulateNextWeek: %v", err)
	}
	if out.Week != 1 {
		t.Fatalf("week: got %d, want 1", out.Week)
	}
	if len(out.Matches) != 2 {
		t.Fatalf("matches: got %d, want 2", len(out.Matches))
	}

	// Fixture updates: 2.
	if fixRepo.UpdateCalls != 2 {
		t.Fatalf("fixture Update calls: %d, want 2", fixRepo.UpdateCalls)
	}
	// Form upserts: 2 fixtures × 2 teams.
	if formRepo.UpsertCalls != 4 {
		t.Fatalf("form Upsert calls: %d, want 4", formRepo.UpsertCalls)
	}
	// Standings replaced once.
	if standingsRepo.BulkReplaceCalls != 1 {
		t.Fatalf("standings BulkReplace calls: %d, want 1", standingsRepo.BulkReplaceCalls)
	}
	// Season week bumped to 1.
	if seasonRepo.UpdateCurrentWeekCalls != 1 {
		t.Fatalf("UpdateCurrentWeek calls: %d, want 1", seasonRepo.UpdateCurrentWeekCalls)
	}
}

// TestSimulationService_SimulateNextWeek_RejectsCompleteSeason — once
// the season is marked complete, SimulateNextWeek must return the
// sentinel error and touch nothing.
func TestSimulationService_SimulateNextWeek_RejectsCompleteSeason(t *testing.T) {
	t.Parallel()

	seasonRepo := &mocks.SeasonRepoMock{
		GetByIDFunc: func(_ context.Context, _ int) (*domain.Season, error) {
			return &domain.Season{ID: 1, CurrentWeek: 6, IsComplete: true}, nil
		},
	}
	fixRepo := &mocks.FixtureRepoMock{}
	svc := NewSimulationService(
		fixRepo, &mocks.StandingsRepoMock{}, &mocks.FormRepoMock{},
		&mocks.PredictionRepoMock{}, seasonRepo, &mocks.TeamRepoMock{},
		&mocks.MatchSimulatorMock{}, &mocks.ChampionshipPredictorMock{},
		NewStandingsService(), NewFormService(&mocks.FormRepoMock{}),
		&mocks.TxManagerMock{},
	)

	_, err := svc.SimulateNextWeek(context.Background(), 1)
	if !errors.Is(err, ErrSeasonComplete) {
		t.Fatalf("expected ErrSeasonComplete, got %v", err)
	}
	if fixRepo.LockForWeekCalls != 0 {
		t.Fatalf("locked fixtures despite complete season")
	}
}

// TestSimulationService_SimulateNextWeek_AlreadyPlayed — if the next
// week has nothing to lock (week already played in a race), the
// sentinel error fires.
func TestSimulationService_SimulateNextWeek_AlreadyPlayed(t *testing.T) {
	t.Parallel()

	fixRepo := &mocks.FixtureRepoMock{
		GetBySeasonFunc: func(_ context.Context, _ int) ([]domain.Fixture, error) {
			return nil, nil
		},
		LockForWeekFunc: func(_ context.Context, _ pgx.Tx, _, _ int) ([]domain.Fixture, error) {
			return nil, nil
		},
	}
	seasonRepo := &mocks.SeasonRepoMock{
		GetByIDFunc: func(_ context.Context, _ int) (*domain.Season, error) {
			return &domain.Season{ID: 1, CurrentWeek: 0}, nil
		},
	}
	svc := NewSimulationService(
		fixRepo, &mocks.StandingsRepoMock{}, &mocks.FormRepoMock{},
		&mocks.PredictionRepoMock{}, seasonRepo, &mocks.TeamRepoMock{},
		&mocks.MatchSimulatorMock{}, &mocks.ChampionshipPredictorMock{},
		NewStandingsService(), NewFormService(&mocks.FormRepoMock{}),
		&mocks.TxManagerMock{},
	)

	_, err := svc.SimulateNextWeek(context.Background(), 1)
	if !errors.Is(err, ErrWeekAlreadyPlayed) {
		t.Fatalf("expected ErrWeekAlreadyPlayed, got %v", err)
	}
}

// TestSimulationService_SimulateNextWeek_TriggersPredictorAtWeek4 —
// after simulating week 4, the predictor runs and predictions are
// replaced atomically.
func TestSimulationService_SimulateNextWeek_TriggersPredictorAtWeek4(t *testing.T) {
	t.Parallel()

	const week = 4
	fixtures := []domain.Fixture{
		{ID: 1, SeasonID: 1, Week: week, HomeTeamID: 1, AwayTeamID: 2},
		{ID: 2, SeasonID: 1, Week: week, HomeTeamID: 3, AwayTeamID: 4},
	}
	fixRepo := &mocks.FixtureRepoMock{
		GetBySeasonFunc: func(_ context.Context, _ int) ([]domain.Fixture, error) { return fixtures, nil },
		LockForWeekFunc: func(_ context.Context, _ pgx.Tx, _, _ int) ([]domain.Fixture, error) {
			return fixtures, nil
		},
	}
	formRepo := &mocks.FormRepoMock{
		GetBySeasonFunc: func(_ context.Context, _ int) ([]domain.TeamForm, error) {
			return []domain.TeamForm{
				{TeamID: 1}, {TeamID: 2}, {TeamID: 3}, {TeamID: 4},
			}, nil
		},
	}
	predictor := &mocks.ChampionshipPredictorMock{
		PredictFunc: func(_ context.Context, sID, wk int, _ []domain.Standing, _ []domain.Fixture, _ []domain.Team, _ []domain.TeamForm) []domain.Prediction {
			return []domain.Prediction{{SeasonID: sID, Week: wk, TeamID: 1, ChampionshipProbability: 25}}
		},
	}
	predRepo := &mocks.PredictionRepoMock{}
	seasonRepo := &mocks.SeasonRepoMock{
		GetByIDFunc: func(_ context.Context, _ int) (*domain.Season, error) {
			return &domain.Season{ID: 1, CurrentWeek: week - 1}, nil
		},
	}

	svc := NewSimulationService(
		fixRepo, &mocks.StandingsRepoMock{}, formRepo, predRepo,
		seasonRepo, &mocks.TeamRepoMock{
			ListFunc: func(_ context.Context) ([]domain.Team, error) { return teamSlice, nil },
		},
		mocks.HomeWinSimulator(), predictor,
		NewStandingsService(), NewFormService(formRepo),
		&mocks.TxManagerMock{},
	)

	if _, err := svc.SimulateNextWeek(context.Background(), 1); err != nil {
		t.Fatalf("SimulateNextWeek: %v", err)
	}

	if predictor.PredictCalls != 1 {
		t.Fatalf("predictor: got %d calls, want 1", predictor.PredictCalls)
	}
	if predRepo.DeleteForWeekCalls != 1 || predRepo.BulkInsertCalls != 1 {
		t.Fatalf("predictions delete/insert: delete=%d insert=%d",
			predRepo.DeleteForWeekCalls, predRepo.BulkInsertCalls)
	}
}
