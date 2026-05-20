// Command league-api is the HTTP entry point for the football league
// simulation. main.go is the single composition root: every concrete
// type is instantiated here and wired together via interfaces. No
// other file in the project constructs concrete dependencies.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Anill17/league-api/config"
	"github.com/Anill17/league-api/internal/handler"
	"github.com/Anill17/league-api/internal/repository"
	"github.com/Anill17/league-api/internal/service"
	"github.com/Anill17/league-api/pkg/cache"
)

const (
	readHeaderTimeout = 10 * time.Second
	shutdownTimeout   = 10 * time.Second
)

func main() {
	if err := run(); err != nil {
		log.Printf("fatal: %v", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT, syscall.SIGTERM,
	)
	defer stop()

	pool, err := newPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	router := buildRouter(pool)

	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("league-api listening on :%s (env=%s)", cfg.Port, cfg.Env)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
		close(serverErr)
	}()

	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():
		log.Println("shutting down gracefully...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return err
		}
		log.Println("server stopped")
		return nil
	}
}

func newPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

// buildRouter is the dependency-injection ceremony. Every concrete
// type is constructed here and only here.
func buildRouter(pool *pgxpool.Pool) http.Handler {
	// Transaction manager.
	tx := repository.NewTxManager(pool)

	// Repositories.
	teamRepo := repository.NewTeamRepo(pool)
	seasonRepo := repository.NewSeasonRepo(pool)
	fixtureRepo := repository.NewFixtureRepo(pool)
	standingsRepo := repository.NewStandingsRepo(pool)
	formRepo := repository.NewFormRepo(pool)
	predictionRepo := repository.NewPredictionRepo(pool)

	// Algorithm implementations.
	standingsSvc := service.NewStandingsService()
	simulator := service.NewPoissonSimulator()
	predictor := service.NewMonteCarloPrediction(simulator)
	formSvc := service.NewFormService(formRepo)

	// Services.
	seasonSvc := service.NewSeasonService(
		seasonRepo, fixtureRepo, standingsRepo,
		formRepo, predictionRepo, teamRepo,
		standingsSvc, tx,
	)
	simulationSvc := service.NewSimulationService(
		fixtureRepo, standingsRepo, formRepo, predictionRepo,
		seasonRepo, teamRepo,
		simulator, predictor, standingsSvc,
		formSvc, tx,
	)
	fixtureSvc := service.NewFixtureService(
		fixtureRepo, standingsRepo, predictionRepo,
		seasonRepo, teamRepo, formRepo,
		standingsSvc, predictor, tx,
	)
	predictionSvc := service.NewPredictionService(predictionRepo)

	// Shared in-memory cache (standings read-through, evicted on every write).
	c := cache.New()

	// Handlers.
	handlers := handler.Handlers{
		Season:     handler.NewSeasonHandler(seasonSvc, simulationSvc, c),
		Fixture:    handler.NewFixtureHandler(fixtureSvc, c),
		Prediction: handler.NewPredictionHandler(predictionSvc),
	}

	return handler.NewRouter(handlers)
}
