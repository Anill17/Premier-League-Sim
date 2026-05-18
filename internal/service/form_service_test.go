package service

import (
	"context"
	"math"
	"testing"

	"github.com/insider/league-api/internal/domain"
	"github.com/insider/league-api/mocks"
)

func TestFormMultiplier(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		results []string
		want    float64
	}{
		{"empty", []string{}, 1.0},
		{"single_win", []string{domain.ResultWin}, 1.1},
		{"single_draw", []string{domain.ResultDraw}, 1.0},
		{"single_loss", []string{domain.ResultLoss}, 0.9},
		{"three_wins", []string{domain.ResultWin, domain.ResultWin, domain.ResultWin}, 1.1},
		{"three_losses", []string{domain.ResultLoss, domain.ResultLoss, domain.ResultLoss}, 0.9},
		{"WDL_avg", []string{domain.ResultWin, domain.ResultDraw, domain.ResultLoss}, 1.0},
		{"WWD_avg", []string{domain.ResultWin, domain.ResultWin, domain.ResultDraw}, (1.1 + 1.1 + 1.0) / 3.0},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := FormMultiplier(domain.TeamForm{RecentResults: c.results})
			if math.Abs(got-c.want) > 1e-9 {
				t.Fatalf("got %v, want %v", got, c.want)
			}
		})
	}
}

func TestResultFromScore(t *testing.T) {
	t.Parallel()
	cases := []struct {
		home, away int
		wantHome   string
		wantAway   string
	}{
		{2, 1, domain.ResultWin, domain.ResultLoss},
		{0, 3, domain.ResultLoss, domain.ResultWin},
		{1, 1, domain.ResultDraw, domain.ResultDraw},
		{0, 0, domain.ResultDraw, domain.ResultDraw},
	}
	for _, c := range cases {
		c := c
		t.Run("", func(t *testing.T) {
			t.Parallel()
			gotHome, gotAway := ResultFromScore(c.home, c.away)
			if gotHome != c.wantHome || gotAway != c.wantAway {
				t.Fatalf("score %d-%d: got %s/%s, want %s/%s",
					c.home, c.away, gotHome, gotAway, c.wantHome, c.wantAway)
			}
		})
	}
}

func TestRecordResult_AppendsAndTruncates(t *testing.T) {
	t.Parallel()

	repo := &mocks.FormRepoMock{}
	svc := NewFormService(repo)

	form := &domain.TeamForm{
		SeasonID: 1, TeamID: 2,
		RecentResults: []string{domain.ResultWin, domain.ResultDraw, domain.ResultLoss},
	}

	if err := svc.RecordResult(context.Background(), nil, form, domain.ResultWin); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got := len(form.RecentResults); got != domain.MaxRecentResults {
		t.Fatalf("len(RecentResults) = %d, want %d", got, domain.MaxRecentResults)
	}
	// Oldest (W) was dropped, newest (W) is at the tail.
	want := []string{domain.ResultDraw, domain.ResultLoss, domain.ResultWin}
	for i := range want {
		if form.RecentResults[i] != want[i] {
			t.Fatalf("position %d: got %s, want %s", i, form.RecentResults[i], want[i])
		}
	}
	if repo.UpsertCalls != 1 {
		t.Fatalf("expected one Upsert call, got %d", repo.UpsertCalls)
	}
}

func TestRecordResult_RejectsInvalid(t *testing.T) {
	t.Parallel()
	svc := NewFormService(&mocks.FormRepoMock{})
	err := svc.RecordResult(context.Background(), nil, &domain.TeamForm{}, "XYZ")
	if err == nil {
		t.Fatalf("expected error for invalid result")
	}
}

func TestRecordResult_RejectsNilForm(t *testing.T) {
	t.Parallel()
	svc := NewFormService(&mocks.FormRepoMock{})
	if err := svc.RecordResult(context.Background(), nil, nil, domain.ResultWin); err == nil {
		t.Fatalf("expected error for nil form")
	}
}
