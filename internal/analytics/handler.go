package analytics

import (
	"log/slog"
	"net/http"

	"github.com/Lucasmenezes08/kotoro-api.git/internal/auth"
	"github.com/Lucasmenezes08/kotoro-api.git/internal/utils"
)

type AnalyticsHandler struct {
	service AnalyticsServiceContract
}

func NewAnalyticsHandler(service AnalyticsServiceContract) *AnalyticsHandler {
	return &AnalyticsHandler{service: service}
}

func (h *AnalyticsHandler) GetSubjects(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		utils.WriteJson(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return
	}

	timeframe := r.URL.Query().Get("timeframe")
	refDateStr := r.URL.Query().Get("reference_date")

	subjects, err := h.service.GetSubjectStudyTimes(r.Context(), userID, timeframe, refDateStr)
	if err != nil {
		slog.Error("failed to get subject study times", "error", err)
		utils.WriteJson(w, http.StatusInternalServerError, map[string]string{"error": "Internal server error"})
		return
	}

	utils.WriteJson(w, http.StatusOK, subjects)
}

func (h *AnalyticsHandler) GetStreak(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		utils.WriteJson(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return
	}

	refDateStr := r.URL.Query().Get("reference_date")

	streak, err := h.service.GetStreak(r.Context(), userID, refDateStr)
	if err != nil {
		slog.Error("failed to get streak", "error", err)
		utils.WriteJson(w, http.StatusInternalServerError, map[string]string{"error": "Internal server error"})
		return
	}

	utils.WriteJson(w, http.StatusOK, streak)
}
