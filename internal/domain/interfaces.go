package domain

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// =====================================================================
// Transaction management
// =====================================================================

// TxManager is the boundary between the service layer and the database
// transaction primitives. Services depend on this, not on *pgxpool.Pool,
// so they can be unit-tested with an in-memory fake.
type TxManager interface {
	// WithTransaction runs fn inside a READ COMMITTED transaction. The
	// transaction is committed if fn returns nil; otherwise it is rolled
	// back. Any panic inside fn is also rolled back and re-raised.
	WithTransaction(ctx context.Context, fn func(tx pgx.Tx) error) error

	// WithSerializableTransaction is identical to WithTransaction but
	// uses the SERIALIZABLE isolation level. Used by standings
	// recalculation per project rules.
	WithSerializableTransaction(ctx context.Context, fn func(tx pgx.Tx) error) error
}

// =====================================================================
// Repositories — split per Interface Segregation Principle.
// Services that only read depend on the *Reader; services that mutate
// depend on the composite *Repository.
// =====================================================================

// ---- Teams -----------------------------------------------------------

type TeamReader interface {
	GetByID(ctx context.Context, id int) (*Team, error)
	List(ctx context.Context) ([]Team, error)
}

type TeamRepository interface {
	TeamReader
}

// ---- Seasons ---------------------------------------------------------

type SeasonReader interface {
	GetByID(ctx context.Context, id int) (*Season, error)
}

type SeasonWriter interface {
	Create(ctx context.Context, tx pgx.Tx) (*Season, error)
	UpdateCurrentWeek(ctx context.Context, tx pgx.Tx, seasonID, week int) error
	MarkComplete(ctx context.Context, tx pgx.Tx, seasonID int) error
	// Reset clears the season row's state (current_week=0, is_complete=false)
	// without touching dependent tables.
	Reset(ctx context.Context, tx pgx.Tx, seasonID int) error
	Delete(ctx context.Context, tx pgx.Tx, seasonID int) error
}

type SeasonRepository interface {
	SeasonReader
	SeasonWriter
}

// ---- Fixtures --------------------------------------------------------

type FixtureReader interface {
	GetByID(ctx context.Context, id int) (*Fixture, error)
	GetBySeasonAndWeek(ctx context.Context, seasonID, week int) ([]Fixture, error)
	GetBySeason(ctx context.Context, seasonID int) ([]Fixture, error)
	GetPlayed(ctx context.Context, seasonID int) ([]Fixture, error)
	GetRemaining(ctx context.Context, seasonID int) ([]Fixture, error)
}

type FixtureWriter interface {
	Create(ctx context.Context, tx pgx.Tx, f *Fixture) error
	Update(ctx context.Context, tx pgx.Tx, f *Fixture) error
	BulkCreate(ctx context.Context, tx pgx.Tx, fixtures []Fixture) error
	// DeleteForSeason wipes every fixture row for a season.
	DeleteForSeason(ctx context.Context, tx pgx.Tx, seasonID int) error
	// LockForWeek selects and locks (FOR UPDATE) the unplayed fixtures of
	// a given week. Returned slice may be empty when all are played.
	LockForWeek(ctx context.Context, tx pgx.Tx, seasonID, week int) ([]Fixture, error)
	// LockByID selects and locks a single fixture row.
	LockByID(ctx context.Context, tx pgx.Tx, id int) (*Fixture, error)
}

type FixtureRepository interface {
	FixtureReader
	FixtureWriter
}

// ---- Standings -------------------------------------------------------

type StandingsReader interface {
	GetBySeason(ctx context.Context, seasonID int) ([]Standing, error)
	GetBySeasonWithTeam(ctx context.Context, seasonID int) ([]StandingWithTeam, error)
}

type StandingsWriter interface {
	InitForSeason(ctx context.Context, tx pgx.Tx, seasonID int, teamIDs []int) error
	Upsert(ctx context.Context, tx pgx.Tx, s *Standing) error
	BulkReplace(ctx context.Context, tx pgx.Tx, seasonID int, rows []Standing) error
	DeleteForSeason(ctx context.Context, tx pgx.Tx, seasonID int) error
}

type StandingsRepository interface {
	StandingsReader
	StandingsWriter
}

// ---- Team form -------------------------------------------------------

type FormReader interface {
	GetBySeasonAndTeam(ctx context.Context, seasonID, teamID int) (*TeamForm, error)
	GetBySeason(ctx context.Context, seasonID int) ([]TeamForm, error)
}

type FormWriter interface {
	InitForSeason(ctx context.Context, tx pgx.Tx, seasonID int, teamIDs []int) error
	Upsert(ctx context.Context, tx pgx.Tx, f *TeamForm) error
	DeleteForSeason(ctx context.Context, tx pgx.Tx, seasonID int) error
}

type FormRepository interface {
	FormReader
	FormWriter
}

// ---- Predictions -----------------------------------------------------

type PredictionReader interface {
	GetLatest(ctx context.Context, seasonID int) ([]PredictionWithTeam, error)
	GetByWeek(ctx context.Context, seasonID, week int) ([]PredictionWithTeam, error)
}

type PredictionWriter interface {
	DeleteForWeek(ctx context.Context, tx pgx.Tx, seasonID, week int) error
	DeleteForSeason(ctx context.Context, tx pgx.Tx, seasonID int) error
	BulkInsert(ctx context.Context, tx pgx.Tx, predictions []Prediction) error
}

type PredictionRepository interface {
	PredictionReader
	PredictionWriter
}

// =====================================================================
// Algorithm interfaces — extensibility via Open/Closed Principle.
// =====================================================================

// MatchSimulator turns two teams (and their current form) into a
// concrete MatchResult. The default implementation is the Poisson xG
// simulator; swap in EloSimulator without touching callers.
//
// Form is passed explicitly so the simulator is a pure function and
// has no repository dependencies.
type MatchSimulator interface {
	Simulate(home, away Team, homeForm, awayForm TeamForm) MatchResult
}

// StandingsCalculator rebuilds the entire standings table from the set
// of played fixtures. The same implementation is used by SimulateWeek
// and EditFixture, eliminating duplicated standings logic (DRY).
type StandingsCalculator interface {
	Calculate(seasonID int, teams []Team, playedFixtures []Fixture) []Standing
}

// StandingsSorter applies the head-to-head tiebreaker on top of the
// canonical points/GD/GF SQL order. Kept distinct from
// StandingsCalculator so the read path can depend only on what it
// needs (Interface Segregation).
type StandingsSorter interface {
	SortWithHeadToHead(rows []StandingWithTeam, playedFixtures []Fixture) []StandingWithTeam
}

// ChampionshipPredictor returns one Prediction row per team after
// running its prediction algorithm (default: Monte Carlo, 10k runs).
//
// teams and forms are passed in explicitly so the predictor stays a
// pure function over an in-memory snapshot. This matters for
// SimulateWeek, where the freshly-updated forms have not yet been
// committed: passing them in avoids reading stale DB state.
type ChampionshipPredictor interface {
	Predict(
		ctx context.Context,
		seasonID int,
		week int,
		standings []Standing,
		remaining []Fixture,
		teams []Team,
		forms []TeamForm,
	) []Prediction
}

// =====================================================================
// Services — what handlers depend on. Handlers must never import
// concrete service types from internal/service.
// =====================================================================

// SeasonService owns the lifecycle of a season and the read-only views
// over its data.
type SeasonService interface {
	Create(ctx context.Context) (*SeasonWithFixtures, error)
	Get(ctx context.Context, id int) (*Season, error)
	GetStandings(ctx context.Context, seasonID int) ([]StandingWithTeam, error)
	GetFixtures(ctx context.Context, seasonID int) ([]FixtureWithTeams, error)
	GetWeek(ctx context.Context, seasonID, week int) (*WeekView, error)
	Reset(ctx context.Context, seasonID int) error
}

// SimulationService drives the league forward.
type SimulationService interface {
	SimulateNextWeek(ctx context.Context, seasonID int) (*WeekResult, error)
	PlayAll(ctx context.Context, seasonID int) (*PlayAllResult, error)
}

// FixtureService owns mutations to fixtures (currently: editing the
// scoreline of an already-played match).
type FixtureService interface {
	Edit(ctx context.Context, fixtureID, homeGoals, awayGoals int) (*EditFixtureResult, error)
}

// PredictionService is a thin read-side facade over the prediction
// table. Write-side prediction logic lives in SimulationService /
// FixtureService since those are the only events that produce
// predictions.
type PredictionService interface {
	GetLatest(ctx context.Context, seasonID int) ([]PredictionWithTeam, error)
	GetByWeek(ctx context.Context, seasonID, week int) ([]PredictionWithTeam, error)
}
