package sprint

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

type fakeSprintService struct {
	deleteByIdFn func(ctx context.Context, id uuid.UUID) error
}

func (f *fakeSprintService) Create(
	ctx context.Context,
	payload CreateSprintModel,
) error {
	panic("unexpected call to Create")
}

func (f *fakeSprintService) GetAll(ctx context.Context) ([]Sprint, error) {
	panic("unexpected call to GetAll")
}

func (f *fakeSprintService) GetById(
	ctx context.Context,
	id uuid.UUID,
) (*Sprint, error) {
	panic("unexpected call to GetById")
}

func (f *fakeSprintService) Update(
	ctx context.Context,
	id uuid.UUID,
	payload UpdateSprintModel,
) error {
	panic("unexpected call to Update")
}

func (f *fakeSprintService) DeleteById(
	ctx context.Context,
	id uuid.UUID,
) error {
	if f.deleteByIdFn == nil {
		panic("unexpected call to DeleteById")
	}

	return f.deleteByIdFn(ctx, id)
}

var _ SprintServiceContract = (*fakeSprintService)(nil)

func TestSprintHandlerDeleteByIdReturnsConflictForInvalidStatus(t *testing.T) {
	id := uuid.New()

	service := &fakeSprintService{
		deleteByIdFn: func(
			ctx context.Context,
			receivedID uuid.UUID,
		) error {
			if receivedID != id {
				t.Errorf("expected id %q, got %q", id, receivedID)
			}

			return ErrSprintDeleteStatusNotCreating
		},
	}

	handler := NewSprintHandler(service)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodDelete,
		"/sprints/"+id.String(),
		nil,
	)
	request.SetPathValue("id", id.String())

	handler.DeleteById(recorder, request)

	if recorder.Code != http.StatusConflict {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusConflict,
			recorder.Code,
		)
	}

	var response errorResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.Error != ErrSprintDeleteStatusNotCreating.Error() {
		t.Errorf(
			"expected error %q, got %q",
			ErrSprintDeleteStatusNotCreating,
			response.Error,
		)
	}
}
