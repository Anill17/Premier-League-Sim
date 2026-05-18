package service

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/insider/league-api/internal/domain"
	"github.com/insider/league-api/mocks"
)

// TestMonteCarloProbabilitiesSumTo100 — the core invariant from the
// project rules: probabilities sum to ~100. The deterministic
// simulator below means every run produces the same standings, so
// the sum should be exactly 100. We use a small tolerance to allow
// for floating-point arithmetic.
func TestMonteCarloProbabilitiesSumTo100(t *testing.T) {
	t.Parallel()

	predictor := NewMonteCarloPredictionWithRuns(mocks.FixedScoreSimulator(2, 0), 500)

	standings := []domain.Standing{
		{TeamID: 1, Played: 3, Points: 9, GoalsFor: 6, GoalsAgainst: 0, GoalDifference: 6},
		{TeamID: 2, Played: 3, Points: 6, GoalsFor: 4, GoalsAgainst: 2, GoalDifference: 2},
		{TeamID: 3, Played: 3, Points: 3, GoalsFor: 2, GoalsAgainst: 4, GoalDifference: -2},
		{TeamID: 4, Played: 3, Points: 0, GoalsFor: 0, GoalsAgainst: 6, GoalDifference: -6},
	}
	remaining := []domain.Fixture{
		{HomeTeamID: 1, AwayTeamID: 4, Played: false},
		{HomeTeamID: 2, AwayTeamID: 3, Played: false},
	}
	forms := []domain.TeamForm{
		{TeamID: 1}, {TeamID: 2}, {TeamID: 3}, {TeamID: 4},
	}

	preds := predictor.Predict(context.Background(), 1, 3, standings, remaining, teamSlice, forms)
	if len(preds) != len(teamSlice) {
		t.Fatalf("expected %d predictions, got %d", len(teamSlice), len(preds))
	}

	var sum float64
	for _, p := range preds {
		sum += p.ChampionshipProbability
		if p.ChampionshipProbability < 0 || p.ChampionshipProbability > 100 {
			t.Fatalf("team %d: out of range %v", p.TeamID, p.ChampionshipProbability)
		}
	}
	if math.Abs(sum-100.0) > 0.001 {
		t.Fatalf("probabilities sum to %v, want ~100", sum)
	}
}

// TestMonteCarloLeaderAlwaysWinsWhenUncatchable — if the leader's
// points lead is greater than the maximum possible remaining points,
// the leader's probability must be 100%.
func TestMonteCarloLeaderAlwaysWinsWhenUncatchable(t *testing.T) {
	t.Parallel()

	// Team 1: 30 pts already. Others: 0. Only 2 fixtures left
	// (3 max pts each). Team 1 cannot be caught.
	predictor := NewMonteCarloPredictionWithRuns(mocks.FixedScoreSimulator(0, 0), 200)
	standings := []domain.Standing{
		{TeamID: 1, Played: 10, Points: 30, GoalsFor: 30, GoalsAgainst: 0, GoalDifference: 30},
		{TeamID: 2, Played: 10, Points: 0},
		{TeamID: 3, Played: 10, Points: 0},
		{TeamID: 4, Played: 10, Points: 0},
	}
	remaining := []domain.Fixture{
		{HomeTeamID: 2, AwayTeamID: 3},
		{HomeTeamID: 4, AwayTeamID: 1},
	}
	forms := []domain.TeamForm{{TeamID: 1}, {TeamID: 2}, {TeamID: 3}, {TeamID: 4}}

	preds := predictor.Predict(context.Background(), 1, 5, standings, remaining, teamSlice, forms)
	for _, p := range preds {
		if p.TeamID == 1 && p.ChampionshipProbability < 99.9 {
			t.Fatalf("uncatchable leader probability = %v, want ~100", p.ChampionshipProbability)
		}
		if p.TeamID != 1 && p.ChampionshipProbability > 0.1 {
			t.Fatalf("non-leader %d probability = %v, want ~0", p.TeamID, p.ChampionshipProbability)
		}
	}
}

// TestMonteCarloDeterministicSimulatorYieldsDeterministicResult —
// when the simulator is deterministic, multiple Predict calls yield
// identical probabilities. This guards against accidental hidden
// randomness leaking into the predictor.
func TestMonteCarloDeterministicSimulatorYieldsDeterministicResult(t *testing.T) {
	t.Parallel()

	predictor := NewMonteCarloPredictionWithRuns(mocks.FixedScoreSimulator(1, 0), 100)
	standings := []domain.Standing{
		{TeamID: 1, Points: 3, GoalsFor: 1},
		{TeamID: 2, Points: 0, GoalsFor: 0},
		{TeamID: 3, Points: 0, GoalsFor: 0},
		{TeamID: 4, Points: 0, GoalsFor: 0},
	}
	remaining := []domain.Fixture{
		{HomeTeamID: 2, AwayTeamID: 3},
		{HomeTeamID: 1, AwayTeamID: 4},
	}
	forms := []domain.TeamForm{{TeamID: 1}, {TeamID: 2}, {TeamID: 3}, {TeamID: 4}}

	first := predictor.Predict(context.Background(), 1, 5, standings, remaining, teamSlice, forms)
	second := predictor.Predict(context.Background(), 1, 5, standings, remaining, teamSlice, forms)
	if len(first) != len(second) {
		t.Fatalf("length mismatch")
	}
	for i := range first {
		if first[i].ChampionshipProbability != second[i].ChampionshipProbability {
			t.Fatalf("team %d: %v vs %v", first[i].TeamID,
				first[i].ChampionshipProbability, second[i].ChampionshipProbability)
		}
	}
}

// TestMonteCarloRespectsContext — cancelling the context aborts the
// run without panicking. The result is whatever was accumulated so far,
// which we don't constrain.
func TestMonteCarloRespectsContext(t *testing.T) {
	t.Parallel()

	predictor := NewMonteCarloPredictionWithRuns(mocks.FixedScoreSimulator(1, 1), 1_000_000)
	standings := []domain.Standing{
		{TeamID: 1}, {TeamID: 2}, {TeamID: 3}, {TeamID: 4},
	}
	remaining := []domain.Fixture{
		{HomeTeamID: 1, AwayTeamID: 2},
		{HomeTeamID: 3, AwayTeamID: 4},
	}
	forms := []domain.TeamForm{{TeamID: 1}, {TeamID: 2}, {TeamID: 3}, {TeamID: 4}}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	preds := predictor.Predict(ctx, 1, 5, standings, remaining, teamSlice, forms)
	if len(preds) == 0 {
		t.Fatalf("expected predictions slice even when context is cancelled")
	}
	// All teams should be present — Predict returns one row per team
	// regardless of how many runs executed.
	for _, p := range preds {
		if errors.Is(ctx.Err(), context.Canceled) && p.SeasonID != 1 {
			t.Fatalf("expected season_id=1, got %d", p.SeasonID)
		}
	}
}

// TestPredictionServiceDelegates — the read-side facade must just
// forward to the repository without altering the data.
func TestPredictionServiceDelegates(t *testing.T) {
	t.Parallel()

	want := []domain.PredictionWithTeam{
		{Prediction: domain.Prediction{TeamID: 1, ChampionshipProbability: 42}, TeamName: "A"},
	}
	repo := &mocks.PredictionRepoMock{
		GetLatestFunc: func(_ context.Context, _ int) ([]domain.PredictionWithTeam, error) {
			return want, nil
		},
		GetByWeekFunc: func(_ context.Context, _, _ int) ([]domain.PredictionWithTeam, error) {
			return want, nil
		},
	}
	svc := NewPredictionService(repo)

	got, err := svc.GetLatest(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetLatest: %v", err)
	}
	if len(got) != 1 || got[0].TeamID != 1 {
		t.Fatalf("GetLatest returned %+v", got)
	}

	got, err = svc.GetByWeek(context.Background(), 1, 4)
	if err != nil {
		t.Fatalf("GetByWeek: %v", err)
	}
	if len(got) != 1 || got[0].ChampionshipProbability != 42 {
		t.Fatalf("GetByWeek returned %+v", got)
	}
}
