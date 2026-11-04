package subject

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

type fakeSubjectRepository struct {
	createFn      func(ctx context.Context, userID uuid.UUID, subject SubjectCreateInput) error
	getAllFn      func(ctx context.Context, userID uuid.UUID) ([]Subject, error)
	getByIdFn     func(ctx context.Context, id uuid.UUID) (*Subject, error)
	getByNameFn   func(ctx context.Context, userID uuid.UUID, name string) (*Subject, error)
	updateFn      func(ctx context.Context, userID uuid.UUID, id uuid.UUID, input SubjectUpdateInput) error
	deleteByIdFn  func(ctx context.Context, userID uuid.UUID, id uuid.UUID) error
}

func (r *fakeSubjectRepository) Create(ctx context.Context, userID uuid.UUID, subject SubjectCreateInput) error {
	if r.createFn == nil {
		panic("unexpected call to create")
	}
	return r.createFn(ctx, userID, subject)
}

func (r *fakeSubjectRepository) GetAll(ctx context.Context, userID uuid.UUID) ([]Subject, error) {
	if r.getAllFn == nil {
		panic("unexpected call to get")
	}
	return r.getAllFn(ctx, userID)
}

func (r *fakeSubjectRepository) GetById(ctx context.Context, id uuid.UUID) (*Subject, error) {
	if r.getByIdFn == nil {
		panic("unexpected call to getById")
	}
	return r.getByIdFn(ctx, id)
}

func (r *fakeSubjectRepository) GetByName(ctx context.Context, userID uuid.UUID, name string) (*Subject, error) {
	if r.getByNameFn == nil {
		panic("unexpected call to get by name")
	}
	return r.getByNameFn(ctx, userID, name)
}

func (r *fakeSubjectRepository) Update(ctx context.Context, userID uuid.UUID, id uuid.UUID, input SubjectUpdateInput) error {
	if r.updateFn == nil {
		panic("unexpected call to update")
	}
	return r.updateFn(ctx, userID, id, input)
}

func (r *fakeSubjectRepository) DeleteById(ctx context.Context, userID uuid.UUID, id uuid.UUID) error {
	if r.deleteByIdFn == nil {
		panic("unexpected call to deleteById")
	}
	return r.deleteByIdFn(ctx, userID, id)
}

var _ SubjectRepositoryContract = (*fakeSubjectRepository)(nil)

func TestServiceCreate(t *testing.T) {
	userID := uuid.New()

	t.Run("returns error when color is invalid", func(t *testing.T) {
		ctx := context.Background()
		newFakeSubjectRepository := &fakeSubjectRepository{
			getByNameFn: func(context.Context, uuid.UUID, string) (*Subject, error) {
				return nil, ErrSubjectNotFound
			},
		}
		service := NewSubjectService(newFakeSubjectRepository)

		newSubject := SubjectCreateInput{
			Name:  "Math",
			Color: "simsalabim",
		}

		err := service.Create(ctx, userID, newSubject)
		if !errors.Is(err, ErrSubjectInvalidColor) {
			t.Fatalf("invalid subject color")
		}
	})

	t.Run("returns error when color is empty", func(t *testing.T) {
		ctx := context.Background()
		newFakeSubjectRepository := &fakeSubjectRepository{
			getByNameFn: func(context.Context, uuid.UUID, string) (*Subject, error) {
				return nil, ErrSubjectNotFound
			},
		}
		service := NewSubjectService(newFakeSubjectRepository)

		newSubject := SubjectCreateInput{
			Name:  "math",
			Color: " ",
		}

		err := service.Create(ctx, userID, newSubject)
		if !errors.Is(err, ErrSubjectRequiredColor) {
			t.Fatalf(
				"expected ErrSubjectColorRequired, got %v",
				err,
			)
		}
	})

	t.Run("returns error when name is empty", func(t *testing.T) {
		ctx := context.Background()
		newFakeSubjectRepository := &fakeSubjectRepository{
			getByNameFn: func(context.Context, uuid.UUID, string) (*Subject, error) {
				return nil, ErrSubjectNotFound
			},
		}
		service := NewSubjectService(newFakeSubjectRepository)

		newSubject := SubjectCreateInput{
			Name:  " ",
			Color: "black",
		}

		err := service.Create(ctx, userID, newSubject)
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
	userID := uuid.New()

	expected := []Subject{
		{
			Id:     uuid.New(),
			UserId: userID,
			Name:   "Math",
			Color:  ColorBlack,
		},
	}

	repository := &fakeSubjectRepository{
		getAllFn: func(ctx context.Context, gotUserID uuid.UUID) ([]Subject, error) {
			if gotUserID != userID {
				t.Fatalf("expected userID %v, got %v", userID, gotUserID)
			}
			return expected, nil
		},
	}
	service := NewSubjectService(repository)

	sub, err := service.GetAll(ctx, userID)

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
