package sprint

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/Lucasmenezes08/kotoro-api.git/internal/utils"
)


type SprintHandler struct {
	service *SprintService
}

type createSprintRequest struct{
	Name       *string      `json:"name"`
	SprintDate time.Time    `json:"sprint_date"`
	Status     SprintStatus `json:"status"`
}


type errorResponse struct {
	Error string `json:"error"`
}


func NewSprintHandler(service *SprintService) *SprintHandler{
	return &SprintHandler{
		service: service,
	}
}

func (h *SprintHandler)Create(w http.ResponseWriter, r *http.Request){
	var req createSprintRequest

	decode := json.NewDecoder(r.Body)
	decode.DisallowUnknownFields()

	if err := decode.Decode(&req); err != nil {
		utils.WriteJson(w, http.StatusBadRequest, errorResponse{Error: "Invalid request body"})
		return 
	}

	payload := CreateSprintModel{
		Name: req.Name,
		SprintDate: req.SprintDate,
		Status: req.Status,
	}


	err := h.service.Create(r.Context(), payload)

	switch{
	case errors.Is(err, ErrSprintAlreadyExists):
		utils.WriteJson(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
	
	case errors.Is(err, ErrSprintNameEmpty):
		utils.WriteJson(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
	
	case errors.Is(err, ErrSprintNotFound):
		utils.WriteJson(w, http.StatusNotFound, errorResponse{Error: err.Error()})
	
	case err != nil:
		slog.Error("failed to create sprint", "error", err)
		utils.WriteJson(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
	
	default:
		w.WriteHeader(http.StatusCreated)
	}
}