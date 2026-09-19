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
	GetAll(ctx context.Context) ([]Subject, error)
	Update(ctx context.Context, id uuid.UUID, input SubjectUpdateInput) error
	DeleteById(ctx context.Context, id uuid.UUID) error
	CreateBatch(ctx context.Context, subjects []SubjectCreateInput) ([]SubjectBatchResult, error)
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

type createSubjectBatchRequest struct {
	Subjects []createSubjectRequest `json:"subjects"`
}

type createSubjectBatchItemResponse struct {
	Index   int    `json:"index"`
	Name    string `json:"name"`
	Created bool   `json:"created"`
	Error   string `json:"error,omitempty"`
}

type updateSubjectRequest struct {
	Name  *string `json:"name"`
	Color *Color  `json:"color"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func (c *SubjectController) GetAll(w http.ResponseWriter, r *http.Request) {
	sub, err := c.service.GetAll(r.Context())
	if err != nil {

		slog.Error("Error to get all subjects", "Error", err)

		utils.WriteJson(w, http.StatusInternalServerError, errorResponse{Error: "Internal server error"})
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

	case errors.Is(err, ErrSubjectDuplicated):
		utils.WriteJson(w, http.StatusConflict, errorResponse{Error: err.Error()})
	case err != nil:
		slog.Error("failed to create subject", "error", err)

		utils.WriteJson(w, http.StatusInternalServerError, errorResponse{
			Error: "internal server error",
		})
	default:
		w.WriteHeader(http.StatusCreated)
	}
}

func (c *SubjectController) Update(w http.ResponseWriter, r *http.Request) {
	var req updateSubjectRequest

	decode := json.NewDecoder(r.Body)
	decode.DisallowUnknownFields()

	if err := decode.Decode(&req); err != nil {
		utils.WriteJson(w, http.StatusBadRequest, errorResponse{Error: "Invalid request body"})
		return
	}

	input := SubjectUpdateInput{
		Name:  req.Name,
		Color: req.Color,
	}

	pathId := r.PathValue("id")

	id, err := uuid.Parse(pathId)
	if err != nil {
		utils.WriteJson(w, http.StatusBadRequest, errorResponse{Error: "Invalid subject id"})
		return
	}

	update := c.service.Update(r.Context(), id, input)

	switch {
	case errors.Is(update, ErrSubjectUpdateEmpty):
		utils.WriteJson(w, http.StatusBadRequest, errorResponse{Error: update.Error()})
	case errors.Is(update, ErrSubjectRequiredName):
		utils.WriteJson(w, http.StatusBadRequest, errorResponse{Error: update.Error()})
	case errors.Is(update, ErrSubjectRequiredColor):
		utils.WriteJson(w, http.StatusBadRequest, errorResponse{Error: update.Error()})
	case errors.Is(update, ErrSubjectInvalidColor):
		utils.WriteJson(w, http.StatusBadRequest, errorResponse{Error: update.Error()})
	case errors.Is(update, ErrSubjectNotFound):
		utils.WriteJson(w, http.StatusNotFound, errorResponse{Error: update.Error()})
	case update != nil:
		slog.Error("failed to update subject", "error", update)

		utils.WriteJson(w, http.StatusInternalServerError, errorResponse{
			Error: "internal server error",
		})
	default:
		w.WriteHeader(http.StatusOK)
	}
}

func (c *SubjectController) DeleteById(w http.ResponseWriter, r *http.Request) {

	pathId := r.PathValue("id")

	param, errParse := uuid.Parse(pathId)
	if errParse != nil {
		utils.WriteJson(w, http.StatusBadRequest, errorResponse{Error: "Invalid subject id"})
		return
	}

	err := c.service.DeleteById(r.Context(), param)

	if err != nil {
		utils.WriteJson(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (c *SubjectController) CreateBatch(
	w http.ResponseWriter,
	r *http.Request,
) {
	var req createSubjectBatchRequest

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&req); err != nil {
		utils.WriteJson(
			w,
			http.StatusBadRequest,
			errorResponse{
				Error: "invalid request body",
			},
		)
		return
	}

	if len(req.Subjects) == 0 {
		utils.WriteJson(
			w,
			http.StatusBadRequest,
			errorResponse{
				Error: "at least one subject is required",
			},
		)
		return
	}

	if len(req.Subjects) > 100 {
		utils.WriteJson(
			w,
			http.StatusBadRequest,
			errorResponse{
				Error: "the batch cannot exceed 100 subjects",
			},
		)
		return
	}

	inputs := make(
		[]SubjectCreateInput,
		0,
		len(req.Subjects),
	)

	for _, subject := range req.Subjects {
		inputs = append(
			inputs,
			SubjectCreateInput{
				Name:  subject.Name,
				Color: subject.Color,
			},
		)
	}

	results, err := c.service.CreateBatch(
		r.Context(),
		inputs,
	)

	if err != nil {
		switch {
		case errors.Is(err, context.Canceled):
			slog.Info("subject import canceled")
			return

		case errors.Is(
			err,
			context.DeadlineExceeded,
		):
			utils.WriteJson(
				w,
				http.StatusGatewayTimeout,
				errorResponse{
					Error: "subject import timeout",
				},
			)
			return

		default:
			slog.Error(
				"failed to import subjects",
				"error",
				err,
			)

			utils.WriteJson(
				w,
				http.StatusInternalServerError,
				errorResponse{
					Error: "internal server error",
				},
			)
			return
		}
	}

	created := 0
	failed := 0

	items := make(
		[]map[string]any,
		0,
		len(results),
	)

	for _, result := range results {
		item := map[string]any{
			"index":   result.Index,
			"name":    result.Name,
			"created": result.Created,
		}

		switch {
		case result.Err == nil:
			created++

		case errors.Is(
			result.Err,
			ErrSubjectRequiredName,
		):
			failed++
			item["error"] = "subject name is required"

		case errors.Is(
			result.Err,
			ErrSubjectRequiredColor,
		):
			failed++
			item["error"] = "subject color is required"

		case errors.Is(
			result.Err,
			ErrSubjectInvalidColor,
		):
			failed++
			item["error"] = "invalid subject color"

		default:
			failed++
			item["error"] = "internal server error"

			slog.Error(
				"failed to create subject during import",
				"index", result.Index,
				"name", result.Name,
				"error", result.Err,
			)
		}

		items = append(items, item)
	}

	utils.WriteJson(
		w,
		http.StatusOK,
		map[string]any{
			"total":   len(req.Subjects),
			"created": created,
			"failed":  failed,
			"results": items,
		},
	)
}
