package studycycles

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/Lucasmenezes08/kotoro-api.git/internal/auth"
	blocktasks "github.com/Lucasmenezes08/kotoro-api.git/internal/block-tasks"
	"github.com/Lucasmenezes08/kotoro-api.git/internal/sprint"
	sprintblocks "github.com/Lucasmenezes08/kotoro-api.git/internal/sprint-blocks"
	"github.com/Lucasmenezes08/kotoro-api.git/internal/utils"
	"github.com/google/uuid"
)

type StudyCycleHandler struct {
	service StudyCycleServiceContract
	now     func() time.Time
}

type playStudyCycleRequest struct {
	ExecutionMode          ExecutionMode `json:"execution_mode"`
	TimerType              TimerType     `json:"timer_type"`
	PlannedDurationSeconds *int          `json:"planned_duration_seconds"`
	BlockTaskIDs           []string      `json:"block_task_ids"`
}

type stopStudyCycleRequest struct {
	Reason CycleStopReason `json:"reason"`
}

type studyCycleErrorResponse struct {
	Error string `json:"error"`
}

func NewStudyCycleHandler(service StudyCycleServiceContract) *StudyCycleHandler {
	return NewStudyCycleHandlerWithClock(service, func() time.Time {
		return time.Now().UTC()
	})
}

func NewStudyCycleHandlerWithClock(service StudyCycleServiceContract, clock func() time.Time) *StudyCycleHandler {
	if clock == nil {
		clock = func() time.Time {
			return time.Now().UTC()
		}
	}
	return &StudyCycleHandler{
		service: service,
		now:     clock,
	}
}

func (h *StudyCycleHandler) Play(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		utils.WriteJson(w, http.StatusUnauthorized, studyCycleErrorResponse{Error: "Unauthorized"})
		return
	}

	sprintID, ok := parseCyclePathUUID(w, r, "sprintID", "Invalid sprint id")
	if !ok {
		return
	}

	var req playStudyCycleRequest
	if !decodeCycleJSON(w, r, &req) {
		return
	}

	taskIDs := make([]uuid.UUID, 0, len(req.BlockTaskIDs))
	for _, rawID := range req.BlockTaskIDs {
		id, err := uuid.Parse(rawID)
		if err != nil || id == uuid.Nil {
			utils.WriteJson(w, http.StatusBadRequest, studyCycleErrorResponse{Error: "Invalid block task id"})
			return
		}
		taskIDs = append(taskIDs, id)
	}

	cycle, err := h.service.Play(r.Context(), userID, sprintID, PlayStudyCycleInput{
		ExecutionMode:          req.ExecutionMode,
		TimerType:              req.TimerType,
		PlannedDurationSeconds: req.PlannedDurationSeconds,
		BlockTaskIDs:           taskIDs,
	})
	if err != nil {
		h.writeError(w, err, "play study cycle")
		return
	}

	utils.WriteJson(w, http.StatusCreated, responseFromModel(&cycle, h.now()))
}

func (h *StudyCycleHandler) Pause(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		utils.WriteJson(w, http.StatusUnauthorized, studyCycleErrorResponse{Error: "Unauthorized"})
		return
	}

	cycleID, ok := parseCyclePathUUID(w, r, "cycleID", "Invalid study cycle id")
	if !ok {
		return
	}
	if !decodeOptionalEmptyJSON(w, r) {
		return
	}

	cycle, err := h.service.Pause(r.Context(), userID, cycleID)
	if err != nil {
		h.writeError(w, err, "pause study cycle")
		return
	}

	utils.WriteJson(w, http.StatusOK, responseFromModel(&cycle, h.now()))
}

func (h *StudyCycleHandler) Resume(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		utils.WriteJson(w, http.StatusUnauthorized, studyCycleErrorResponse{Error: "Unauthorized"})
		return
	}

	cycleID, ok := parseCyclePathUUID(w, r, "cycleID", "Invalid study cycle id")
	if !ok {
		return
	}
	if !decodeOptionalEmptyJSON(w, r) {
		return
	}

	cycle, err := h.service.Resume(r.Context(), userID, cycleID)
	if err != nil {
		h.writeError(w, err, "resume study cycle")
		return
	}

	utils.WriteJson(w, http.StatusOK, responseFromModel(&cycle, h.now()))
}

func (h *StudyCycleHandler) Stop(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		utils.WriteJson(w, http.StatusUnauthorized, studyCycleErrorResponse{Error: "Unauthorized"})
		return
	}

	cycleID, ok := parseCyclePathUUID(w, r, "cycleID", "Invalid study cycle id")
	if !ok {
		return
	}

	var req stopStudyCycleRequest
	if !decodeCycleJSON(w, r, &req) {
		return
	}

	cycle, err := h.service.Stop(r.Context(), userID, cycleID, StopStudyCycleInput{
		Reason: req.Reason,
	})
	if err != nil {
		h.writeError(w, err, "stop study cycle")
		return
	}

	utils.WriteJson(w, http.StatusOK, responseFromModel(&cycle, h.now()))
}

func (h *StudyCycleHandler) GetActive(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		utils.WriteJson(w, http.StatusUnauthorized, studyCycleErrorResponse{Error: "Unauthorized"})
		return
	}

	sprintID, ok := parseCyclePathUUID(w, r, "sprintID", "Invalid sprint id")
	if !ok {
		return
	}

	cycle, err := h.service.GetActive(r.Context(), userID, sprintID)
	if err != nil {
		h.writeError(w, err, "get active study cycle")
		return
	}

	if cycle == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	utils.WriteJson(w, http.StatusOK, responseFromModel(cycle, h.now()))
}

func (h *StudyCycleHandler) GetAll(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.GetUserIDFromContext(r.Context())
	if err != nil {
		utils.WriteJson(w, http.StatusUnauthorized, studyCycleErrorResponse{Error: "Unauthorized"})
		return
	}

	sprintID, ok := parseCyclePathUUID(w, r, "sprintID", "Invalid sprint id")
	if !ok {
		return
	}

	cycles, err := h.service.GetAll(r.Context(), userID, sprintID)
	if err != nil {
		h.writeError(w, err, "get sprint study cycles")
		return
	}

	responses := make([]StudyCycleResponse, len(cycles))
	serverNow := h.now()
	for i := range cycles {
		responses[i] = responseFromModel(&cycles[i], serverNow)
	}

	utils.WriteJson(w, http.StatusOK, responses)
}

func (h *StudyCycleHandler) writeError(w http.ResponseWriter, err error, operation string) {
	switch {
	case errors.Is(err, ErrInvalidExecutionMode),
		errors.Is(err, ErrInvalidTimerConfiguration),
		errors.Is(err, ErrInvalidSelectedTasks),
		errors.Is(err, ErrInvalidStopReason):
		utils.WriteJson(w, http.StatusBadRequest, studyCycleErrorResponse{Error: err.Error()})

	case errors.Is(err, sprint.ErrSprintNotFound),
		errors.Is(err, sprintblocks.ErrSprintBlockNotFound),
		errors.Is(err, blocktasks.ErrBlockTaskNotFound),
		errors.Is(err, ErrStudyCycleNotFound):
		utils.WriteJson(w, http.StatusNotFound, studyCycleErrorResponse{Error: err.Error()})

	case errors.Is(err, ErrStudyCycleAlreadyActive),
		errors.Is(err, ErrStudyCycleNotRunning),
		errors.Is(err, ErrStudyCycleNotPaused),
		errors.Is(err, ErrStudyCycleAlreadyStopped),
		errors.Is(err, ErrStudyCycleInvalidTransition),
		errors.Is(err, ErrSprintNotStarted),
		errors.Is(err, ErrSprintHasNoAvailableBlock):
		utils.WriteJson(w, http.StatusConflict, studyCycleErrorResponse{Error: err.Error()})

	default:
		slog.Error("study cycle request failed", "operation", operation, "error", err)
		utils.WriteJson(w, http.StatusInternalServerError, studyCycleErrorResponse{Error: "Internal server error"})
	}
}

func decodeCycleJSON(w http.ResponseWriter, r *http.Request, destination any) bool {
	if r.Body == nil {
		utils.WriteJson(w, http.StatusBadRequest, studyCycleErrorResponse{Error: "Invalid request body"})
		return false
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		utils.WriteJson(w, http.StatusBadRequest, studyCycleErrorResponse{Error: "Invalid request body"})
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		utils.WriteJson(w, http.StatusBadRequest, studyCycleErrorResponse{Error: "Invalid request body"})
		return false
	}
	return true
}

func decodeOptionalEmptyJSON(w http.ResponseWriter, r *http.Request) bool {
	if r.Body == nil {
		return true
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var empty struct{}
	if err := decoder.Decode(&empty); err != nil {
		if errors.Is(err, io.EOF) {
			return true
		}
		utils.WriteJson(w, http.StatusBadRequest, studyCycleErrorResponse{Error: "Invalid request body"})
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		utils.WriteJson(w, http.StatusBadRequest, studyCycleErrorResponse{Error: "Invalid request body"})
		return false
	}
	return true
}

func parseCyclePathUUID(w http.ResponseWriter, r *http.Request, key, message string) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue(key))
	if err != nil || id == uuid.Nil {
		utils.WriteJson(w, http.StatusBadRequest, studyCycleErrorResponse{Error: message})
		return uuid.Nil, false
	}
	return id, true
}
