package subject

import (
	"context"
	"errors"
	"testing"
)


type fakeSubjectRepository struct {
	createFn func(ctx context.Context, subject SubjectCreateInput)error
}


func (r *fakeSubjectRepository)Create(ctx context.Context, subject SubjectCreateInput)error{
	if r.createFn == nil {
		panic("unexpected call to create")
	}

	return r.createFn(ctx, subject)
}

var _ SubjectContract = (*fakeSubjectRepository)(nil)

func TestServiceCreate(t *testing.T){
	t.Run("returns error when color is invalid", func(t *testing.T){
		ctx := context.Background()
		newFakeSubjectRepository := &fakeSubjectRepository{}
		service := NewSubjectService(newFakeSubjectRepository)

		newSubject := SubjectCreateInput{
			Name: "Math",
			Color: "simsalabim",
		}
		
		err := service.Create(ctx, newSubject)
		if !errors.Is(err, ErrSubjectInvalidColor){
			t.Fatalf("invalid subject color")
		}
	})

	t.Run("returns error when color is empty", func(t *testing.T) {
		ctx := context.Background()
		newFakeSubjectRepository := &fakeSubjectRepository{}
		service := NewSubjectService(newFakeSubjectRepository)

		newSubject := SubjectCreateInput{
			Name: "math",
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
			Name: " ",
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