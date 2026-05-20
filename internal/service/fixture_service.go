package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Anill17/league-api/internal/domain"
	"github.com/Anill17/league-api/internal/repository"
)

// GenerateSchedule produces a double round-robin schedule for the
// supplied teams using the classical circle method. For 4 teams it
// returns exactly 12 fixtures spread across 6 weeks (2 per week),
// with each pair playing once at home and once away.
//
// The function is pure (no I/O) so it is trivially testable.
func GenerateSchedule(seasonID int, teamIDs []int) []domain.Fixture {
	n := len(teamIDs)
	if n < 2 || n%2 != 0 {
		return nil
	}

	positions := append([]int(nil), teamIDs...)
	roundsPerHalf := n - 1
	matchesPerRound := n / 2

	firstHalf := make([][2]int, 0, roundsPerHalf*matchesPerRound)
	for r := 0; r < roundsPerHalf; r++ {
		for i := 0; i < matchesPerRound; i++ {
			home := positions[i]
			away := positions[n-1-i]
			// Alternate home/away each round for the games that
			// don't involve the fixed slot, so each team's
			// home/away split is even across the season.
			if i > 0 && r%2 == 1 {
				home, away = away, home
			}
			firstHalf = append(firstHalf, [2]int{home, away})
		}
		// Rotate: keep positions[0] fixed, shift the rest clockwise.
		last := positions[n-1]
		copy(positions[2:], positions[1:n-1])
		positions[1] = last
	}

	fixtures := make([]domain.Fixture, 0, 2*len(firstHalf))
	// First half: weeks 1..roundsPerHalf
	for r := 0; r < roundsPerHalf; r++ {
		for m := 0; m < matchesPerRound; m++ {
			pair := firstHalf[r*matchesPerRound+m]
			fixtures = append(fixtures, domain.Fixture{
				SeasonID:   seasonID,
				Week:       r + 1,
				HomeTeamID: pair[0],
				AwayTeamID: pair[1],
			})
		}
	}
	// Second half: weeks (roundsPerHalf+1)..2*roundsPerHalf, reversed.
	for r := 0; r < roundsPerHalf; r++ {
		for m := 0; m < matchesPerRound; m++ {
			pair := firstHalf[r*matchesPerRound+m]
			fixtures = append(fixtures, domain.Fixture{
				SeasonID:   seasonID,
				Week:       roundsPerHalf + r + 1,
				HomeTeamID: pair[1],
				AwayTeamID: pair[0],
			})
		}
	}
	return fixtures
}

// FixtureService handles mutations to fixtures. EditFixture is the
// only externally-visible operation; it must run inside a single
// SERIALIZABLE transaction so the recalculation of all standings
// stays consistent (project rule, section 3).
type FixtureService struct {
	fixtures    domain.FixtureRepository
	standings   domain.StandingsRepository
	predictions domain.PredictionRepository
	seasons     domain.SeasonRepository
	teams       domain.TeamRepository
	forms       domain.FormRepository
	calculator  domain.StandingsCalculator
	predictor   domain.ChampionshipPredictor
	tx          domain.TxManager
}

// NewFixtureService wires every dependency required for EditFixture.
func NewFixtureService(
	fixtures domain.FixtureRepository,
	standings domain.StandingsRepository,
	predictions domain.PredictionRepository,
	seasons domain.SeasonRepository,
	teams domain.TeamRepository,
	forms domain.FormRepository,
	calculator domain.StandingsCalculator,
	predictor domain.ChampionshipPredictor,
	tx domain.TxManager,
) *FixtureService {
	return &FixtureService{
		fixtures:    fixtures,
		standings:   standings,
		predictions: predictions,
		seasons:     seasons,
		teams:       teams,
		forms:       forms,
		calculator:  calculator,
		predictor:   predictor,
		tx:          tx,
	}
}

var _ domain.FixtureService = (*FixtureService)(nil)

// PredictionStartWeek is the first week after which the predictor is
// triggered, per project rule (section 9: "after week 4"). Edits to
// already-played fixtures only re-run predictions if the current week
// has already reached this threshold.
const PredictionStartWeek = 4

// Edit applies a new scoreline to an already-existing fixture and
// recomputes every downstream artifact (standings + predictions).
// The whole thing happens in one SERIALIZABLE transaction.
func (s *FixtureService) Edit(
	ctx context.Context,
	fixtureID, homeGoals, awayGoals int,
) (*domain.EditFixtureResult, error) {
	var result *domain.EditFixtureResult

	err := s.tx.WithSerializableTransaction(ctx, func(tx pgx.Tx) error {
		fix, err := s.fixtures.LockByID(ctx, tx, fixtureID)
		if err != nil {
			return err
		}

		now := time.Now().UTC()
		hg, ag := homeGoals, awayGoals
		fix.HomeGoals = &hg
		fix.AwayGoals = &ag
		fix.Played = true
		fix.PlayedAt = &now
		if err := s.fixtures.Update(ctx, tx, fix); err != nil {
			return err
		}

		teams, err := s.teams.List(ctx)
		if err != nil {
			return err
		}
		played, err := s.fixtures.GetPlayed(ctx, fix.SeasonID)
		if err != nil {
			return err
		}
		rows := s.calculator.Calculate(fix.SeasonID, teams, played)
		if err := s.standings.BulkReplace(ctx, tx, fix.SeasonID, rows); err != nil {
			return err
		}

		season, err := s.seasons.GetByID(ctx, fix.SeasonID)
		if err != nil {
			return err
		}
		predictions, err := s.refreshPredictions(ctx, tx, season, rows, teams)
		if err != nil {
			return err
		}

		decorated, err := decorateFixture(ctx, s.teams, *fix)
		if err != nil {
			return err
		}
		standingsView, err := s.standings.GetBySeasonWithTeam(ctx, fix.SeasonID)
		if err != nil {
			return err
		}
		result = &domain.EditFixtureResult{
			Fixture:     decorated,
			Standings:   standingsView,
			Predictions: predictions,
		}
		return nil
	})
	if err != nil {
		return nil, mapEditError(err)
	}
	return result, nil
}

// refreshPredictions re-runs the championship predictor for the
// current week if (and only if) the season has reached
// PredictionStartWeek. Returns the decorated predictions (with team
// names) for the response payload.
func (s *FixtureService) refreshPredictions(
	ctx context.Context,
	tx pgx.Tx,
	season *domain.Season,
	standings []domain.Standing,
	teams []domain.Team,
) ([]domain.PredictionWithTeam, error) {
	if season.CurrentWeek < PredictionStartWeek {
		return nil, nil
	}

	remaining, err := s.fixtures.GetRemaining(ctx, season.ID)
	if err != nil {
		return nil, err
	}
	forms, err := s.forms.GetBySeason(ctx, season.ID)
	if err != nil {
		return nil, err
	}
	preds := s.predictor.Predict(ctx, season.ID, season.CurrentWeek, standings, remaining, teams, forms)
	if err := s.predictions.DeleteForWeek(ctx, tx, season.ID, season.CurrentWeek); err != nil {
		return nil, err
	}
	if err := s.predictions.BulkInsert(ctx, tx, preds); err != nil {
		return nil, err
	}
	return s.predictions.GetByWeek(ctx, season.ID, season.CurrentWeek)
}

func mapEditError(err error) error {
	if errors.Is(err, repository.ErrNotFound) {
		return fmt.Errorf("edit fixture: %w", err)
	}
	return err
}

// =====================================================================
// Decoration helpers — used by every service that returns fixtures or
// predictions to the API. Centralised here so DRY is preserved.
// =====================================================================

// decorateFixture loads the two team names and returns a FixtureWithTeams.
// Used when only one fixture needs decorating (e.g. Edit endpoint).
func decorateFixture(
	ctx context.Context,
	teams domain.TeamRepository,
	f domain.Fixture,
) (domain.FixtureWithTeams, error) {
	home, err := teams.GetByID(ctx, f.HomeTeamID)
	if err != nil {
		return domain.FixtureWithTeams{}, err
	}
	away, err := teams.GetByID(ctx, f.AwayTeamID)
	if err != nil {
		return domain.FixtureWithTeams{}, err
	}
	return domain.FixtureWithTeams{
		Fixture:      f,
		HomeTeamName: home.Name,
		AwayTeamName: away.Name,
	}, nil
}

// decorateFixtures batches the lookup via a name map so we issue one
// "list teams" query for many fixtures instead of N.
func decorateFixtures(fixtures []domain.Fixture, teamNames map[int]string) []domain.FixtureWithTeams {
	out := make([]domain.FixtureWithTeams, 0, len(fixtures))
	for _, f := range fixtures {
		out = append(out, domain.FixtureWithTeams{
			Fixture:      f,
			HomeTeamName: teamNames[f.HomeTeamID],
			AwayTeamName: teamNames[f.AwayTeamID],
		})
	}
	return out
}

// teamNameMap builds the id→name lookup used by decorateFixtures.
func teamNameMap(teams []domain.Team) map[int]string {
	m := make(map[int]string, len(teams))
	for _, t := range teams {
		m[t.ID] = t.Name
	}
	return m
}
