package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/insider/league-api/internal/domain"
)

// ErrSeasonHasNoTeams is returned when an attempt is made to create a
// season before the teams table has been seeded.
var ErrSeasonHasNoTeams = errors.New("season: no teams seeded")

// SeasonService owns the lifecycle of a season and the read-only
// projections (standings / fixtures / week view).
type SeasonService struct {
	seasons     domain.SeasonRepository
	fixtures    domain.FixtureRepository
	standings   domain.StandingsRepository
	form        domain.FormRepository
	predictions domain.PredictionRepository
	teams       domain.TeamRepository
	sorter      domain.StandingsSorter
	tx          domain.TxManager
}

// NewSeasonService wires every dependency required.
func NewSeasonService(
	seasons domain.SeasonRepository,
	fixtures domain.FixtureRepository,
	standings domain.StandingsRepository,
	form domain.FormRepository,
	predictions domain.PredictionRepository,
	teams domain.TeamRepository,
	sorter domain.StandingsSorter,
	tx domain.TxManager,
) *SeasonService {
	return &SeasonService{
		seasons:     seasons,
		fixtures:    fixtures,
		standings:   standings,
		form:        form,
		predictions: predictions,
		teams:       teams,
		sorter:      sorter,
		tx:          tx,
	}
}

var _ domain.SeasonService = (*SeasonService)(nil)

// Create starts a fresh season: insert the season row, generate the
// 12-fixture schedule, init standings and form rows for all teams.
// All of it happens in one transaction.
func (s *SeasonService) Create(ctx context.Context) (*domain.SeasonWithFixtures, error) {
	teams, err := s.teams.List(ctx)
	if err != nil {
		return nil, err
	}
	if len(teams) == 0 {
		return nil, ErrSeasonHasNoTeams
	}

	teamIDs := make([]int, 0, len(teams))
	for _, t := range teams {
		teamIDs = append(teamIDs, t.ID)
	}

	var (
		season   *domain.Season
		fixtures []domain.Fixture
	)
	err = s.tx.WithTransaction(ctx, func(tx pgx.Tx) error {
		season, err = s.seasons.Create(ctx, tx)
		if err != nil {
			return err
		}
		fixtures = GenerateSchedule(season.ID, teamIDs)
		if err := s.fixtures.BulkCreate(ctx, tx, fixtures); err != nil {
			return err
		}
		if err := s.standings.InitForSeason(ctx, tx, season.ID, teamIDs); err != nil {
			return err
		}
		if err := s.form.InitForSeason(ctx, tx, season.ID, teamIDs); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("create season: %w", err)
	}

	nameMap := teamNameMap(teams)
	return &domain.SeasonWithFixtures{
		Season:   *season,
		Fixtures: decorateFixtures(fixtures, nameMap),
	}, nil
}

// Get returns a single season row by id.
func (s *SeasonService) Get(ctx context.Context, id int) (*domain.Season, error) {
	return s.seasons.GetByID(ctx, id)
}

// GetStandings returns the league table with head-to-head applied as
// the final tiebreaker.
func (s *SeasonService) GetStandings(
	ctx context.Context,
	seasonID int,
) ([]domain.StandingWithTeam, error) {
	if _, err := s.seasons.GetByID(ctx, seasonID); err != nil {
		return nil, err
	}
	rows, err := s.standings.GetBySeasonWithTeam(ctx, seasonID)
	if err != nil {
		return nil, err
	}
	played, err := s.fixtures.GetPlayed(ctx, seasonID)
	if err != nil {
		return nil, err
	}
	return s.sorter.SortWithHeadToHead(rows, played), nil
}

// GetFixtures returns every fixture for a season, decorated with team
// names. Order is by week then id, so handlers can group client-side.
func (s *SeasonService) GetFixtures(
	ctx context.Context,
	seasonID int,
) ([]domain.FixtureWithTeams, error) {
	if _, err := s.seasons.GetByID(ctx, seasonID); err != nil {
		return nil, err
	}
	fixtures, err := s.fixtures.GetBySeason(ctx, seasonID)
	if err != nil {
		return nil, err
	}
	teams, err := s.teams.List(ctx)
	if err != nil {
		return nil, err
	}
	return decorateFixtures(fixtures, teamNameMap(teams)), nil
}

// GetWeek returns the fixtures for a specific week and the standings
// snapshot AS OF NOW (not historical).
func (s *SeasonService) GetWeek(
	ctx context.Context,
	seasonID, week int,
) (*domain.WeekView, error) {
	if _, err := s.seasons.GetByID(ctx, seasonID); err != nil {
		return nil, err
	}
	fixtures, err := s.fixtures.GetBySeasonAndWeek(ctx, seasonID, week)
	if err != nil {
		return nil, err
	}
	teams, err := s.teams.List(ctx)
	if err != nil {
		return nil, err
	}
	standings, err := s.GetStandings(ctx, seasonID)
	if err != nil {
		return nil, err
	}
	return &domain.WeekView{
		Week:      week,
		Fixtures:  decorateFixtures(fixtures, teamNameMap(teams)),
		Standings: standings,
	}, nil
}

// Reset wipes a season's fixtures / standings / form / predictions and
// returns it to week 0. The fixtures table is repopulated with a fresh
// schedule so the same season id can be replayed.
//
// Implementation: delete + recreate inside one transaction. Cascading
// FKs handle most of the cleanup; the season row itself is kept (we
// only delete dependent rows).
func (s *SeasonService) Reset(ctx context.Context, seasonID int) error {
	if _, err := s.seasons.GetByID(ctx, seasonID); err != nil {
		return err
	}
	teams, err := s.teams.List(ctx)
	if err != nil {
		return err
	}
	teamIDs := make([]int, 0, len(teams))
	for _, t := range teams {
		teamIDs = append(teamIDs, t.ID)
	}

	return s.tx.WithTransaction(ctx, func(tx pgx.Tx) error {
		// Order matters only because predictions and form have no
		// FK back to fixtures/standings, but we keep dependents
		// first for clarity.
		if err := s.predictions.DeleteForSeason(ctx, tx, seasonID); err != nil {
			return err
		}
		if err := s.form.DeleteForSeason(ctx, tx, seasonID); err != nil {
			return err
		}
		if err := s.standings.DeleteForSeason(ctx, tx, seasonID); err != nil {
			return err
		}
		if err := s.fixtures.DeleteForSeason(ctx, tx, seasonID); err != nil {
			return err
		}
		if err := s.seasons.Reset(ctx, tx, seasonID); err != nil {
			return err
		}

		fixtures := GenerateSchedule(seasonID, teamIDs)
		if err := s.fixtures.BulkCreate(ctx, tx, fixtures); err != nil {
			return err
		}
		if err := s.standings.InitForSeason(ctx, tx, seasonID, teamIDs); err != nil {
			return err
		}
		if err := s.form.InitForSeason(ctx, tx, seasonID, teamIDs); err != nil {
			return err
		}
		return nil
	})
}
