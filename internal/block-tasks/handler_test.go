package blocktasks

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Lucasmenezes08/kotoro-api.git/internal/sprint"
	sprintblocks "github.com/Lucasmenezes08/kotoro-api.git/internal/sprint-blocks"
	"github.com/google/uuid"
)

type fakeTaskService struct {
	createFn  func(context.Context, uuid.UUID, uuid.UUID, CreateBlockTaskInput) (*BlockTask, error)
	getAllFn  func(context.Context, uuid.UUID, uuid.UUID) ([]BlockTask, error)
	getByIDFn func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (*BlockTask, error)
	updateFn  func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, BlockTaskUpdateInput) (*BlockTask, error)
	deleteFn  func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error
	reorderFn func(context.Context, uuid.UUID, uuid.UUID, []uuid.UUID) error
}

func (f *fakeTaskService) Create(ctx context.Context, sprintID, blockID uuid.UUID, input CreateBlockTaskInput) (*BlockTask, error) {
	if f.createFn == nil {
		panic("unexpected Create")
	}
	return f.createFn(ctx, sprintID, blockID, input)
}
func (f *fakeTaskService) GetAll(ctx context.Context, sprintID, blockID uuid.UUID) ([]BlockTask, error) {
	if f.getAllFn == nil {
		panic("unexpected GetAll")
	}
	return f.getAllFn(ctx, sprintID, blockID)
}
func (f *fakeTaskService) GetByID(ctx context.Context, sprintID, blockID, taskID uuid.UUID) (*BlockTask, error) {
	if f.getByIDFn == nil {
		panic("unexpected GetByID")
	}
	return f.getByIDFn(ctx, sprintID, blockID, taskID)
}
func (f *fakeTaskService) Update(ctx context.Context, sprintID, blockID, taskID uuid.UUID, input BlockTaskUpdateInput) (*BlockTask, error) {
	if f.updateFn == nil {
		panic("unexpected Update")
	}
	return f.updateFn(ctx, sprintID, blockID, taskID, input)
}
func (f *fakeTaskService) Delete(ctx context.Context, sprintID, blockID, taskID uuid.UUID) error {
	if f.deleteFn == nil {
		panic("unexpected Delete")
	}
	return f.deleteFn(ctx, sprintID, blockID, taskID)
}
func (f *fakeTaskService) Reorder(ctx context.Context, sprintID, blockID uuid.UUID, ids []uuid.UUID) error {
	if f.reorderFn == nil {
		panic("unexpected Reorder")
	}
	return f.reorderFn(ctx, sprintID, blockID, ids)
}

func taskRequest(method, body string, sprintID, blockID, taskID uuid.UUID) *http.Request {
	r := httptest.NewRequest(method, "/", strings.NewReader(body))
	r.SetPathValue("sprintID", sprintID.String())
	r.SetPathValue("blockID", blockID.String())
	if taskID != uuid.Nil {
		r.SetPathValue("taskID", taskID.String())
	}
	return r
}

func TestBlockTaskHandlerCreate(t *testing.T) {
	sprintID, blockID, taskID := uuid.New(), uuid.New(), uuid.New()
	service := &fakeTaskService{createFn: func(_ context.Context, gotSprint, gotBlock uuid.UUID, input CreateBlockTaskInput) (*BlockTask, error) {
		if gotSprint != sprintID || gotBlock != blockID || input.Title != "task" || input.Position != 1 {
			t.Fatal("wrong input")
		}
		return &BlockTask{ID: taskID, SprintBlockID: blockID, Title: input.Title, Position: 1, Status: TaskPending, Priority: PriorityMedium}, nil
	}}
	recorder := httptest.NewRecorder()
	NewBlockTaskHandler(service).Create(recorder, taskRequest(http.MethodPost, `{"title":"task","position":1}`, sprintID, blockID, uuid.Nil))
	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"status":"pendente"`) || !strings.Contains(recorder.Body.String(), `"priority":"media"`) {
		t.Fatalf("unexpected body: %s", recorder.Body.String())
	}
}

func TestBlockTaskHandlerRejectsInvalidBodies(t *testing.T) {
	sprintID, blockID := uuid.New(), uuid.New()
	for _, body := range []string{`{`, `{"title":"task","position":1,"status":"feita"}`, `{"title":"task","position":1,"completed_at":"2026-01-01T00:00:00Z"}`} {
		recorder := httptest.NewRecorder()
		NewBlockTaskHandler(&fakeTaskService{}).Create(recorder, taskRequest(http.MethodPost, body, sprintID, blockID, uuid.Nil))
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for %q, got %d", body, recorder.Code)
		}
	}
}

func TestBlockTaskHandlerRejectsInvalidPathIDs(t *testing.T) {
	tests := []struct {
		key                       string
		sprintID, blockID, taskID string
	}{
		{"sprint", "invalid", uuid.NewString(), uuid.NewString()},
		{"block", uuid.NewString(), "invalid", uuid.NewString()},
		{"task", uuid.NewString(), uuid.NewString(), "invalid"},
	}
	for _, test := range tests {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.SetPathValue("sprintID", test.sprintID)
		r.SetPathValue("blockID", test.blockID)
		r.SetPathValue("taskID", test.taskID)
		recorder := httptest.NewRecorder()
		NewBlockTaskHandler(&fakeTaskService{}).GetByID(recorder, r)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for %s id, got %d", test.key, recorder.Code)
		}
	}
}

func TestBlockTaskHandlerErrorMappings(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"validation", ErrBlockTaskTitleRequired, http.StatusBadRequest},
		{"sprint missing", sprint.ErrSprintNotFound, http.StatusNotFound},
		{"block missing", sprintblocks.ErrSprintBlockNotFound, http.StatusNotFound},
		{"task missing", ErrBlockTaskNotFound, http.StatusNotFound},
		{"sprint conflict", sprintblocks.ErrSprintNotInPlanning, http.StatusConflict},
		{"block conflict", ErrSprintBlockNotInPlanning, http.StatusConflict},
		{"position conflict", ErrBlockTaskPositionConflict, http.StatusConflict},
		{"unexpected", errors.New("sql details"), http.StatusInternalServerError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &fakeTaskService{deleteFn: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error { return test.err }}
			recorder := httptest.NewRecorder()
			NewBlockTaskHandler(service).Delete(recorder, taskRequest(http.MethodDelete, "", uuid.New(), uuid.New(), uuid.New()))
			if recorder.Code != test.want {
				t.Fatalf("expected %d, got %d", test.want, recorder.Code)
			}
			if test.want == 500 && strings.Contains(recorder.Body.String(), "sql details") {
				t.Fatal("internal error leaked")
			}
		})
	}
}

func TestBlockTaskHandlerSuccessResponses(t *testing.T) {
	sprintID, blockID, taskID := uuid.New(), uuid.New(), uuid.New()
	t.Run("empty list is json array", func(t *testing.T) {
		service := &fakeTaskService{getAllFn: func(context.Context, uuid.UUID, uuid.UUID) ([]BlockTask, error) { return []BlockTask{}, nil }}
		recorder := httptest.NewRecorder()
		NewBlockTaskHandler(service).GetAll(recorder, taskRequest(http.MethodGet, "", sprintID, blockID, uuid.Nil))
		if recorder.Code != 200 || strings.TrimSpace(recorder.Body.String()) != "[]" {
			t.Fatalf("unexpected response: %d %q", recorder.Code, recorder.Body.String())
		}
	})
	t.Run("patch returns json", func(t *testing.T) {
		title := "updated"
		service := &fakeTaskService{updateFn: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, BlockTaskUpdateInput) (*BlockTask, error) {
			return &BlockTask{ID: taskID, SprintBlockID: blockID, Title: title, Status: TaskPending, Priority: PriorityMedium}, nil
		}}
		recorder := httptest.NewRecorder()
		NewBlockTaskHandler(service).Update(recorder, taskRequest(http.MethodPatch, `{"title":"updated"}`, sprintID, blockID, taskID))
		if recorder.Code != 200 || !strings.Contains(recorder.Body.String(), `"title":"updated"`) {
			t.Fatalf("unexpected response: %d %q", recorder.Code, recorder.Body.String())
		}
	})
	t.Run("delete has no body", func(t *testing.T) {
		service := &fakeTaskService{deleteFn: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error { return nil }}
		recorder := httptest.NewRecorder()
		NewBlockTaskHandler(service).Delete(recorder, taskRequest(http.MethodDelete, "", sprintID, blockID, taskID))
		if recorder.Code != 204 || recorder.Body.Len() != 0 {
			t.Fatalf("unexpected response: %d %q", recorder.Code, recorder.Body.String())
		}
	})
	t.Run("reorder has no body", func(t *testing.T) {
		service := &fakeTaskService{reorderFn: func(_ context.Context, _, _ uuid.UUID, ids []uuid.UUID) error {
			if len(ids) != 1 || ids[0] != taskID {
				t.Fatal("wrong ids")
			}
			return nil
		}}
		recorder := httptest.NewRecorder()
		NewBlockTaskHandler(service).Reorder(recorder, taskRequest(http.MethodPut, `{"task_ids":["`+taskID.String()+`"]}`, sprintID, blockID, uuid.Nil))
		if recorder.Code != 204 || recorder.Body.Len() != 0 {
			t.Fatalf("unexpected response: %d %q", recorder.Code, recorder.Body.String())
		}
	})
}

var _ BlockTaskServiceContract = (*fakeTaskService)(nil)
