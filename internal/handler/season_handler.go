package handler

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/insider/league-api/internal/domain"
	"github.com/insider/league-api/internal/repository"
	"github.com/insider/league-api/internal/service"
	"github.com/insider/league-api/pkg/cache"
	"github.com/insider/league-api/pkg/response"
	"github.com/insider/league-api/pkg/validator"
)

// SeasonHandler owns every endpoint under /api/seasons. It delegates
// to SeasonService and SimulationService; it never reaches into the
// repository or domain layers directly.
type SeasonHandler struct {
	seasons    domain.SeasonService
	simulation domain.SimulationService
	cache      *cache.Cache
}

// NewSeasonHandler wires the two services. Handlers are stateless so
// a value type would do, but pointer makes mocking easier.
func NewSeasonHandler(
	seasons domain.SeasonService,
	simulation domain.SimulationService,
	c *cache.Cache,
) *SeasonHandler {
	return &SeasonHandler{seasons: seasons, simulation: simulation, cache: c}
}

// Create handles POST /api/seasons.
func (h *SeasonHandler) Create(w http.ResponseWriter, r *http.Request) {
	out, err := h.seasons.Create(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	response.Created(w, out)
}

// Get handles GET /api/seasons/{id}.
func (h *SeasonHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := validator.ParsePositiveInt(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	s, err := h.seasons.Get(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	response.OK(w, s)
}

// GetStandings handles GET /api/seasons/{id}/standings.
func (h *SeasonHandler) GetStandings(w http.ResponseWriter, r *http.Request) {
	id, err := validator.ParsePositiveInt(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	key := cache.StandingsKey(id)
	if v, ok := h.cache.Get(key); ok {
		response.OK(w, v.([]domain.StandingWithTeam))
		return
	}
	rows, err := h.seasons.GetStandings(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	h.cache.Set(key, rows)
	response.OK(w, rows)
}

// GetFixtures handles GET /api/seasons/{id}/fixtures.
func (h *SeasonHandler) GetFixtures(w http.ResponseWriter, r *http.Request) {
	id, err := validator.ParsePositiveInt(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	fixtures, err := h.seasons.GetFixtures(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	response.OK(w, groupFixturesByWeek(fixtures))
}

// GetWeek handles GET /api/seasons/{id}/week/{week}.
func (h *SeasonHandler) GetWeek(w http.ResponseWriter, r *http.Request) {
	id, err := validator.ParsePositiveInt(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	week, err := validator.ParseWeek(chi.URLParam(r, "week"))
	if err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	out, err := h.seasons.GetWeek(r.Context(), id, week)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	response.OK(w, out)
}

// SimulateNext handles POST /api/seasons/{id}/next-week.
func (h *SeasonHandler) SimulateNext(w http.ResponseWriter, r *http.Request) {
	id, err := validator.ParsePositiveInt(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	out, err := h.simulation.SimulateNextWeek(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	h.cache.Evict(cache.StandingsKey(id))
	response.OK(w, out)
}

// PlayAll handles POST /api/seasons/{id}/play-all.
func (h *SeasonHandler) PlayAll(w http.ResponseWriter, r *http.Request) {
	id, err := validator.ParsePositiveInt(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	out, err := h.simulation.PlayAll(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	h.cache.Evict(cache.StandingsKey(id))
	response.OK(w, out)
}

// Reset handles DELETE /api/seasons/{id}/reset.
func (h *SeasonHandler) Reset(w http.ResponseWriter, r *http.Request) {
	id, err := validator.ParsePositiveInt(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	if err := h.seasons.Reset(r.Context(), id); err != nil {
		writeServiceError(w, err)
		return
	}
	h.cache.Evict(cache.StandingsKey(id))
	response.OK(w, map[string]any{"season_id": id, "reset": true})
}

// =====================================================================
// Shared handler helpers
// =====================================================================

// writeServiceError maps service / repository errors to HTTP status
// codes via the response package. The mapping is the single source of
// truth — handlers never decide status codes inline.
func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		response.NotFound(w, response.CodeNotFound, err.Error())
	case errors.Is(err, service.ErrSeasonComplete):
		response.Conflict(w, response.CodeSeasonComplete, err.Error())
	case errors.Is(err, service.ErrWeekAlreadyPlayed):
		response.Conflict(w, response.CodeWeekAlreadyPlayed, err.Error())
	case errors.Is(err, service.ErrSeasonHasNoTeams):
		response.Unprocessable(w, response.CodeUnprocessable, err.Error())
	default:
		response.Internal(w, err.Error())
	}
}

// groupFixturesByWeek turns a flat list into the
// { week_n: [fixtures] } shape that the /fixtures endpoint promises.
func groupFixturesByWeek(fixtures []domain.FixtureWithTeams) map[int][]domain.FixtureWithTeams {
	out := make(map[int][]domain.FixtureWithTeams)
	for _, f := range fixtures {
		out[f.Week] = append(out[f.Week], f)
	}
	return out
}
