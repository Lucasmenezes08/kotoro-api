package subject

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/Lucasmenezes08/kotoro-api.git/internal/utils"
	"github.com/google/uuid"
)

type SubjectServiceContract interface {
	Create(ctx context.Context, subject SubjectCreateInput) error
	GetAll(ctx context.Context)([]Subject, error)
	Update(ctx context.Context, id uuid.UUID, input SubjectUpdateInput)error
}

type SubjectController struct {
	service SubjectServiceContract
}

func NewSubjectController(service SubjectServiceContract) *SubjectController {
	return &SubjectController{
		service: service,
	}
}

type createSubjectRequest struct {
	Name  string `json:"name"`
	Color Color  `json:"color"`
}

type updateSubjectRequest struct {
	Name  *string `json:"name"`
	Color *Color  `json:"color"`
}

type errorResponse struct {
	Error string `json:"error"`
}


func (c *SubjectController)GetAll(w http.ResponseWriter, r *http.Request){
	sub , err := c.service.GetAll(r.Context())
	if err != nil{

		slog.Error("Error to get all subjects", "Error", err)

		utils.WriteJson(w, http.StatusInternalServerError, errorResponse{Error: "Internal server error" })
		return 
	}

	utils.WriteJson(w, http.StatusOK, sub)
}


func (c *SubjectController) Create(w http.ResponseWriter, r *http.Request) {
	var req createSubjectRequest

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&req); err != nil {
		utils.WriteJson(w, http.StatusBadRequest, errorResponse{Error: "Invalid request body"})
		return
	}

	input := SubjectCreateInput{
		Name:  req.Name,
		Color: req.Color,
	}

	err := c.service.Create(r.Context(), input)

	switch {
	case errors.Is(err, ErrSubjectRequiredName):
		utils.WriteJson(w, http.StatusBadRequest, errorResponse{Error: "Subject name is required"})

	case errors.Is(err, ErrSubjectRequiredColor):
		utils.WriteJson(w, http.StatusBadRequest, errorResponse{Error: "Subject color is required"})

	case errors.Is(err, ErrSubjectInvalidColor):
		utils.WriteJson(w, http.StatusBadRequest, errorResponse{Error: "invalid Subject color"})

	case err != nil:
		slog.Error("failed to create subject", "error", err)

		utils.WriteJson(w, http.StatusInternalServerError, errorResponse{
			Error: "internal server error",
		})
	default:
		w.WriteHeader(http.StatusCreated)
	}
}


func (c *SubjectController) Update(w http.ResponseWriter, r *http.Request){
	var req updateSubjectRequest

	decode := json.NewDecoder(r.Body)
	decode.DisallowUnknownFields()

	if err := decode.Decode(&req); err != nil{
		utils.WriteJson(w, http.StatusBadRequest, errorResponse{Error: "Invalid request body"})
	}

	input := SubjectUpdateInput{
		Name: req.Name,
		Color: req.Color,
	}

	pathId := r.PathValue("id")

	id, err := uuid.Parse(pathId)
	if err != nil {
		utils.WriteJson(w, http.StatusBadRequest, errorResponse{Error: "Invalid subject id"} )
	}

	update := c.service.Update(r.Context(), id, input)

	switch {
	case errors.Is(update, ErrSubjectUpdateEmpty):
		utils.WriteJson(w, http.StatusNoContent, update)
	case errors.Is(update, ErrSubjectRequiredName):
		utils.WriteJson(w, http.StatusBadRequest, update)
	case errors.Is(update, ErrSubjectRequiredName):
		utils.WriteJson(w, http.StatusBadRequest, update)
	case errors.Is(update, ErrSubjectRequiredColor):
		utils.WriteJson(w, http.StatusBadRequest, update)
	case errors.Is(update, ErrSubjectInvalidColor):
		utils.WriteJson(w, http.StatusBadRequest, update)
	case err != nil:
		slog.Error("failed to update subject", "error", err)

		utils.WriteJson(w, http.StatusInternalServerError, errorResponse{
			Error: "internal server error",
		})
	default:
		w.WriteHeader(http.StatusOK)
	}
}