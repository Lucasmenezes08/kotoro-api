package fixedcycles

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/Lucasmenezes08/kotoro-api.git/internal/auth"
	"github.com/Lucasmenezes08/kotoro-api.git/internal/utils"
	"github.com/google/uuid"
)

type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

type fixedCycleErrorResponse struct {
	Error string `json:"error"`
}

type createFixedCycleRequest struct {
	SubjectID        string   `json:"subject_id"`
	Title            string   `json:"title"`
	EstimatedSeconds int      `json:"estimated_seconds"`
	DayOfWeek        string   `json:"day_of_week"`
	Tasks            []string `json:"tasks"`
}

type updateFixedCycleRequest struct {
	SubjectID        *string   `json:"subject_id"`
	Title            *string   `json:"title"`
	EstimatedSeconds *int      `json:"estimated_seconds"`
	DayOfWeek        *string   `json:"day_of_week"`
	Tasks            *[]string `json:"tasks"`
}

func (h *Handler) GetAll(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		utils.WriteJson(w, http.StatusUnauthorized, fixedCycleErrorResponse{Error: "Unauthorized"})
		return
	}
	var dayOfWeek *string
	if rawDay := r.URL.Query().Get("day_of_week"); rawDay != "" {
		dayOfWeek = &rawDay
	}
	cycles, err := h.service.GetAll(r.Context(), userID, dayOfWeek)
	if err != nil {
		h.writeError(w, err, "get all fixed cycles")
		return
	}
	utils.WriteJson(w, http.StatusOK, cycles)
}

func (h *Handler) GetByID(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		utils.WriteJson(w, http.StatusUnauthorized, fixedCycleErrorResponse{Error: "Unauthorized"})
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.WriteJson(w, http.StatusBadRequest, fixedCycleErrorResponse{Error: "Invalid fixed cycle id"})
		return
	}
	cycle, err := h.service.GetByID(r.Context(), userID, id)
	if err != nil {
		h.writeError(w, err, "get fixed cycle by id")
		return
	}
	utils.WriteJson(w, http.StatusOK, cycle)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		utils.WriteJson(w, http.StatusUnauthorized, fixedCycleErrorResponse{Error: "Unauthorized"})
		return
	}
	var req createFixedCycleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteJson(w, http.StatusBadRequest, fixedCycleErrorResponse{Error: "Invalid request payload"})
		return
	}
	subjectID, err := uuid.Parse(req.SubjectID)
	if err != nil {
		utils.WriteJson(w, http.StatusBadRequest, fixedCycleErrorResponse{Error: "Invalid subject id"})
		return
	}
	cycle, err := h.service.Create(r.Context(), userID, CreateFixedCycleInput{
		SubjectID:        subjectID,
		Title:            req.Title,
		EstimatedSeconds: req.EstimatedSeconds,
		DayOfWeek:        req.DayOfWeek,
		Tasks:            req.Tasks,
	})
	if err != nil {
		h.writeError(w, err, "create fixed cycle")
		return
	}
	utils.WriteJson(w, http.StatusCreated, cycle)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		utils.WriteJson(w, http.StatusUnauthorized, fixedCycleErrorResponse{Error: "Unauthorized"})
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.WriteJson(w, http.StatusBadRequest, fixedCycleErrorResponse{Error: "Invalid fixed cycle id"})
		return
	}
	var req updateFixedCycleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteJson(w, http.StatusBadRequest, fixedCycleErrorResponse{Error: "Invalid request payload"})
		return
	}
	var subjectUUID *uuid.UUID
	if req.SubjectID != nil {
		parsed, err := uuid.Parse(*req.SubjectID)
		if err != nil {
			utils.WriteJson(w, http.StatusBadRequest, fixedCycleErrorResponse{Error: "Invalid subject id"})
			return
		}
		subjectUUID = &parsed
	}
	cycle, err := h.service.Update(r.Context(), userID, id, UpdateFixedCycleInput{
		SubjectID:        subjectUUID,
		Title:            req.Title,
		EstimatedSeconds: req.EstimatedSeconds,
		DayOfWeek:        req.DayOfWeek,
		Tasks:            req.Tasks,
	})
	if err != nil {
		h.writeError(w, err, "update fixed cycle")
		return
	}
	utils.WriteJson(w, http.StatusOK, cycle)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		utils.WriteJson(w, http.StatusUnauthorized, fixedCycleErrorResponse{Error: "Unauthorized"})
		return
	}
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		utils.WriteJson(w, http.StatusBadRequest, fixedCycleErrorResponse{Error: "Invalid fixed cycle id"})
		return
	}
	if err := h.service.Delete(r.Context(), userID, id); err != nil {
		h.writeError(w, err, "delete fixed cycle")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) writeError(w http.ResponseWriter, err error, operation string) {
	switch {
	case errors.Is(err, ErrFixedCycleNotFound):
		utils.WriteJson(w, http.StatusNotFound, fixedCycleErrorResponse{Error: err.Error()})
	case errors.Is(err, ErrFixedCycleTitleRequired),
		errors.Is(err, ErrFixedCycleTitleTooLong),
		errors.Is(err, ErrFixedCycleInvalidSubject),
		errors.Is(err, ErrFixedCycleInvalidDayOfWeek),
		errors.Is(err, ErrFixedCycleInvalidEstimated):
		utils.WriteJson(w, http.StatusBadRequest, fixedCycleErrorResponse{Error: err.Error()})
	default:
		slog.Error("fixed cycle operation failed", "operation", operation, "error", err)
		utils.WriteJson(w, http.StatusInternalServerError, fixedCycleErrorResponse{Error: "Internal server error"})
	}
}
