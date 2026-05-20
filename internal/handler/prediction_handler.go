package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/Anill17/league-api/internal/domain"
	"github.com/Anill17/league-api/pkg/response"
	"github.com/Anill17/league-api/pkg/validator"
)

// PredictionHandler owns the read-only prediction endpoints.
type PredictionHandler struct {
	predictions domain.PredictionService
}

// NewPredictionHandler wires the service dependency.
func NewPredictionHandler(predictions domain.PredictionService) *PredictionHandler {
	return &PredictionHandler{predictions: predictions}
}

// GetLatest handles GET /api/seasons/{id}/predictions.
func (h *PredictionHandler) GetLatest(w http.ResponseWriter, r *http.Request) {
	id, err := validator.ParsePositiveInt(chi.URLParam(r, "id"))
	if err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	rows, err := h.predictions.GetLatest(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	response.OK(w, rows)
}

// GetByWeek handles GET /api/seasons/{id}/predictions/{week}.
func (h *PredictionHandler) GetByWeek(w http.ResponseWriter, r *http.Request) {
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
	rows, err := h.predictions.GetByWeek(r.Context(), id, week)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	response.OK(w, rows)
}
