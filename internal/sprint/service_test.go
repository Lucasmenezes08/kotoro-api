package sprint

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
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

	updateFn func(
		ctx context.Context,
		id uuid.UUID,
		payload UpdateSprintModel,
	) error

	getAllFn func(
		ctx context.Context,
	) ([]Sprint, error)

	getByIdFn func(
		ctx context.Context,
		id uuid.UUID,
	) (*Sprint, error)

	deleteByIdFn func(
		ctx context.Context,
		id uuid.UUID,
	) error
}

func (f *fakeSprintRepository) GetAll(ctx context.Context) ([]Sprint, error) {
	if f.getAllFn == nil {
		panic("unexpected call to GetAll")
	}

	return f.getAllFn(ctx)
}

func (f *fakeSprintRepository) GetById(
	ctx context.Context,
	id uuid.UUID,
) (*Sprint, error) {
	if f.getByIdFn == nil {
		panic("unexpected call to GetById")
	}

	return f.getByIdFn(ctx, id)
}

func (f *fakeSprintRepository) DeleteById(
	ctx context.Context,
	id uuid.UUID,
) error {
	if f.deleteByIdFn == nil {
		panic("unexpected call to DeleteById")
	}

	return f.deleteByIdFn(ctx, id)
}

func (f *fakeSprintRepository) Update(
	ctx context.Context,
	id uuid.UUID,
	payload UpdateSprintModel,
) error {
	if f.updateFn == nil {
		panic("unexpected call to Update")
	}

	return f.updateFn(ctx, id, payload)
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
	tomorrow := time.Now().UTC().AddDate(0, 0, 1)

	return time.Date(
		tomorrow.Year(),
		tomorrow.Month(),
		tomorrow.Day(),
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
				Status:     SprintStatus("finished"),
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

	t.Run("returns error when sprint date is required", func(t *testing.T) {
		service := NewSprintService(&fakeSprintRepository{})

		err := service.Create(
			context.Background(),
			CreateSprintModel{},
		)

		if !errors.Is(err, ErrSprintDateRequired) {
			t.Fatalf(
				"expected ErrSprintDateRequired, got %v",
				err,
			)
		}
	})

	t.Run("returns error when sprint date is in the past", func(t *testing.T) {
		yesterday := time.Now().UTC().AddDate(0, 0, -1)

		service := NewSprintService(&fakeSprintRepository{})

		err := service.Create(
			context.Background(),
			CreateSprintModel{SprintDate: yesterday},
		)

		if !errors.Is(err, ErrSprintDateInPast) {
			t.Fatalf(
				"expected ErrSprintDateInPast, got %v",
				err,
			)
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

func TestSprintServiceUpdate(t *testing.T) {
	t.Run("updates sprint name and status", func(t *testing.T) {
		ctx := context.Background()
		id := uuid.New()
		name := "Sprint atualizada"
		status := InProgress

		updateCalled := false

		repository := &fakeSprintRepository{
			updateFn: func(
				ctx context.Context,
				receivedID uuid.UUID,
				payload UpdateSprintModel,
			) error {
				updateCalled = true

				if receivedID != id {
					t.Errorf("expected id %q, got %q", id, receivedID)
				}

				if payload.Name == nil || *payload.Name != name {
					t.Errorf("expected name %q, got %v", name, payload.Name)
				}

				if payload.Status == nil || *payload.Status != status {
					t.Errorf("expected status %q, got %v", status, payload.Status)
				}

				return nil
			},
		}

		service := NewSprintService(repository)

		err := service.Update(
			ctx,
			id,
			UpdateSprintModel{
				Name:   &name,
				Status: &status,
			},
		)

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if !updateCalled {
			t.Error("expected Update to be called")
		}
	})

	t.Run("updates only sprint name", func(t *testing.T) {
		name := "Sprint renomeada"

		repository := &fakeSprintRepository{
			updateFn: func(
				ctx context.Context,
				id uuid.UUID,
				payload UpdateSprintModel,
			) error {
				if payload.Name == nil || *payload.Name != name {
					t.Errorf("expected name %q, got %v", name, payload.Name)
				}

				if payload.Status != nil {
					t.Errorf("expected nil status, got %q", *payload.Status)
				}

				return nil
			},
		}

		service := NewSprintService(repository)

		err := service.Update(
			context.Background(),
			uuid.New(),
			UpdateSprintModel{Name: &name},
		)

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("returns error when update is empty", func(t *testing.T) {
		service := NewSprintService(&fakeSprintRepository{})

		err := service.Update(
			context.Background(),
			uuid.New(),
			UpdateSprintModel{},
		)

		if !errors.Is(err, ErrSprintUpdateEmpty) {
			t.Fatalf("expected ErrSprintUpdateEmpty, got %v", err)
		}
	})

	t.Run("returns error when name is blank", func(t *testing.T) {
		name := "   "
		service := NewSprintService(&fakeSprintRepository{})

		err := service.Update(
			context.Background(),
			uuid.New(),
			UpdateSprintModel{Name: &name},
		)

		if !errors.Is(err, ErrSprintNameEmpty) {
			t.Fatalf("expected ErrSprintNameEmpty, got %v", err)
		}
	})

	t.Run("returns error when status is invalid", func(t *testing.T) {
		status := SprintStatus("paused")
		service := NewSprintService(&fakeSprintRepository{})

		err := service.Update(
			context.Background(),
			uuid.New(),
			UpdateSprintModel{Status: &status},
		)

		if !errors.Is(err, ErrSprintStatusInvalid) {
			t.Fatalf("expected ErrSprintStatusInvalid, got %v", err)
		}
	})

	t.Run("propagates repository error", func(t *testing.T) {
		repositoryError := errors.New("update sprint failed")
		status := Finished

		repository := &fakeSprintRepository{
			updateFn: func(
				ctx context.Context,
				id uuid.UUID,
				payload UpdateSprintModel,
			) error {
				return repositoryError
			},
		}

		service := NewSprintService(repository)

		err := service.Update(
			context.Background(),
			uuid.New(),
			UpdateSprintModel{Status: &status},
		)

		if !errors.Is(err, repositoryError) {
			t.Fatalf("expected repository error, got %v", err)
		}
	})
}

func TestSprintServiceGetAll(t *testing.T) {
	t.Run("returns all sprints", func(t *testing.T) {
		expected := []Sprint{
			{
				ID:         uuid.New(),
				SprintDate: sprintTestDate(),
				Status:     Creating,
			},
		}

		repository := &fakeSprintRepository{
			getAllFn: func(ctx context.Context) ([]Sprint, error) {
				return expected, nil
			},
		}

		service := NewSprintService(repository)

		sprints, err := service.GetAll(context.Background())
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if len(sprints) != len(expected) {
			t.Fatalf("expected %d sprint, got %d", len(expected), len(sprints))
		}

		if sprints[0].ID != expected[0].ID {
			t.Errorf("expected id %q, got %q", expected[0].ID, sprints[0].ID)
		}
	})

	t.Run("propagates repository error", func(t *testing.T) {
		repositoryError := errors.New("get all sprints failed")

		repository := &fakeSprintRepository{
			getAllFn: func(ctx context.Context) ([]Sprint, error) {
				return nil, repositoryError
			},
		}

		service := NewSprintService(repository)

		sprints, err := service.GetAll(context.Background())
		if !errors.Is(err, repositoryError) {
			t.Fatalf("expected repository error, got %v", err)
		}

		if sprints != nil {
			t.Errorf("expected nil sprints, got %v", sprints)
		}
	})
}

func TestSprintServiceGetById(t *testing.T) {
	t.Run("returns sprint by id", func(t *testing.T) {
		id := uuid.New()
		expected := &Sprint{
			ID:         id,
			SprintDate: sprintTestDate(),
			Status:     InProgress,
		}

		repository := &fakeSprintRepository{
			getByIdFn: func(
				ctx context.Context,
				receivedID uuid.UUID,
			) (*Sprint, error) {
				if receivedID != id {
					t.Errorf("expected id %q, got %q", id, receivedID)
				}

				return expected, nil
			},
		}

		service := NewSprintService(repository)

		sprint, err := service.GetById(context.Background(), id)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if sprint != expected {
			t.Errorf("expected sprint %v, got %v", expected, sprint)
		}
	})

	t.Run("propagates repository error", func(t *testing.T) {
		repositoryError := ErrSprintNotFound

		repository := &fakeSprintRepository{
			getByIdFn: func(
				ctx context.Context,
				id uuid.UUID,
			) (*Sprint, error) {
				return nil, repositoryError
			},
		}

		service := NewSprintService(repository)

		sprint, err := service.GetById(context.Background(), uuid.New())
		if !errors.Is(err, repositoryError) {
			t.Fatalf("expected repository error, got %v", err)
		}

		if sprint != nil {
			t.Errorf("expected nil sprint, got %v", sprint)
		}
	})
}

func TestSprintServiceDeleteById(t *testing.T) {
	t.Run("deletes sprint by id", func(t *testing.T) {
		id := uuid.New()
		deleteCalled := false

		repository := &fakeSprintRepository{
			deleteByIdFn: func(
				ctx context.Context,
				receivedID uuid.UUID,
			) error {
				deleteCalled = true

				if receivedID != id {
					t.Errorf("expected id %q, got %q", id, receivedID)
				}

				return nil
			},
		}

		service := NewSprintService(repository)

		err := service.DeleteById(context.Background(), id)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if !deleteCalled {
			t.Error("expected DeleteById to be called")
		}
	})

	t.Run("propagates repository error", func(t *testing.T) {
		repositoryError := ErrSprintNotFound

		repository := &fakeSprintRepository{
			deleteByIdFn: func(
				ctx context.Context,
				id uuid.UUID,
			) error {
				return repositoryError
			},
		}

		service := NewSprintService(repository)

		err := service.DeleteById(context.Background(), uuid.New())
		if !errors.Is(err, repositoryError) {
			t.Fatalf("expected repository error, got %v", err)
		}
	})
}
