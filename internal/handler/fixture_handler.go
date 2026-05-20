package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/insider/league-api/internal/domain"
	"github.com/insider/league-api/pkg/cache"
	"github.com/insider/league-api/pkg/response"
	"github.com/insider/league-api/pkg/validator"
)

// FixtureHandler owns PUT /api/fixtures/{id}. The handler is
// deliberately tiny: parse, validate, delegate.
type FixtureHandler struct {
	fixtures domain.FixtureService
	cache    *cache.Cache
}

// NewFixtureHandler wires the service dependency.
func NewFixtureHandler(fixtures domain.FixtureService, c *cache.Cache) *FixtureHandler {
	return &FixtureHandler{fixtures: fixtures, cache: c}
}

// Edit handles PUT /api/fixtures/{id}.
//
// Body: { "home_goals": int, "away_goals": int }
// Response: updated fixture, full standings, fresh predictions for the
// current week (if week ≥ 4).
func (h *FixtureHandler) Edit(w http.ResponseWriter, r *http.Request) {
	id, err := validator.ParsePositiveInt(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, err.Error())
		return
	}

	var body validator.EditFixturePayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, response.CodeInvalidJSON,
			"request body is not valid JSON")
		return
	}
	if err := validator.ValidateEditFixture(body); err != nil {
		response.BadRequest(w, err.Error())
		return
	}

	out, err := h.fixtures.Edit(r.Context(), id, body.HomeGoals, body.AwayGoals)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	h.cache.Evict(cache.StandingsKey(out.Fixture.SeasonID))
	response.OK(w, out)
}
