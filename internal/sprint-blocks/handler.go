package sprintblocks

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/Lucasmenezes08/kotoro-api.git/internal/sprint"
	"github.com/Lucasmenezes08/kotoro-api.git/internal/subject"
	"github.com/Lucasmenezes08/kotoro-api.git/internal/utils"
	"github.com/google/uuid"
)

type SprintBlockHandler struct{ service SprintBlockServiceContract }

type createSprintBlockRequest struct {
	SubjectID        string  `json:"subject_id"`
	Title            *string `json:"title"`
	Position         int     `json:"position"`
	EstimatedSeconds *int    `json:"estimated_seconds"`
}

type updateSprintBlockRequest struct {
	SubjectID        *string `json:"subject_id"`
	Title            *string `json:"title"`
	Position         *int    `json:"position"`
	EstimatedSeconds *int    `json:"estimated_seconds"`
}

type reorderSprintBlocksRequest struct {
	BlockIDs []string `json:"block_ids"`
}

type sprintBlockErrorResponse struct {
	Error string `json:"error"`
}

func NewSprintBlockHandler(service SprintBlockServiceContract) *SprintBlockHandler {
	return &SprintBlockHandler{service: service}
}

func (h *SprintBlockHandler) Create(w http.ResponseWriter, r *http.Request) {
	sprintID, ok := parsePathUUID(w, r, "sprintID", "Invalid sprint id")
	if !ok {
		return
	}
	var req createSprintBlockRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	subjectID, err := uuid.Parse(req.SubjectID)
	if err != nil {
		utils.WriteJson(w, http.StatusBadRequest, sprintBlockErrorResponse{Error: "Invalid subject id"})
		return
	}
	block, err := h.service.Create(r.Context(), sprintID, CreateSprintBlockInput{
		SubjectID: subjectID, Title: req.Title, Position: req.Position, EstimatedSeconds: req.EstimatedSeconds,
	})
	if err != nil {
		h.writeError(w, err, "create sprint block")
		return
	}
	utils.WriteJson(w, http.StatusCreated, responseFromModel(block))
}

func (h *SprintBlockHandler) GetAll(w http.ResponseWriter, r *http.Request) {
	sprintID, ok := parsePathUUID(w, r, "sprintID", "Invalid sprint id")
	if !ok {
		return
	}
	blocks, err := h.service.GetAll(r.Context(), sprintID)
	if err != nil {
		h.writeError(w, err, "get sprint blocks")
		return
	}
	response := make([]SprintBlockResponse, 0, len(blocks))
	for index := range blocks {
		response = append(response, responseFromModel(&blocks[index]))
	}
	utils.WriteJson(w, http.StatusOK, response)
}

func (h *SprintBlockHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	sprintID, blockID, ok := parseBlockPath(w, r)
	if !ok {
		return
	}
	block, err := h.service.GetByID(r.Context(), sprintID, blockID)
	if err != nil {
		h.writeError(w, err, "get sprint block")
		return
	}
	utils.WriteJson(w, http.StatusOK, responseFromModel(block))
}

func (h *SprintBlockHandler) Update(w http.ResponseWriter, r *http.Request) {
	sprintID, blockID, ok := parseBlockPath(w, r)
	if !ok {
		return
	}
	var req updateSprintBlockRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	var subjectID *uuid.UUID
	if req.SubjectID != nil {
		parsed, err := uuid.Parse(*req.SubjectID)
		if err != nil {
			utils.WriteJson(w, http.StatusBadRequest, sprintBlockErrorResponse{Error: "Invalid subject id"})
			return
		}
		subjectID = &parsed
	}
	block, err := h.service.Update(r.Context(), sprintID, blockID, SprintBlockUpdateModel{
		SubjectID: subjectID, Title: req.Title, Position: req.Position, EstimatedSeconds: req.EstimatedSeconds,
	})
	if err != nil {
		h.writeError(w, err, "update sprint block")
		return
	}
	utils.WriteJson(w, http.StatusOK, responseFromModel(block))
}

func (h *SprintBlockHandler) Delete(w http.ResponseWriter, r *http.Request) {
	sprintID, blockID, ok := parseBlockPath(w, r)
	if !ok {
		return
	}
	if err := h.service.Delete(r.Context(), sprintID, blockID); err != nil {
		h.writeError(w, err, "delete sprint block")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *SprintBlockHandler) Reorder(w http.ResponseWriter, r *http.Request) {
	sprintID, ok := parsePathUUID(w, r, "sprintID", "Invalid sprint id")
	if !ok {
		return
	}
	var req reorderSprintBlocksRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	ids := make([]uuid.UUID, 0, len(req.BlockIDs))
	for _, rawID := range req.BlockIDs {
		id, err := uuid.Parse(rawID)
		if err != nil {
			utils.WriteJson(w, http.StatusBadRequest, sprintBlockErrorResponse{Error: "Invalid block id"})
			return
		}
		ids = append(ids, id)
	}
	if err := h.service.Reorder(r.Context(), sprintID, ids); err != nil {
		h.writeError(w, err, "reorder sprint blocks")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *SprintBlockHandler) writeError(w http.ResponseWriter, err error, operation string) {
	switch {
	case errors.Is(err, ErrSprintBlockUpdateEmpty),
		errors.Is(err, ErrSprintBlockPositionInvalid),
		errors.Is(err, ErrSprintBlockEstimatedSecondsInvalid),
		errors.Is(err, ErrSprintBlockTitleInvalid),
		errors.Is(err, ErrSprintBlockTitleTooLong),
		errors.Is(err, ErrInvalidSprintBlockOrder):
		utils.WriteJson(w, http.StatusBadRequest, sprintBlockErrorResponse{Error: err.Error()})
	case errors.Is(err, sprint.ErrSprintNotFound),
		errors.Is(err, subject.ErrSubjectNotFound),
		errors.Is(err, ErrSprintBlockNotFound):
		utils.WriteJson(w, http.StatusNotFound, sprintBlockErrorResponse{Error: err.Error()})
	case errors.Is(err, ErrSprintNotInPlanning),
		errors.Is(err, ErrSprintBlockPositionConflict),
		errors.Is(err, ErrSprintBlockSubjectConflict):
		utils.WriteJson(w, http.StatusConflict, sprintBlockErrorResponse{Error: err.Error()})
	default:
		slog.Error("sprint block request failed", "operation", operation, "error", err)
		utils.WriteJson(w, http.StatusInternalServerError, sprintBlockErrorResponse{Error: "Internal server error"})
	}
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) bool {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		utils.WriteJson(w, http.StatusBadRequest, sprintBlockErrorResponse{Error: "Invalid request body"})
		return false
	}
	return true
}

func parseBlockPath(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	sprintID, ok := parsePathUUID(w, r, "sprintID", "Invalid sprint id")
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	blockID, ok := parsePathUUID(w, r, "blockID", "Invalid block id")
	return sprintID, blockID, ok
}

func parsePathUUID(w http.ResponseWriter, r *http.Request, key, message string) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue(key))
	if err != nil {
		utils.WriteJson(w, http.StatusBadRequest, sprintBlockErrorResponse{Error: message})
		return uuid.Nil, false
	}
	return id, true
}
