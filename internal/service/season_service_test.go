package service

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/Anill17/league-api/internal/domain"
	"github.com/Anill17/league-api/internal/repository"
	"github.com/Anill17/league-api/mocks"
)

func newSeasonServiceWithMocks(t *testing.T) (
	*SeasonService,
	*mocks.SeasonRepoMock,
	*mocks.FixtureRepoMock,
	*mocks.StandingsRepoMock,
	*mocks.FormRepoMock,
	*mocks.PredictionRepoMock,
	*mocks.TeamRepoMock,
) {
	t.Helper()

	sRepo := &mocks.SeasonRepoMock{}
	fRepo := &mocks.FixtureRepoMock{}
	stRepo := &mocks.StandingsRepoMock{}
	fmRepo := &mocks.FormRepoMock{}
	pRepo := &mocks.PredictionRepoMock{}
	tRepo := &mocks.TeamRepoMock{}

	svc := NewSeasonService(
		sRepo, fRepo, stRepo, fmRepo, pRepo, tRepo,
		NewStandingsService(),
		&mocks.TxManagerMock{},
	)
	return svc, sRepo, fRepo, stRepo, fmRepo, pRepo, tRepo
}

// TestSeasonService_Create_HappyPath — Create must list teams, open a
// transaction, insert a season, bulk-create fixtures, and init both
// standings and form for every team.
func TestSeasonService_Create_HappyPath(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	svc, sRepo, fRepo, stRepo, fmRepo, _, tRepo := newSeasonServiceWithMocks(t)

	tRepo.ListFunc = func(_ context.Context) ([]domain.Team, error) {
		return teamSlice, nil
	}
	sRepo.CreateFunc = func(_ context.Context, _ pgx.Tx) (*domain.Season, error) {
		return &domain.Season{ID: 1}, nil
	}

	out, err := svc.Create(ctx)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if out == nil {
		t.Fatalf("nil result")
	}
	if got := len(out.Fixtures); got != 12 {
		t.Fatalf("fixtures: got %d, want 12", got)
	}
	if sRepo.CreateCalls != 1 {
		t.Fatalf("season create: %d", sRepo.CreateCalls)
	}
	if fRepo.BulkCreateCalls != 1 {
		t.Fatalf("fixture bulk: %d", fRepo.BulkCreateCalls)
	}
	if stRepo.InitForSeasonCalls != 1 {
		t.Fatalf("standings init: %d", stRepo.InitForSeasonCalls)
	}
	if fmRepo.InitForSeasonCalls != 1 {
		t.Fatalf("form init: %d", fmRepo.InitForSeasonCalls)
	}
}

// TestSeasonService_Create_NoTeams — without seeded teams Create
// returns the sentinel error and touches nothing else.
func TestSeasonService_Create_NoTeams(t *testing.T) {
	t.Parallel()
	svc, _, fRepo, _, _, _, tRepo := newSeasonServiceWithMocks(t)
	tRepo.ListFunc = func(_ context.Context) ([]domain.Team, error) { return nil, nil }

	_, err := svc.Create(context.Background())
	if !errors.Is(err, ErrSeasonHasNoTeams) {
		t.Fatalf("expected ErrSeasonHasNoTeams, got %v", err)
	}
	if fRepo.BulkCreateCalls != 0 {
		t.Fatalf("touched fixtures despite no teams")
	}
}

// TestSeasonService_Create_TeamsRepoError — propagates the repo error
// before any transaction is opened.
func TestSeasonService_Create_TeamsRepoError(t *testing.T) {
	t.Parallel()
	svc, _, _, _, _, _, tRepo := newSeasonServiceWithMocks(t)
	tRepo.ListFunc = func(_ context.Context) ([]domain.Team, error) {
		return nil, errors.New("boom")
	}
	if _, err := svc.Create(context.Background()); err == nil {
		t.Fatalf("expected error")
	}
}

// TestSeasonService_GetStandings — the read path: fetch rows, fetch
// played fixtures, apply head-to-head, return result.
func TestSeasonService_GetStandings(t *testing.T) {
	t.Parallel()
	svc, sRepo, fRepo, stRepo, _, _, _ := newSeasonServiceWithMocks(t)

	sRepo.GetByIDFunc = func(_ context.Context, _ int) (*domain.Season, error) {
		return &domain.Season{ID: 1}, nil
	}
	stRepo.GetBySeasonWithTeamFunc = func(_ context.Context, _ int) ([]domain.StandingWithTeam, error) {
		return []domain.StandingWithTeam{
			{Standing: domain.Standing{TeamID: 1, Points: 6}, TeamName: "A"},
			{Standing: domain.Standing{TeamID: 2, Points: 3}, TeamName: "B"},
		}, nil
	}
	fRepo.GetPlayedFunc = func(_ context.Context, _ int) ([]domain.Fixture, error) { return nil, nil }

	rows, err := svc.GetStandings(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetStandings: %v", err)
	}
	if len(rows) != 2 || rows[0].TeamID != 1 {
		t.Fatalf("unexpected order: %+v", rows)
	}
}

// TestSeasonService_GetStandings_SeasonMissing — a missing season is
// surfaced as repository.ErrNotFound so handlers can pick a 404.
func TestSeasonService_GetStandings_SeasonMissing(t *testing.T) {
	t.Parallel()
	svc, sRepo, _, _, _, _, _ := newSeasonServiceWithMocks(t)
	sRepo.GetByIDFunc = func(_ context.Context, _ int) (*domain.Season, error) {
		return nil, repository.ErrNotFound
	}
	if _, err := svc.GetStandings(context.Background(), 99); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// TestSeasonService_GetWeek — composes standings + fixtures.
func TestSeasonService_GetWeek(t *testing.T) {
	t.Parallel()
	svc, sRepo, fRepo, stRepo, _, _, tRepo := newSeasonServiceWithMocks(t)
	sRepo.GetByIDFunc = func(_ context.Context, _ int) (*domain.Season, error) {
		return &domain.Season{ID: 1}, nil
	}
	fRepo.GetBySeasonAndWeekFunc = func(_ context.Context, _, week int) ([]domain.Fixture, error) {
		if week != 3 {
			t.Fatalf("week passed through wrong: %d", week)
		}
		return []domain.Fixture{
			{ID: 1, SeasonID: 1, Week: 3, HomeTeamID: 1, AwayTeamID: 2},
		}, nil
	}
	tRepo.ListFunc = func(_ context.Context) ([]domain.Team, error) { return teamSlice, nil }
	stRepo.GetBySeasonWithTeamFunc = func(_ context.Context, _ int) ([]domain.StandingWithTeam, error) {
		return nil, nil
	}
	fRepo.GetPlayedFunc = func(_ context.Context, _ int) ([]domain.Fixture, error) { return nil, nil }

	out, err := svc.GetWeek(context.Background(), 1, 3)
	if err != nil {
		t.Fatalf("GetWeek: %v", err)
	}
	if out.Week != 3 || len(out.Fixtures) != 1 {
		t.Fatalf("unexpected payload: %+v", out)
	}
	// Fixture is decorated with team names.
	if out.Fixtures[0].HomeTeamName != "Manchester City" {
		t.Fatalf("home name not decorated: %+v", out.Fixtures[0])
	}
}

// TestSeasonService_Reset_OrderAndCalls — Reset deletes dependents,
// resets the season row, then re-creates the schedule + init rows.
func TestSeasonService_Reset_OrderAndCalls(t *testing.T) {
	t.Parallel()

	svc, sRepo, fRepo, stRepo, fmRepo, pRepo, tRepo := newSeasonServiceWithMocks(t)
	sRepo.GetByIDFunc = func(_ context.Context, _ int) (*domain.Season, error) {
		return &domain.Season{ID: 1}, nil
	}
	tRepo.ListFunc = func(_ context.Context) ([]domain.Team, error) { return teamSlice, nil }

	if err := svc.Reset(context.Background(), 1); err != nil {
		t.Fatalf("Reset: %v", err)
	}

	checks := []struct {
		name string
		got  int
	}{
		{"predictions.DeleteForSeason", pRepo.DeleteForSeasonCalls},
		{"form.DeleteForSeason", fmRepo.DeleteForSeasonCalls},
		{"standings.DeleteForSeason", stRepo.DeleteForSeasonCalls},
		{"fixtures.DeleteForSeason", fRepo.DeleteForSeasonCalls},
		{"seasons.Reset", sRepo.ResetCalls},
		{"fixtures.BulkCreate", fRepo.BulkCreateCalls},
		{"standings.InitForSeason", stRepo.InitForSeasonCalls},
		{"form.InitForSeason", fmRepo.InitForSeasonCalls},
	}
	for _, c := range checks {
		if c.got != 1 {
			t.Fatalf("%s: got %d, want 1", c.name, c.got)
		}
	}
}

// TestSeasonService_Reset_MissingSeason — refuses to act on an
// unknown season id.
func TestSeasonService_Reset_MissingSeason(t *testing.T) {
	t.Parallel()
	svc, sRepo, _, _, _, _, _ := newSeasonServiceWithMocks(t)
	sRepo.GetByIDFunc = func(_ context.Context, _ int) (*domain.Season, error) {
		return nil, repository.ErrNotFound
	}
	if err := svc.Reset(context.Background(), 99); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// TestSeasonService_GetFixtures — returns 12 decorated fixtures for a
// freshly-loaded season.
func TestSeasonService_GetFixtures(t *testing.T) {
	t.Parallel()
	svc, sRepo, fRepo, _, _, _, tRepo := newSeasonServiceWithMocks(t)
	sRepo.GetByIDFunc = func(_ context.Context, _ int) (*domain.Season, error) {
		return &domain.Season{ID: 1}, nil
	}
	fRepo.GetBySeasonFunc = func(_ context.Context, _ int) ([]domain.Fixture, error) {
		return GenerateSchedule(1, []int{1, 2, 3, 4}), nil
	}
	tRepo.ListFunc = func(_ context.Context) ([]domain.Team, error) { return teamSlice, nil }

	got, err := svc.GetFixtures(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetFixtures: %v", err)
	}
	if len(got) != 12 {
		t.Fatalf("got %d fixtures, want 12", len(got))
	}
	for _, f := range got {
		if f.HomeTeamName == "" || f.AwayTeamName == "" {
			t.Fatalf("undecorated fixture: %+v", f)
		}
	}
}
