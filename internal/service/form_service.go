package service

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/Anill17/league-api/internal/domain"
)

// Form multiplier constants — averaged across the team's last
// domain.MaxRecentResults outcomes by the Poisson xG model.
const (
	formWeightWin  = 1.1
	formWeightDraw = 1.0
	formWeightLoss = 0.9
)

// FormService is the single owner of team form mutations.
type FormService struct {
	forms domain.FormRepository
}

// NewFormService wires a FormRepository into the service.
func NewFormService(forms domain.FormRepository) *FormService {
	return &FormService{forms: forms}
}

// RecordResult appends result to the team's recent results, truncates
// to the last domain.MaxRecentResults entries, and persists. The
// passed-in *TeamForm is mutated in place so callers may continue to
// use it for the rest of the in-flight transaction without re-reading.
func (s *FormService) RecordResult(
	ctx context.Context,
	tx pgx.Tx,
	form *domain.TeamForm,
	result string,
) error {
	if form == nil {
		return fmt.Errorf("form: nil TeamForm")
	}
	if !isValidResult(result) {
		return fmt.Errorf("form: invalid result %q", result)
	}

	form.RecentResults = append(form.RecentResults, result)
	if len(form.RecentResults) > domain.MaxRecentResults {
		form.RecentResults = form.RecentResults[len(form.RecentResults)-domain.MaxRecentResults:]
	}
	return s.forms.Upsert(ctx, tx, form)
}

// FormMultiplier returns the average form weight over the team's
// recent results. With no recorded results it returns 1.0 (neutral).
// This is the single source of truth referenced by the simulator.
func FormMultiplier(form domain.TeamForm) float64 {
	if len(form.RecentResults) == 0 {
		return 1.0
	}
	var sum float64
	for _, r := range form.RecentResults {
		switch r {
		case domain.ResultWin:
			sum += formWeightWin
		case domain.ResultDraw:
			sum += formWeightDraw
		case domain.ResultLoss:
			sum += formWeightLoss
		}
	}
	return sum / float64(len(form.RecentResults))
}

// ResultFromScore returns the (home, away) W/D/L pair from a final
// score line. Lives here because form_service is the package that
// uses W/D/L; nothing else needs to know.
func ResultFromScore(homeGoals, awayGoals int) (homeResult, awayResult string) {
	switch {
	case homeGoals > awayGoals:
		return domain.ResultWin, domain.ResultLoss
	case homeGoals < awayGoals:
		return domain.ResultLoss, domain.ResultWin
	default:
		return domain.ResultDraw, domain.ResultDraw
	}
}

func isValidResult(r string) bool {
	switch r {
	case domain.ResultWin, domain.ResultDraw, domain.ResultLoss:
		return true
	default:
		return false
	}
}
