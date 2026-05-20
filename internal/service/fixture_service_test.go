package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Anill17/league-api/internal/domain"
	"github.com/Anill17/league-api/internal/repository"
	"github.com/Anill17/league-api/mocks"
)

// TestGenerateSchedule_StructuralInvariants covers the structural
// guarantees of a 4-team double round-robin: 12 fixtures, 2 per week,
// each pair plays exactly twice, each team has 3 home and 3 away games.
func TestGenerateSchedule_StructuralInvariants(t *testing.T) {
	t.Parallel()

	teamIDs := []int{1, 2, 3, 4}
	fixtures := GenerateSchedule(99, teamIDs)

	if len(fixtures) != 12 {
		t.Fatalf("expected 12 fixtures, got %d", len(fixtures))
	}

	// 2 matches per week, weeks 1..6.
	perWeek := map[int]int{}
	homeCount := map[int]int{}
	awayCount := map[int]int{}
	pairCount := map[[2]int]int{}
	for _, f := range fixtures {
		if f.SeasonID != 99 {
			t.Fatalf("fixture season_id %d, want 99", f.SeasonID)
		}
		if f.Week < 1 || f.Week > 6 {
			t.Fatalf("fixture week %d out of range", f.Week)
		}
		if f.HomeTeamID == f.AwayTeamID {
			t.Fatalf("fixture self-play: %+v", f)
		}
		perWeek[f.Week]++
		homeCount[f.HomeTeamID]++
		awayCount[f.AwayTeamID]++
		pair := normalizePair(f.HomeTeamID, f.AwayTeamID)
		pairCount[pair]++
	}

	for w := 1; w <= 6; w++ {
		if perWeek[w] != 2 {
			t.Fatalf("week %d: got %d fixtures, want 2", w, perWeek[w])
		}
	}
	for _, id := range teamIDs {
		if homeCount[id] != 3 {
			t.Fatalf("team %d home count %d, want 3", id, homeCount[id])
		}
		if awayCount[id] != 3 {
			t.Fatalf("team %d away count %d, want 3", id, awayCount[id])
		}
	}
	for pair, count := range pairCount {
		if count != 2 {
			t.Fatalf("pair %v played %d times, want 2", pair, count)
		}
	}
}

// TestGenerateSchedule_DoubleRoundRobinPairs — every distinct pair
// (i, j) with i < j has one home and one away match.
func TestGenerateSchedule_DoubleRoundRobinPairs(t *testing.T) {
	t.Parallel()
	teamIDs := []int{1, 2, 3, 4}
	fixtures := GenerateSchedule(1, teamIDs)

	type homeAwayCount struct{ home, away int }
	pairs := map[[2]int]homeAwayCount{}
	for _, f := range fixtures {
		key := normalizePair(f.HomeTeamID, f.AwayTeamID)
		c := pairs[key]
		if f.HomeTeamID == key[0] {
			c.home++
		} else {
			c.away++
		}
		pairs[key] = c
	}
	for pair, c := range pairs {
		if c.home != 1 || c.away != 1 {
			t.Fatalf("pair %v: home %d away %d, want 1/1", pair, c.home, c.away)
		}
	}
}

func TestGenerateSchedule_RejectsOdd(t *testing.T) {
	t.Parallel()
	if got := GenerateSchedule(1, []int{1, 2, 3}); got != nil {
		t.Fatalf("expected nil for odd team count, got %v", got)
	}
	if got := GenerateSchedule(1, nil); got != nil {
		t.Fatalf("expected nil for empty team count, got %v", got)
	}
}

// TestFixtureService_Edit_HappyPath — Edit must lock, update,
// recalculate standings, replace them, fetch the season, and skip the
// predictor when the current week is below the threshold.
func TestFixtureService_Edit_HappyPath(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	fixID := 7
	seasonID := 1
	original := &domain.Fixture{
		ID: fixID, SeasonID: seasonID, Week: 2,
		HomeTeamID: 1, AwayTeamID: 2,
		HomeGoals: ptr(2), AwayGoals: ptr(1), Played: true,
	}

	fixRepo := &mocks.FixtureRepoMock{
		LockByIDFunc: func(_ context.Context, _ pgx.Tx, id int) (*domain.Fixture, error) {
			if id != fixID {
				t.Fatalf("LockByID id %d", id)
			}
			return original, nil
		},
		GetPlayedFunc: func(_ context.Context, _ int) ([]domain.Fixture, error) {
			return []domain.Fixture{*original}, nil
		},
	}
	standingsRepo := &mocks.StandingsRepoMock{
		GetBySeasonWithTeamFunc: func(_ context.Context, _ int) ([]domain.StandingWithTeam, error) {
			return []domain.StandingWithTeam{{Standing: domain.Standing{TeamID: 1}}}, nil
		},
	}
	teamRepo := &mocks.TeamRepoMock{
		ListFunc: func(_ context.Context) ([]domain.Team, error) { return teamSlice, nil },
		GetByIDFunc: func(_ context.Context, id int) (*domain.Team, error) {
			return &domain.Team{ID: id, Name: "T"}, nil
		},
	}
	seasonRepo := &mocks.SeasonRepoMock{
		GetByIDFunc: func(_ context.Context, _ int) (*domain.Season, error) {
			return &domain.Season{ID: seasonID, CurrentWeek: 2}, nil
		},
	}
	predRepo := &mocks.PredictionRepoMock{}
	formRepo := &mocks.FormRepoMock{}
	predictor := &mocks.ChampionshipPredictorMock{}

	svc := NewFixtureService(
		fixRepo, standingsRepo, predRepo, seasonRepo,
		teamRepo, formRepo,
		NewStandingsService(), predictor,
		&mocks.TxManagerMock{},
	)

	out, err := svc.Edit(ctx, fixID, 4, 0)
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}
	if out == nil {
		t.Fatalf("nil result")
	}

	// Fixture was updated in place.
	if original.HomeGoals == nil || *original.HomeGoals != 4 {
		t.Fatalf("home goals: got %v, want 4", original.HomeGoals)
	}
	if original.AwayGoals == nil || *original.AwayGoals != 0 {
		t.Fatalf("away goals: got %v, want 0", original.AwayGoals)
	}
	if original.PlayedAt == nil || time.Since(*original.PlayedAt) > time.Minute {
		t.Fatalf("PlayedAt not set: %v", original.PlayedAt)
	}

	if fixRepo.UpdateCalls != 1 {
		t.Fatalf("Update calls: %d", fixRepo.UpdateCalls)
	}
	if standingsRepo.BulkReplaceCalls != 1 {
		t.Fatalf("BulkReplace calls: %d", standingsRepo.BulkReplaceCalls)
	}
	if predictor.PredictCalls != 0 {
		t.Fatalf("Predictor should not run pre-week-4, got %d", predictor.PredictCalls)
	}
}

// TestFixtureService_Edit_RunsPredictorAtWeek4 — once the current week
// hits the threshold, the predictor must run and predictions
// are replaced.
func TestFixtureService_Edit_RunsPredictorAtWeek4(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	fixRepo := &mocks.FixtureRepoMock{
		LockByIDFunc: func(_ context.Context, _ pgx.Tx, _ int) (*domain.Fixture, error) {
			return &domain.Fixture{ID: 1, SeasonID: 1, Week: 4, HomeTeamID: 1, AwayTeamID: 2}, nil
		},
		GetPlayedFunc:    func(_ context.Context, _ int) ([]domain.Fixture, error) { return nil, nil },
		GetRemainingFunc: func(_ context.Context, _ int) ([]domain.Fixture, error) { return nil, nil },
	}
	standingsRepo := &mocks.StandingsRepoMock{}
	teamRepo := &mocks.TeamRepoMock{
		ListFunc:    func(_ context.Context) ([]domain.Team, error) { return teamSlice, nil },
		GetByIDFunc: func(_ context.Context, id int) (*domain.Team, error) { return &domain.Team{ID: id}, nil },
	}
	seasonRepo := &mocks.SeasonRepoMock{
		GetByIDFunc: func(_ context.Context, _ int) (*domain.Season, error) {
			return &domain.Season{ID: 1, CurrentWeek: PredictionStartWeek}, nil
		},
	}
	predRepo := &mocks.PredictionRepoMock{}
	formRepo := &mocks.FormRepoMock{}
	predictor := &mocks.ChampionshipPredictorMock{
		PredictFunc: func(_ context.Context, sID, wk int, _ []domain.Standing, _ []domain.Fixture, _ []domain.Team, _ []domain.TeamForm) []domain.Prediction {
			return []domain.Prediction{{SeasonID: sID, Week: wk, TeamID: 1, ChampionshipProbability: 50}}
		},
	}

	svc := NewFixtureService(
		fixRepo, standingsRepo, predRepo, seasonRepo,
		teamRepo, formRepo,
		NewStandingsService(), predictor,
		&mocks.TxManagerMock{},
	)

	if _, err := svc.Edit(ctx, 1, 1, 1); err != nil {
		t.Fatalf("Edit: %v", err)
	}
	if predictor.PredictCalls != 1 {
		t.Fatalf("Predict calls: %d, want 1", predictor.PredictCalls)
	}
	if predRepo.DeleteForWeekCalls != 1 {
		t.Fatalf("DeleteForWeek calls: %d", predRepo.DeleteForWeekCalls)
	}
	if predRepo.BulkInsertCalls != 1 {
		t.Fatalf("BulkInsert calls: %d", predRepo.BulkInsertCalls)
	}
}

// TestFixtureService_Edit_NotFound — LockByID returning ErrNotFound
// propagates as the same sentinel so handlers can map it to 404.
func TestFixtureService_Edit_NotFound(t *testing.T) {
	t.Parallel()

	fixRepo := &mocks.FixtureRepoMock{
		LockByIDFunc: func(_ context.Context, _ pgx.Tx, _ int) (*domain.Fixture, error) {
			return nil, repository.ErrNotFound
		},
	}
	svc := NewFixtureService(
		fixRepo, &mocks.StandingsRepoMock{}, &mocks.PredictionRepoMock{},
		&mocks.SeasonRepoMock{}, &mocks.TeamRepoMock{}, &mocks.FormRepoMock{},
		NewStandingsService(), &mocks.ChampionshipPredictorMock{},
		&mocks.TxManagerMock{},
	)

	_, err := svc.Edit(context.Background(), 99, 0, 0)
	if !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// normalizePair returns the (small, large) tuple of two team ids so a
// fixture's pair is comparable regardless of who was home.
func normalizePair(a, b int) [2]int {
	if a < b {
		return [2]int{a, b}
	}
	return [2]int{b, a}
}
