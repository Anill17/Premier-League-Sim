package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/insider/league-api/pkg/response"
)

// Handlers groups every concrete handler so router wiring is a single
// well-typed construction. The struct is intentionally a value (not
// pointers) — handlers are stateless and shared across goroutines.
type Handlers struct {
	Season     *SeasonHandler
	Fixture    *FixtureHandler
	Prediction *PredictionHandler
}

// NewRouter assembles the HTTP router. The handler struct is provided
// already-constructed so this function knows nothing about the
// service layer.
func NewRouter(h Handlers) http.Handler {
	r := chi.NewRouter()

	// Order matters: Recoverer first so panics in any later
	// middleware are still caught, RequestID before Logger so the
	// log line includes the id.
	r.Use(Recoverer)
	r.Use(RequestID)
	r.Use(Logger)

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		response.OK(w, map[string]string{"status": "ok"})
	})

	r.Get("/openapi.yaml", ServeOpenAPISpec)
	r.Get("/docs", ServeSwaggerUI)

	r.Route("/api", func(r chi.Router) {
		r.Route("/seasons", func(r chi.Router) {
			r.Post("/", h.Season.Create)
			r.Get("/{id}", h.Season.Get)
			r.Get("/{id}/standings", h.Season.GetStandings)
			r.Get("/{id}/fixtures", h.Season.GetFixtures)
			r.Get("/{id}/week/{week}", h.Season.GetWeek)
			r.Post("/{id}/next-week", h.Season.SimulateNext)
			r.Post("/{id}/play-all", h.Season.PlayAll)
			r.Get("/{id}/predictions", h.Prediction.GetLatest)
			r.Get("/{id}/predictions/{week}", h.Prediction.GetByWeek)
			r.Delete("/{id}/reset", h.Season.Reset)
		})

		r.Put("/fixtures/{id}", h.Fixture.Edit)
	})

	return r
}
