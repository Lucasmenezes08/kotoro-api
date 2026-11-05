package sprintblocks

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Lucasmenezes08/kotoro-api.git/internal/auth"
	"github.com/Lucasmenezes08/kotoro-api.git/internal/sprint"
	"github.com/Lucasmenezes08/kotoro-api.git/internal/subject"
	"github.com/google/uuid"
)

type fakeBlockService struct {
	createFn  func(context.Context, uuid.UUID, uuid.UUID, CreateSprintBlockInput) (*SprintBlock, error)
	getAllFn  func(context.Context, uuid.UUID, uuid.UUID) ([]SprintBlock, error)
	getByIDFn func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (*SprintBlock, error)
	updateFn  func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, SprintBlockUpdateModel) (*SprintBlock, error)
	deleteFn             func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error
	reorderFn            func(context.Context, uuid.UUID, uuid.UUID, []uuid.UUID) error
	updateCanvasLayoutFn func(context.Context, uuid.UUID, uuid.UUID, UpdateCanvasLayoutInput) error
}

func (f *fakeBlockService) Create(ctx context.Context, userID, id uuid.UUID, input CreateSprintBlockInput) (*SprintBlock, error) {
	if f.createFn == nil {
		panic("unexpected Create")
	}
	return f.createFn(ctx, userID, id, input)
}
func (f *fakeBlockService) GetAll(ctx context.Context, userID, id uuid.UUID) ([]SprintBlock, error) {
	if f.getAllFn == nil {
		panic("unexpected GetAll")
	}
	return f.getAllFn(ctx, userID, id)
}
func (f *fakeBlockService) GetByID(ctx context.Context, userID, sprintID, blockID uuid.UUID) (*SprintBlock, error) {
	if f.getByIDFn == nil {
		panic("unexpected GetByID")
	}
	return f.getByIDFn(ctx, userID, sprintID, blockID)
}
func (f *fakeBlockService) Update(ctx context.Context, userID, sprintID, blockID uuid.UUID, input SprintBlockUpdateModel) (*SprintBlock, error) {
	if f.updateFn == nil {
		panic("unexpected Update")
	}
	return f.updateFn(ctx, userID, sprintID, blockID, input)
}
func (f *fakeBlockService) Delete(ctx context.Context, userID, sprintID, blockID uuid.UUID) error {
	if f.deleteFn == nil {
		panic("unexpected Delete")
	}
	return f.deleteFn(ctx, userID, sprintID, blockID)
}
func (f *fakeBlockService) Reorder(ctx context.Context, userID, sprintID uuid.UUID, ids []uuid.UUID) error {
	if f.reorderFn == nil {
		panic("unexpected Reorder")
	}
	return f.reorderFn(ctx, userID, sprintID, ids)
}
func (f *fakeBlockService) UpdateCanvasLayout(ctx context.Context, userID, sprintID uuid.UUID, input UpdateCanvasLayoutInput) error {
	if f.updateCanvasLayoutFn != nil {
		return f.updateCanvasLayoutFn(ctx, userID, sprintID, input)
	}
	return nil
}

func blockRequest(method, target, body string, sprintID, blockID uuid.UUID) *http.Request {
	r := httptest.NewRequest(method, target, strings.NewReader(body))
	r = r.WithContext(auth.WithUserID(r.Context(), defaultTestUserID))
	r.SetPathValue("sprintID", sprintID.String())
	if blockID != uuid.Nil {
		r.SetPathValue("blockID", blockID.String())
	}
	return r
}

func TestSprintBlockHandlerCreate(t *testing.T) {
	sprintID, subjectID, blockID := uuid.New(), uuid.New(), uuid.New()
	service := &fakeBlockService{createFn: func(_ context.Context, gotUser, gotSprint uuid.UUID, input CreateSprintBlockInput) (*SprintBlock, error) {
		if gotUser != defaultTestUserID || gotSprint != sprintID || input.SubjectID != subjectID || input.Position != 1 {
			t.Fatal("incorrect create input")
		}
		return &SprintBlock{ID: blockID, SprintID: sprintID, SubjectID: subjectID, Position: 1, Status: Pending}, nil
	}}
	recorder := httptest.NewRecorder()
	NewSprintBlockHandler(service).Create(recorder, blockRequest(http.MethodPost, "/", `{"subject_id":"`+subjectID.String()+`","position":1}`, sprintID, uuid.Nil))
	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"status":"pendente"`) {
		t.Fatalf("unexpected body: %s", recorder.Body.String())
	}
}

func TestSprintBlockHandlerRejectsInvalidJSONAndUnknownFields(t *testing.T) {
	for _, body := range []string{`{`, `{"subject_id":"x","position":1,"status":"finalizado"}`} {
		recorder := httptest.NewRecorder()
		NewSprintBlockHandler(&fakeBlockService{}).Create(recorder, blockRequest(http.MethodPost, "/", body, uuid.New(), uuid.Nil))
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for %q, got %d", body, recorder.Code)
		}
	}
}

func TestSprintBlockHandlerRejectsInvalidPathIDs(t *testing.T) {
	handler := NewSprintBlockHandler(&fakeBlockService{})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request = request.WithContext(auth.WithUserID(request.Context(), defaultTestUserID))
	request.SetPathValue("sprintID", "invalid")
	handler.GetAll(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid sprint 400, got %d", recorder.Code)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/", nil)
	request = request.WithContext(auth.WithUserID(request.Context(), defaultTestUserID))
	request.SetPathValue("sprintID", uuid.NewString())
	request.SetPathValue("blockID", "invalid")
	handler.GetByID(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid block 400, got %d", recorder.Code)
	}
}

func TestSprintBlockHandlerErrorMappings(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
	}{
		{"validation", ErrSprintBlockPositionInvalid, http.StatusBadRequest},
		{"sprint missing", sprint.ErrSprintNotFound, http.StatusNotFound},
		{"subject missing", subject.ErrSubjectNotFound, http.StatusNotFound},
		{"block missing", ErrSprintBlockNotFound, http.StatusNotFound},
		{"state conflict", ErrSprintNotInPlanning, http.StatusConflict},
		{"position conflict", ErrSprintBlockPositionConflict, http.StatusConflict},
		{"unexpected", errors.New("database unavailable"), http.StatusInternalServerError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &fakeBlockService{deleteFn: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error { return test.err }}
			recorder := httptest.NewRecorder()
			NewSprintBlockHandler(service).Delete(recorder, blockRequest(http.MethodDelete, "/", "", uuid.New(), uuid.New()))
			if recorder.Code != test.status {
				t.Fatalf("expected %d, got %d", test.status, recorder.Code)
			}
			if test.status == http.StatusInternalServerError && strings.Contains(recorder.Body.String(), "database unavailable") {
				t.Fatal("internal error leaked")
			}
		})
	}
}

func TestSprintBlockHandlerResponsesAndNoContent(t *testing.T) {
	sprintID, blockID := uuid.New(), uuid.New()
	t.Run("list returns json array", func(t *testing.T) {
		service := &fakeBlockService{getAllFn: func(context.Context, uuid.UUID, uuid.UUID) ([]SprintBlock, error) { return []SprintBlock{}, nil }}
		recorder := httptest.NewRecorder()
		NewSprintBlockHandler(service).GetAll(recorder, blockRequest(http.MethodGet, "/", "", sprintID, uuid.Nil))
		if recorder.Code != http.StatusOK || strings.TrimSpace(recorder.Body.String()) != "[]" {
			t.Fatalf("unexpected response: %d %q", recorder.Code, recorder.Body.String())
		}
	})
	t.Run("delete has no body", func(t *testing.T) {
		service := &fakeBlockService{deleteFn: func(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error { return nil }}
		recorder := httptest.NewRecorder()
		NewSprintBlockHandler(service).Delete(recorder, blockRequest(http.MethodDelete, "/", "", sprintID, blockID))
		if recorder.Code != http.StatusNoContent || recorder.Body.Len() != 0 {
			t.Fatalf("unexpected response: %d %q", recorder.Code, recorder.Body.String())
		}
	})
	t.Run("reorder has no body", func(t *testing.T) {
		service := &fakeBlockService{reorderFn: func(_ context.Context, gotUser, gotSprint uuid.UUID, ids []uuid.UUID) error {
			if gotUser != defaultTestUserID || gotSprint != sprintID || len(ids) != 1 || ids[0] != blockID {
				t.Fatal("wrong ids")
			}
			return nil
		}}
		body := bytes.NewBufferString(`{"block_ids":["` + blockID.String() + `"]}`)
		request := httptest.NewRequest(http.MethodPut, "/", body)
		request = request.WithContext(auth.WithUserID(request.Context(), defaultTestUserID))
		request.SetPathValue("sprintID", sprintID.String())
		recorder := httptest.NewRecorder()
		NewSprintBlockHandler(service).Reorder(recorder, request)
		if recorder.Code != http.StatusNoContent || recorder.Body.Len() != 0 {
			t.Fatalf("unexpected response: %d %q", recorder.Code, recorder.Body.String())
		}
	})
	t.Run("patch returns updated json", func(t *testing.T) {
		title := "updated"
		service := &fakeBlockService{updateFn: func(_ context.Context, gotUser, gotSprint, gotBlock uuid.UUID, input SprintBlockUpdateModel) (*SprintBlock, error) {
			if gotUser != defaultTestUserID || gotSprint != sprintID || gotBlock != blockID || input.Title == nil || *input.Title != title {
				t.Fatal("wrong update input")
			}
			return &SprintBlock{ID: blockID, SprintID: sprintID, Title: input.Title, Status: Pending}, nil
		}}
		recorder := httptest.NewRecorder()
		NewSprintBlockHandler(service).Update(recorder, blockRequest(http.MethodPatch, "/", `{"title":"updated"}`, sprintID, blockID))
		if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"title":"updated"`) {
			t.Fatalf("unexpected response: %d %q", recorder.Code, recorder.Body.String())
		}
	})
	t.Run("patch allows concluding block with status and timestamps", func(t *testing.T) {
		service := &fakeBlockService{updateFn: func(_ context.Context, gotUser, gotSprint, gotBlock uuid.UUID, input SprintBlockUpdateModel) (*SprintBlock, error) {
			if input.Status == nil || *input.Status != Finished || input.StartedAt == nil || input.CompletedAt == nil {
				t.Fatalf("wrong status/timestamps input: %#v", input)
			}
			return &SprintBlock{ID: blockID, SprintID: sprintID, Status: Finished, StartedAt: input.StartedAt, CompletedAt: input.CompletedAt}, nil
		}}
		recorder := httptest.NewRecorder()
		payload := `{"status":"finalizado","started_at":"2026-11-05T08:00:00Z","completed_at":"2026-11-05T08:50:00Z"}`
		NewSprintBlockHandler(service).Update(recorder, blockRequest(http.MethodPatch, "/", payload, sprintID, blockID))
		if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"status":"finalizado"`) {
			t.Fatalf("unexpected response: %d %q", recorder.Code, recorder.Body.String())
		}
	})
	t.Run("update canvas layout has no body and returns 204", func(t *testing.T) {
		called := false
		service := &fakeBlockService{updateCanvasLayoutFn: func(_ context.Context, gotUser, gotSprint uuid.UUID, input UpdateCanvasLayoutInput) error {
			called = len(input.Blocks) == 1 && input.Blocks[0].X == 120.5
			return nil
		}}
		recorder := httptest.NewRecorder()
		request := blockRequest(http.MethodPut, "/", `{"blocks":[{"id":"`+blockID.String()+`","position":1,"x":120.5,"y":240.0}]}`, sprintID, uuid.Nil)
		NewSprintBlockHandler(service).UpdateCanvasLayout(recorder, request)
		if recorder.Code != http.StatusNoContent || recorder.Body.Len() != 0 || !called {
			t.Fatalf("unexpected response: %d %q, called: %v", recorder.Code, recorder.Body.String(), called)
		}
	})
	t.Run("returns 401 when unauthorized", func(t *testing.T) {
		service := &fakeBlockService{}
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.SetPathValue("sprintID", sprintID.String())
		NewSprintBlockHandler(service).GetAll(recorder, req)
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", recorder.Code)
		}
	})
}

var _ SprintBlockServiceContract = (*fakeBlockService)(nil)
