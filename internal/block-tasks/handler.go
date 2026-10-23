package blocktasks

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/Lucasmenezes08/kotoro-api.git/internal/sprint"
	sprintblocks "github.com/Lucasmenezes08/kotoro-api.git/internal/sprint-blocks"
	"github.com/Lucasmenezes08/kotoro-api.git/internal/utils"
	"github.com/google/uuid"
)

type BlockTaskHandler struct{ service BlockTaskServiceContract }

type createBlockTaskRequest struct {
	Title            string        `json:"title"`
	Description      *string       `json:"description"`
	Position         int           `json:"position"`
	Priority         *TaskPriority `json:"priority"`
	EstimatedSeconds *int          `json:"estimated_seconds"`
}

type updateBlockTaskRequest struct {
	Title            *string       `json:"title"`
	Description      *string       `json:"description"`
	Position         *int          `json:"position"`
	Priority         *TaskPriority `json:"priority"`
	EstimatedSeconds *int          `json:"estimated_seconds"`
}

type reorderBlockTasksRequest struct {
	TaskIDs []string `json:"task_ids"`
}

type blockTaskErrorResponse struct {
	Error string `json:"error"`
}

func NewBlockTaskHandler(service BlockTaskServiceContract) *BlockTaskHandler {
	return &BlockTaskHandler{service: service}
}

func (h *BlockTaskHandler) Create(w http.ResponseWriter, r *http.Request) {
	sprintID, blockID, ok := parseTaskParentPath(w, r)
	if !ok {
		return
	}
	var req createBlockTaskRequest
	if !decodeTaskJSON(w, r, &req) {
		return
	}
	task, err := h.service.Create(r.Context(), sprintID, blockID, CreateBlockTaskInput{
		Title: req.Title, Description: req.Description, Position: req.Position,
		Priority: req.Priority, EstimatedSeconds: req.EstimatedSeconds,
	})
	if err != nil {
		h.writeError(w, err, "create block task")
		return
	}
	utils.WriteJson(w, http.StatusCreated, responseFromModel(task))
}

func (h *BlockTaskHandler) GetAll(w http.ResponseWriter, r *http.Request) {
	sprintID, blockID, ok := parseTaskParentPath(w, r)
	if !ok {
		return
	}
	tasks, err := h.service.GetAll(r.Context(), sprintID, blockID)
	if err != nil {
		h.writeError(w, err, "get block tasks")
		return
	}
	response := make([]BlockTaskResponse, 0, len(tasks))
	for index := range tasks {
		response = append(response, responseFromModel(&tasks[index]))
	}
	utils.WriteJson(w, http.StatusOK, response)
}

func (h *BlockTaskHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	sprintID, blockID, taskID, ok := parseTaskPath(w, r)
	if !ok {
		return
	}
	task, err := h.service.GetByID(r.Context(), sprintID, blockID, taskID)
	if err != nil {
		h.writeError(w, err, "get block task")
		return
	}
	utils.WriteJson(w, http.StatusOK, responseFromModel(task))
}

func (h *BlockTaskHandler) Update(w http.ResponseWriter, r *http.Request) {
	sprintID, blockID, taskID, ok := parseTaskPath(w, r)
	if !ok {
		return
	}
	var req updateBlockTaskRequest
	if !decodeTaskJSON(w, r, &req) {
		return
	}
	task, err := h.service.Update(r.Context(), sprintID, blockID, taskID, BlockTaskUpdateInput{
		Title: req.Title, Description: req.Description, Position: req.Position,
		Priority: req.Priority, EstimatedSeconds: req.EstimatedSeconds,
	})
	if err != nil {
		h.writeError(w, err, "update block task")
		return
	}
	utils.WriteJson(w, http.StatusOK, responseFromModel(task))
}

func (h *BlockTaskHandler) Delete(w http.ResponseWriter, r *http.Request) {
	sprintID, blockID, taskID, ok := parseTaskPath(w, r)
	if !ok {
		return
	}
	if err := h.service.Delete(r.Context(), sprintID, blockID, taskID); err != nil {
		h.writeError(w, err, "delete block task")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *BlockTaskHandler) Reorder(w http.ResponseWriter, r *http.Request) {
	sprintID, blockID, ok := parseTaskParentPath(w, r)
	if !ok {
		return
	}
	var req reorderBlockTasksRequest
	if !decodeTaskJSON(w, r, &req) {
		return
	}
	ids := make([]uuid.UUID, 0, len(req.TaskIDs))
	for _, rawID := range req.TaskIDs {
		id, err := uuid.Parse(rawID)
		if err != nil {
			utils.WriteJson(w, http.StatusBadRequest, blockTaskErrorResponse{Error: "Invalid task id"})
			return
		}
		ids = append(ids, id)
	}
	if err := h.service.Reorder(r.Context(), sprintID, blockID, ids); err != nil {
		h.writeError(w, err, "reorder block tasks")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *BlockTaskHandler) writeError(w http.ResponseWriter, err error, operation string) {
	switch {
	case errors.Is(err, ErrBlockTaskTitleRequired), errors.Is(err, ErrBlockTaskTitleTooLong),
		errors.Is(err, ErrBlockTaskUpdateEmpty), errors.Is(err, ErrBlockTaskPositionInvalid),
		errors.Is(err, ErrBlockTaskPriorityInvalid), errors.Is(err, ErrBlockTaskEstimatedSecondsInvalid),
		errors.Is(err, ErrBlockTaskOrderInvalid):
		utils.WriteJson(w, http.StatusBadRequest, blockTaskErrorResponse{Error: err.Error()})
	case errors.Is(err, sprint.ErrSprintNotFound), errors.Is(err, sprintblocks.ErrSprintBlockNotFound),
		errors.Is(err, ErrBlockTaskNotFound):
		utils.WriteJson(w, http.StatusNotFound, blockTaskErrorResponse{Error: err.Error()})
	case errors.Is(err, sprintblocks.ErrSprintNotInPlanning), errors.Is(err, ErrSprintBlockNotInPlanning),
		errors.Is(err, ErrBlockTaskPositionConflict):
		utils.WriteJson(w, http.StatusConflict, blockTaskErrorResponse{Error: err.Error()})
	default:
		slog.Error("block task request failed", "operation", operation, "error", err)
		utils.WriteJson(w, http.StatusInternalServerError, blockTaskErrorResponse{Error: "Internal server error"})
	}
}

func decodeTaskJSON(w http.ResponseWriter, r *http.Request, destination any) bool {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		utils.WriteJson(w, http.StatusBadRequest, blockTaskErrorResponse{Error: "Invalid request body"})
		return false
	}
	return true
}

func parseTaskPath(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, uuid.UUID, bool) {
	sprintID, blockID, ok := parseTaskParentPath(w, r)
	if !ok {
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	taskID, ok := parseTaskPathUUID(w, r, "taskID", "Invalid task id")
	return sprintID, blockID, taskID, ok
}

func parseTaskParentPath(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	sprintID, ok := parseTaskPathUUID(w, r, "sprintID", "Invalid sprint id")
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	blockID, ok := parseTaskPathUUID(w, r, "blockID", "Invalid block id")
	return sprintID, blockID, ok
}

func parseTaskPathUUID(w http.ResponseWriter, r *http.Request, key, message string) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue(key))
	if err != nil {
		utils.WriteJson(w, http.StatusBadRequest, blockTaskErrorResponse{Error: message})
		return uuid.Nil, false
	}
	return id, true
}
