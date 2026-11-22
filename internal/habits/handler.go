package habits

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/Lucasmenezes08/kotoro-api.git/internal/auth"
	"github.com/Lucasmenezes08/kotoro-api.git/internal/utils"
)

type HabitsHandler struct {
	service HabitsServiceContract
}

func NewHabitsHandler(service HabitsServiceContract) *HabitsHandler {
	return &HabitsHandler{service: service}
}

func (h *HabitsHandler) GetWater(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		utils.WriteJson(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return
	}

	dateStr := r.URL.Query().Get("date")
	res, err := h.service.GetWaterDay(r.Context(), userID, dateStr)
	if err != nil {
		slog.Error("failed to get water day record", "error", err)
		utils.WriteJson(w, http.StatusInternalServerError, map[string]string{"error": "Internal server error"})
		return
	}

	utils.WriteJson(w, http.StatusOK, res)
}

func (h *HabitsHandler) AddWater(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		utils.WriteJson(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return
	}

	var req AddWaterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteJson(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	res, err := h.service.AddWaterLog(r.Context(), userID, req)
	if err != nil {
		slog.Error("failed to add water log", "error", err)
		utils.WriteJson(w, http.StatusInternalServerError, map[string]string{"error": "Internal server error"})
		return
	}

	utils.WriteJson(w, http.StatusOK, res)
}

func (h *HabitsHandler) UpdateWaterTarget(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		utils.WriteJson(w, http.StatusUnauthorized, map[string]string{"error": "Unauthorized"})
		return
	}

	var req UpdateWaterTargetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteJson(w, http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
		return
	}

	res, err := h.service.UpdateWaterTarget(r.Context(), userID, req)
	if err != nil {
		slog.Error("failed to update water target", "error", err)
		utils.WriteJson(w, http.StatusInternalServerError, map[string]string{"error": "Internal server error"})
		return
	}

	utils.WriteJson(w, http.StatusOK, res)
}
