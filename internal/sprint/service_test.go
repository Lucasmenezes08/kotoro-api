package sprint

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeSprintRepository struct {
	createFn func(
		ctx context.Context,
		payload CreateSprintModel,
	) error

	existsByDateFn func(
		ctx context.Context,
		date time.Time,
	) (bool, error)
}

func (f *fakeSprintRepository) Create(
	ctx context.Context,
	payload CreateSprintModel,
) error {
	if f.createFn == nil {
		panic("unexpected call to Create")
	}

	return f.createFn(ctx, payload)
}

func (f *fakeSprintRepository) ExistsByDate(
	ctx context.Context,
	date time.Time,
) (bool, error) {
	if f.existsByDateFn == nil {
		panic("unexpected call to ExistsByDate")
	}

	return f.existsByDateFn(ctx, date)
}

var _ SprintRepositoryContract = (*fakeSprintRepository)(nil)

func stringPointer(value string) *string {
	return &value
}

func sprintTestDate() time.Time {
	return time.Date(
		2026,
		time.September,
		25,
		0,
		0,
		0,
		0,
		time.UTC,
	)
}

func TestSprintServiceCreate(t *testing.T) {
	t.Run("creates sprint with creating status", func(t *testing.T) {
		ctx := context.Background()
		date := sprintTestDate()
		name := "Sprint de estudos"

		existsCalled := false
		createCalled := false

		repository := &fakeSprintRepository{
			existsByDateFn: func(
				ctx context.Context,
				receivedDate time.Time,
			) (bool, error) {
				existsCalled = true

				if !receivedDate.Equal(date) {
					t.Errorf(
						"expected date %v, got %v",
						date,
						receivedDate,
					)
				}

				return false, nil
			},

			createFn: func(
				ctx context.Context,
				payload CreateSprintModel,
			) error {
				createCalled = true

				if payload.Name == nil {
					t.Fatal("expected sprint name, got nil")
				}

				if *payload.Name != name {
					t.Errorf(
						"expected name %q, got %q",
						name,
						*payload.Name,
					)
				}

				if !payload.SprintDate.Equal(date) {
					t.Errorf(
						"expected date %v, got %v",
						date,
						payload.SprintDate,
					)
				}

				if payload.Status != Creating {
					t.Errorf(
						"expected status %q, got %q",
						Creating,
						payload.Status,
					)
				}

				return nil
			},
		}

		service := NewSprintService(repository)

		err := service.Create(
			ctx,
			CreateSprintModel{
				Name:       &name,
				SprintDate: date,
				Status: SprintStatus("finished"),
			},
		)

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if !existsCalled {
			t.Error("expected ExistsByDate to be called")
		}

		if !createCalled {
			t.Error("expected Create to be called")
		}
	})

	t.Run("creates sprint without name", func(t *testing.T) {
		ctx := context.Background()
		date := sprintTestDate()

		repository := &fakeSprintRepository{
			existsByDateFn: func(
				ctx context.Context,
				date time.Time,
			) (bool, error) {
				return false, nil
			},

			createFn: func(
				ctx context.Context,
				payload CreateSprintModel,
			) error {
				if payload.Name != nil {
					t.Errorf(
						"expected nil name, got %q",
						*payload.Name,
					)
				}

				if payload.Status != Creating {
					t.Errorf(
						"expected status %q, got %q",
						Creating,
						payload.Status,
					)
				}

				return nil
			},
		}

		service := NewSprintService(repository)

		err := service.Create(
			ctx,
			CreateSprintModel{
				Name:       nil,
				SprintDate: date,
			},
		)

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("returns error when name is blank", func(t *testing.T) {
		ctx := context.Background()
		name := "   "

		repository := &fakeSprintRepository{
			existsByDateFn: func(
				ctx context.Context,
				date time.Time,
			) (bool, error) {
				return false, nil
			},
		}

		service := NewSprintService(repository)

		err := service.Create(
			ctx,
			CreateSprintModel{
				Name:       &name,
				SprintDate: sprintTestDate(),
			},
		)

		if !errors.Is(err, ErrSprintNameEmpty) {
			t.Fatalf(
				"expected ErrSprintNameEmpty, got %v",
				err,
			)
		}
	})

	t.Run("returns error when sprint already exists", func(t *testing.T) {
		ctx := context.Background()

		repository := &fakeSprintRepository{
			existsByDateFn: func(
				ctx context.Context,
				date time.Time,
			) (bool, error) {
				return true, nil
			},
			createFn: nil,
		}

		service := NewSprintService(repository)

		err := service.Create(
			ctx,
			CreateSprintModel{
				SprintDate: sprintTestDate(),
			},
		)

		if !errors.Is(err, ErrSprintAlreadyExists) {
			t.Fatalf(
				"expected ErrSprintAlreadyExists, got %v",
				err,
			)
		}
	})

	t.Run("propagates ExistsByDate error", func(t *testing.T) {
		ctx := context.Background()

		repositoryError := errors.New(
			"database unavailable",
		)

		repository := &fakeSprintRepository{
			existsByDateFn: func(
				ctx context.Context,
				date time.Time,
			) (bool, error) {
				return false, repositoryError
			},
		}

		service := NewSprintService(repository)

		err := service.Create(
			ctx,
			CreateSprintModel{
				SprintDate: sprintTestDate(),
			},
		)

		if !errors.Is(err, repositoryError) {
			t.Fatalf(
				"expected repository error, got %v",
				err,
			)
		}
	})

	t.Run("propagates Create repository error", func(t *testing.T) {
		ctx := context.Background()

		repositoryError := errors.New(
			"insert sprint failed",
		)

		repository := &fakeSprintRepository{
			existsByDateFn: func(
				ctx context.Context,
				date time.Time,
			) (bool, error) {
				return false, nil
			},

			createFn: func(
				ctx context.Context,
				payload CreateSprintModel,
			) error {
				return repositoryError
			},
		}

		service := NewSprintService(repository)

		err := service.Create(
			ctx,
			CreateSprintModel{
				SprintDate: sprintTestDate(),
			},
		)

		if !errors.Is(err, repositoryError) {
			t.Fatalf(
				"expected repository error, got %v",
				err,
			)
		}
	})

	t.Run(
		"returns duplicate detected during repository Create",
		func(t *testing.T) {
			ctx := context.Background()

			repository := &fakeSprintRepository{
				existsByDateFn: func(
					ctx context.Context,
					date time.Time,
				) (bool, error) {
					return false, nil
				},

				createFn: func(
					ctx context.Context,
					payload CreateSprintModel,
				) error {
					return ErrSprintAlreadyExists
				},
			}

			service := NewSprintService(repository)

			err := service.Create(
				ctx,
				CreateSprintModel{
					SprintDate: sprintTestDate(),
				},
			)

			if !errors.Is(err, ErrSprintAlreadyExists) {
				t.Fatalf(
					"expected ErrSprintAlreadyExists, got %v",
					err,
				)
			}
		},
	)
}