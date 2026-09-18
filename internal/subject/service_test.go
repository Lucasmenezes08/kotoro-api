package subject

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

type fakeSubjectRepository struct {
	createFn func(ctx context.Context, subject SubjectCreateInput) error
	getAllFn func(ctx context.Context)([]Subject, error)
	updateFn func(ctx context.Context, id uuid.UUID, input SubjectUpdateInput) error
	deleteByIdFn func(ctx context.Context, id uuid.UUID)error 
	createBatchFn func(ctx context.Context, subjects []SubjectCreateInput)([]SubjectBatchResult,error)
}

func (r *fakeSubjectRepository) Create(ctx context.Context, subject SubjectCreateInput) error {
	if r.createFn == nil {
		panic("unexpected call to create")
	}

	return r.createFn(ctx, subject)
}

func (r *fakeSubjectRepository) GetAll(ctx context.Context) ([]Subject, error) {
	if r.getAllFn == nil {
		panic("unexpected call to get")
	}
	return r.getAllFn(ctx)
}

func (r *fakeSubjectRepository) Update(ctx context.Context, id uuid.UUID, input SubjectUpdateInput)error {
	if r.getAllFn == nil {
		panic("unexpected call to update")
	}
	return r.updateFn(ctx, id, input)
}

func (r *fakeSubjectRepository) DeleteById(ctx context.Context, id uuid.UUID)error {
	if r.deleteByIdFn == nil {
		panic("unexpected call to deleteById")
	}
	return r.deleteByIdFn(ctx, id)
}

func (r *fakeSubjectRepository) CreateBatch(ctx context.Context, subjects []SubjectCreateInput)([]SubjectBatchResult,error) {
	if r.createBatchFn == nil {
		panic("unexpected call to deleteById")
	}
	return r.createBatchFn(ctx, subjects)
}


var _ SubjectContract = (*fakeSubjectRepository)(nil)

func TestServiceCreate(t *testing.T) {
	t.Run("returns error when color is invalid", func(t *testing.T) {
		ctx := context.Background()
		newFakeSubjectRepository := &fakeSubjectRepository{}
		service := NewSubjectService(newFakeSubjectRepository)

		newSubject := SubjectCreateInput{
			Name:  "Math",
			Color: "simsalabim",
		}

		err := service.Create(ctx, newSubject)
		if !errors.Is(err, ErrSubjectInvalidColor) {
			t.Fatalf("invalid subject color")
		}
	})

	t.Run("returns error when color is empty", func(t *testing.T) {
		ctx := context.Background()
		newFakeSubjectRepository := &fakeSubjectRepository{}
		service := NewSubjectService(newFakeSubjectRepository)

		newSubject := SubjectCreateInput{
			Name:  "math",
			Color: " ",
		}

		err := service.Create(ctx, newSubject)
		if !errors.Is(err, ErrSubjectRequiredColor) {
			t.Fatalf(
				"expected ErrSubjectColorRequired, got %v",
				err,
			)
		}
	})

	t.Run("returns error when name is empty", func(t *testing.T) {
		ctx := context.Background()
		newFakeSubjectRepository := &fakeSubjectRepository{}
		service := NewSubjectService(newFakeSubjectRepository)

		newSubject := SubjectCreateInput{
			Name:  " ",
			Color: "black",
		}

		err := service.Create(ctx, newSubject)
		if !errors.Is(err, ErrSubjectRequiredName) {
			t.Fatalf(
				"expected ErrSubjectRequiredName, got %v",
				err,
			)
		}
	})
}

func TestServiceGet(t *testing.T) {
	ctx := context.Background()

	expected := []Subject{
		{
			Name:  "Math",
			Color: ColorBlack,
		},
	}

	repository := &fakeSubjectRepository{
		getAllFn: func(ctx context.Context) ([]Subject, error) {
			return expected,nil
		},
	}
	service := NewSubjectService(repository)

	sub, err := service.GetAll(ctx)

	if err != nil {
		t.Fatalf("Must return getAll error")
	}

	if len(sub) != 1 {
		t.Fatalf("expected %d, got %d subjects", len(expected), len(sub))
	}

	if sub[0].Name != expected[0].Name {
		t.Errorf(
			"expected name %q, got %q",
			expected[0].Name,
			sub[0].Name,
		)
	}

	if sub[0].Color != expected[0].Color {
		t.Errorf(
			"expected color %q, got %q",
			expected[0].Color,
			sub[0].Color,
		)
	}
}
