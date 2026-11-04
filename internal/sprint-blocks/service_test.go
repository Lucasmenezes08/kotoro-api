package sprintblocks

import (
	"context"
	"errors"
	"testing"

	"github.com/Lucasmenezes08/kotoro-api.git/internal/sprint"
	"github.com/Lucasmenezes08/kotoro-api.git/internal/subject"
	"github.com/google/uuid"
)

type fakeBlockRepository struct {
	createFn         func(context.Context, CreateSprintBlockModel) (*SprintBlock, error)
	getAllFn         func(context.Context, uuid.UUID) ([]SprintBlock, error)
	getByIDFn        func(context.Context, uuid.UUID, uuid.UUID) (*SprintBlock, error)
	updateFn         func(context.Context, uuid.UUID, uuid.UUID, SprintBlockUpdateModel) (*SprintBlock, error)
	deleteFn         func(context.Context, uuid.UUID, uuid.UUID) error
	reorderFn            func(context.Context, uuid.UUID, []uuid.UUID) error
	updateCanvasLayoutFn func(context.Context, uuid.UUID, []CanvasBlockLayoutItem) error
	existsSubjectFn      func(context.Context, uuid.UUID, uuid.UUID, *uuid.UUID) (bool, error)
	existsPositionFn func(context.Context, uuid.UUID, int, *uuid.UUID) (bool, error)
}

func (f *fakeBlockRepository) Create(ctx context.Context, input CreateSprintBlockModel) (*SprintBlock, error) {
	if f.createFn == nil {
		panic("unexpected Create")
	}
	return f.createFn(ctx, input)
}
func (f *fakeBlockRepository) GetAllBySprintID(ctx context.Context, id uuid.UUID) ([]SprintBlock, error) {
	if f.getAllFn == nil {
		panic("unexpected GetAllBySprintID")
	}
	return f.getAllFn(ctx, id)
}
func (f *fakeBlockRepository) GetByID(ctx context.Context, sprintID, blockID uuid.UUID) (*SprintBlock, error) {
	if f.getByIDFn == nil {
		panic("unexpected GetByID")
	}
	return f.getByIDFn(ctx, sprintID, blockID)
}
func (f *fakeBlockRepository) Update(ctx context.Context, sprintID, blockID uuid.UUID, input SprintBlockUpdateModel) (*SprintBlock, error) {
	if f.updateFn == nil {
		panic("unexpected Update")
	}
	return f.updateFn(ctx, sprintID, blockID, input)
}
func (f *fakeBlockRepository) Delete(ctx context.Context, sprintID, blockID uuid.UUID) error {
	if f.deleteFn == nil {
		panic("unexpected Delete")
	}
	return f.deleteFn(ctx, sprintID, blockID)
}
func (f *fakeBlockRepository) Reorder(ctx context.Context, sprintID uuid.UUID, ids []uuid.UUID) error {
	if f.reorderFn == nil {
		panic("unexpected Reorder")
	}
	return f.reorderFn(ctx, sprintID, ids)
}
func (f *fakeBlockRepository) UpdateCanvasLayout(ctx context.Context, sprintID uuid.UUID, items []CanvasBlockLayoutItem) error {
	if f.updateCanvasLayoutFn != nil {
		return f.updateCanvasLayoutFn(ctx, sprintID, items)
	}
	return nil
}
func (f *fakeBlockRepository) ExistsBySubjectID(ctx context.Context, sprintID, subjectID uuid.UUID, exclude *uuid.UUID) (bool, error) {
	if f.existsSubjectFn == nil {
		panic("unexpected ExistsBySubjectID")
	}
	return f.existsSubjectFn(ctx, sprintID, subjectID, exclude)
}
func (f *fakeBlockRepository) ExistsByPosition(ctx context.Context, sprintID uuid.UUID, position int, exclude *uuid.UUID) (bool, error) {
	if f.existsPositionFn == nil {
		panic("unexpected ExistsByPosition")
	}
	return f.existsPositionFn(ctx, sprintID, position, exclude)
}

type fakeSprintReader struct {
	fn func(context.Context, uuid.UUID) (*sprint.Sprint, error)
}

func (f fakeSprintReader) GetById(ctx context.Context, id uuid.UUID) (*sprint.Sprint, error) {
	return f.fn(ctx, id)
}

type fakeSubjectReader struct {
	fn func(context.Context, uuid.UUID) (*subject.Subject, error)
}

func (f fakeSubjectReader) GetById(ctx context.Context, id uuid.UUID) (*subject.Subject, error) {
	return f.fn(ctx, id)
}

var defaultTestUserID = uuid.MustParse("00000000-0000-0000-0000-000000000001")

func creatingSprintReader() fakeSprintReader {
	return fakeSprintReader{fn: func(context.Context, uuid.UUID) (*sprint.Sprint, error) {
		return &sprint.Sprint{Status: sprint.Creating, UserID: defaultTestUserID}, nil
	}}
}
func existingSubjectReader() fakeSubjectReader {
	return fakeSubjectReader{fn: func(context.Context, uuid.UUID) (*subject.Subject, error) {
		return &subject.Subject{UserId: defaultTestUserID}, nil
	}}
}

func TestSprintBlockServiceCreate(t *testing.T) {
	sprintID, subjectID := uuid.New(), uuid.New()
	title := "  Algebra  "
	estimate := 3600
	created := false
	repo := &fakeBlockRepository{
		existsSubjectFn:  func(context.Context, uuid.UUID, uuid.UUID, *uuid.UUID) (bool, error) { return false, nil },
		existsPositionFn: func(context.Context, uuid.UUID, int, *uuid.UUID) (bool, error) { return false, nil },
		createFn: func(_ context.Context, input CreateSprintBlockModel) (*SprintBlock, error) {
			created = true
			if input.Status != Pending {
				t.Fatalf("expected pending status, got %q", input.Status)
			}
			if input.Title == nil || *input.Title != "Algebra" {
				t.Fatalf("title was not normalized: %#v", input.Title)
			}
			return &SprintBlock{ID: uuid.New(), SprintID: sprintID, SubjectID: subjectID, Status: input.Status}, nil
		},
	}
	service := NewSprintBlockService(repo, creatingSprintReader(), existingSubjectReader())
	_, err := service.Create(context.Background(), defaultTestUserID, sprintID, CreateSprintBlockInput{
		SubjectID: subjectID, Title: &title, Position: 1, EstimatedSeconds: &estimate,
	})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if !created {
		t.Fatal("expected Create to be called")
	}
}

func TestSprintBlockServiceCreateValidation(t *testing.T) {
	sprintID, subjectID := uuid.New(), uuid.New()
	zero, negative := 0, -1
	blank := "   "
	tests := []struct {
		name  string
		input CreateSprintBlockInput
		want  error
	}{
		{"zero position", CreateSprintBlockInput{SubjectID: subjectID, Position: 0}, ErrSprintBlockPositionInvalid},
		{"negative position", CreateSprintBlockInput{SubjectID: subjectID, Position: -1}, ErrSprintBlockPositionInvalid},
		{"zero estimate", CreateSprintBlockInput{SubjectID: subjectID, Position: 1, EstimatedSeconds: &zero}, ErrSprintBlockEstimatedSecondsInvalid},
		{"negative estimate", CreateSprintBlockInput{SubjectID: subjectID, Position: 1, EstimatedSeconds: &negative}, ErrSprintBlockEstimatedSecondsInvalid},
		{"blank title", CreateSprintBlockInput{SubjectID: subjectID, Position: 1, Title: &blank}, ErrSprintBlockTitleInvalid},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := NewSprintBlockService(&fakeBlockRepository{}, creatingSprintReader(), existingSubjectReader())
			_, err := service.Create(context.Background(), defaultTestUserID, sprintID, test.input)
			if !errors.Is(err, test.want) {
				t.Fatalf("expected %v, got %v", test.want, err)
			}
		})
	}
}

func TestSprintBlockServiceCreateRelationsAndConflicts(t *testing.T) {
	sprintID, subjectID := uuid.New(), uuid.New()
	t.Run("sprint not found", func(t *testing.T) {
		service := NewSprintBlockService(&fakeBlockRepository{}, fakeSprintReader{fn: func(context.Context, uuid.UUID) (*sprint.Sprint, error) {
			return nil, sprint.ErrSprintNotFound
		}}, existingSubjectReader())
		_, err := service.Create(context.Background(), defaultTestUserID, sprintID, CreateSprintBlockInput{SubjectID: subjectID, Position: 1})
		if !errors.Is(err, sprint.ErrSprintNotFound) {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("sprint outside planning", func(t *testing.T) {
		service := NewSprintBlockService(&fakeBlockRepository{}, fakeSprintReader{fn: func(context.Context, uuid.UUID) (*sprint.Sprint, error) {
			return &sprint.Sprint{Status: sprint.Finished, UserID: defaultTestUserID}, nil
		}}, existingSubjectReader())
		_, err := service.Create(context.Background(), defaultTestUserID, sprintID, CreateSprintBlockInput{SubjectID: subjectID, Position: 1})
		if !errors.Is(err, ErrSprintNotInPlanning) {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("subject not found", func(t *testing.T) {
		service := NewSprintBlockService(&fakeBlockRepository{}, creatingSprintReader(), fakeSubjectReader{fn: func(context.Context, uuid.UUID) (*subject.Subject, error) {
			return nil, subject.ErrSubjectNotFound
		}})
		_, err := service.Create(context.Background(), defaultTestUserID, sprintID, CreateSprintBlockInput{SubjectID: subjectID, Position: 1})
		if !errors.Is(err, subject.ErrSubjectNotFound) {
			t.Fatalf("got %v", err)
		}
	})
	for _, test := range []struct {
		name                              string
		subjectConflict, positionConflict bool
		want                              error
	}{
		{"subject conflict", true, false, ErrSprintBlockSubjectConflict},
		{"position conflict", false, true, ErrSprintBlockPositionConflict},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := &fakeBlockRepository{
				existsSubjectFn: func(context.Context, uuid.UUID, uuid.UUID, *uuid.UUID) (bool, error) {
					return test.subjectConflict, nil
				},
				existsPositionFn: func(context.Context, uuid.UUID, int, *uuid.UUID) (bool, error) { return test.positionConflict, nil },
			}
			service := NewSprintBlockService(repo, creatingSprintReader(), existingSubjectReader())
			_, err := service.Create(context.Background(), defaultTestUserID, sprintID, CreateSprintBlockInput{SubjectID: subjectID, Position: 1})
			if !errors.Is(err, test.want) {
				t.Fatalf("expected %v, got %v", test.want, err)
			}
		})
	}
}

func TestSprintBlockServiceReads(t *testing.T) {
	sprintID, blockID := uuid.New(), uuid.New()
	repo := &fakeBlockRepository{
		getAllFn: func(context.Context, uuid.UUID) ([]SprintBlock, error) { return nil, nil },
		getByIDFn: func(_ context.Context, gotSprint, gotBlock uuid.UUID) (*SprintBlock, error) {
			if gotSprint != sprintID || gotBlock != blockID {
				t.Fatal("wrong identifiers")
			}
			return &SprintBlock{ID: blockID}, nil
		},
	}
	service := NewSprintBlockService(repo, creatingSprintReader(), existingSubjectReader())
	blocks, err := service.GetAll(context.Background(), defaultTestUserID, sprintID)
	if err != nil || blocks == nil || len(blocks) != 0 {
		t.Fatalf("expected empty slice, got %#v, %v", blocks, err)
	}
	block, err := service.GetByID(context.Background(), defaultTestUserID, sprintID, blockID)
	if err != nil || block.ID != blockID {
		t.Fatalf("unexpected block/error: %#v %v", block, err)
	}
}

func TestSprintBlockServiceUpdate(t *testing.T) {
	sprintID, blockID := uuid.New(), uuid.New()
	t.Run("empty payload", func(t *testing.T) {
		service := NewSprintBlockService(&fakeBlockRepository{}, creatingSprintReader(), existingSubjectReader())
		_, err := service.Update(context.Background(), defaultTestUserID, sprintID, blockID, SprintBlockUpdateModel{})
		if !errors.Is(err, ErrSprintBlockUpdateEmpty) {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("updates title only and keeps absent fields", func(t *testing.T) {
		title := "  New title "
		repo := &fakeBlockRepository{
			getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*SprintBlock, error) {
				return &SprintBlock{ID: blockID}, nil
			},
			updateFn: func(_ context.Context, _, _ uuid.UUID, input SprintBlockUpdateModel) (*SprintBlock, error) {
				if input.Title == nil || *input.Title != "New title" {
					t.Fatalf("unexpected title: %#v", input.Title)
				}
				if input.SubjectID != nil || input.Position != nil || input.EstimatedSeconds != nil {
					t.Fatal("absent fields changed")
				}
				return &SprintBlock{ID: blockID, Title: input.Title}, nil
			},
		}
		service := NewSprintBlockService(repo, creatingSprintReader(), existingSubjectReader())
		_, err := service.Update(context.Background(), defaultTestUserID, sprintID, blockID, SprintBlockUpdateModel{Title: &title})
		if err != nil {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("rejects missing block", func(t *testing.T) {
		position := 2
		repo := &fakeBlockRepository{getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*SprintBlock, error) { return nil, ErrSprintBlockNotFound }}
		service := NewSprintBlockService(repo, creatingSprintReader(), existingSubjectReader())
		_, err := service.Update(context.Background(), defaultTestUserID, sprintID, blockID, SprintBlockUpdateModel{Position: &position})
		if !errors.Is(err, ErrSprintBlockNotFound) {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("updates subject position and estimate", func(t *testing.T) {
		subjectID := uuid.New()
		position, estimate := 2, 1800
		repo := &fakeBlockRepository{
			getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*SprintBlock, error) {
				return &SprintBlock{ID: blockID}, nil
			},
			existsSubjectFn: func(_ context.Context, _, gotSubject uuid.UUID, exclude *uuid.UUID) (bool, error) {
				if gotSubject != subjectID || exclude == nil || *exclude != blockID {
					t.Fatal("incorrect subject conflict check")
				}
				return false, nil
			},
			existsPositionFn: func(_ context.Context, _ uuid.UUID, gotPosition int, exclude *uuid.UUID) (bool, error) {
				if gotPosition != position || exclude == nil || *exclude != blockID {
					t.Fatal("incorrect position conflict check")
				}
				return false, nil
			},
			updateFn: func(_ context.Context, _, _ uuid.UUID, input SprintBlockUpdateModel) (*SprintBlock, error) {
				if input.SubjectID == nil || *input.SubjectID != subjectID || input.Position == nil || *input.Position != position || input.EstimatedSeconds == nil || *input.EstimatedSeconds != estimate {
					t.Fatalf("incorrect update input: %#v", input)
				}
				return &SprintBlock{ID: blockID}, nil
			},
		}
		service := NewSprintBlockService(repo, creatingSprintReader(), existingSubjectReader())
		_, err := service.Update(context.Background(), defaultTestUserID, sprintID, blockID, SprintBlockUpdateModel{
			SubjectID: &subjectID, Position: &position, EstimatedSeconds: &estimate,
		})
		if err != nil {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("rejects invalid update values", func(t *testing.T) {
		zero := 0
		for _, input := range []SprintBlockUpdateModel{{Position: &zero}, {EstimatedSeconds: &zero}} {
			repo := &fakeBlockRepository{getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*SprintBlock, error) {
				return &SprintBlock{ID: blockID}, nil
			}}
			service := NewSprintBlockService(repo, creatingSprintReader(), existingSubjectReader())
			if _, err := service.Update(context.Background(), defaultTestUserID, sprintID, blockID, input); err == nil {
				t.Fatal("expected validation error")
			}
		}
	})
	t.Run("propagates repository error", func(t *testing.T) {
		title := "valid"
		failure := errors.New("write failed")
		repo := &fakeBlockRepository{
			getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*SprintBlock, error) {
				return &SprintBlock{ID: blockID}, nil
			},
			updateFn: func(context.Context, uuid.UUID, uuid.UUID, SprintBlockUpdateModel) (*SprintBlock, error) {
				return nil, failure
			},
		}
		service := NewSprintBlockService(repo, creatingSprintReader(), existingSubjectReader())
		_, err := service.Update(context.Background(), defaultTestUserID, sprintID, blockID, SprintBlockUpdateModel{Title: &title})
		if !errors.Is(err, failure) {
			t.Fatalf("expected wrapped failure, got %v", err)
		}
	})
	t.Run("allows updating pending block when sprint is in_progress", func(t *testing.T) {
		title := "new title"
		inProgressSprintReader := fakeSprintReader{fn: func(context.Context, uuid.UUID) (*sprint.Sprint, error) {
			return &sprint.Sprint{Status: sprint.InProgress, UserID: defaultTestUserID}, nil
		}}
		repo := &fakeBlockRepository{
			getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*SprintBlock, error) {
				return &SprintBlock{ID: blockID, Status: Pending}, nil
			},
			updateFn: func(context.Context, uuid.UUID, uuid.UUID, SprintBlockUpdateModel) (*SprintBlock, error) {
				return &SprintBlock{ID: blockID, Status: Pending, Title: &title}, nil
			},
		}
		service := NewSprintBlockService(repo, inProgressSprintReader, existingSubjectReader())
		updated, err := service.Update(context.Background(), defaultTestUserID, sprintID, blockID, SprintBlockUpdateModel{Title: &title})
		if err != nil || updated == nil {
			t.Fatalf("expected successful update of pending block in in_progress sprint, got %v", err)
		}
	})
	t.Run("rejects updating non-pending block when sprint is in_progress", func(t *testing.T) {
		title := "new title"
		inProgressSprintReader := fakeSprintReader{fn: func(context.Context, uuid.UUID) (*sprint.Sprint, error) {
			return &sprint.Sprint{Status: sprint.InProgress, UserID: defaultTestUserID}, nil
		}}
		repo := &fakeBlockRepository{
			getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*SprintBlock, error) {
				return &SprintBlock{ID: blockID, Status: InProgress}, nil
			},
		}
		service := NewSprintBlockService(repo, inProgressSprintReader, existingSubjectReader())
		_, err := service.Update(context.Background(), defaultTestUserID, sprintID, blockID, SprintBlockUpdateModel{Title: &title})
		if !errors.Is(err, ErrSprintBlockNotEditable) {
			t.Fatalf("expected ErrSprintBlockNotEditable, got %v", err)
		}
	})
}

func TestSprintBlockServiceDeleteAndReorder(t *testing.T) {
	sprintID, first, second := uuid.New(), uuid.New(), uuid.New()
	t.Run("delete", func(t *testing.T) {
		called := false
		repo := &fakeBlockRepository{deleteFn: func(context.Context, uuid.UUID, uuid.UUID) error { called = true; return nil }}
		service := NewSprintBlockService(repo, creatingSprintReader(), existingSubjectReader())
		if err := service.Delete(context.Background(), defaultTestUserID, sprintID, first); err != nil {
			t.Fatal(err)
		}
		if !called {
			t.Fatal("expected Delete")
		}
	})
	t.Run("valid reorder", func(t *testing.T) {
		called := false
		repo := &fakeBlockRepository{
			getAllFn: func(context.Context, uuid.UUID) ([]SprintBlock, error) {
				return []SprintBlock{{ID: first}, {ID: second}}, nil
			},
			reorderFn: func(_ context.Context, _ uuid.UUID, ids []uuid.UUID) error {
				called = ids[0] == second && ids[1] == first
				return nil
			},
		}
		service := NewSprintBlockService(repo, creatingSprintReader(), existingSubjectReader())
		if err := service.Reorder(context.Background(), defaultTestUserID, sprintID, []uuid.UUID{second, first}); err != nil {
			t.Fatal(err)
		}
		if !called {
			t.Fatal("expected reordered ids")
		}
	})
	for _, ids := range [][]uuid.UUID{{first}, {first, first}, {first, uuid.New()}} {
		repo := &fakeBlockRepository{getAllFn: func(context.Context, uuid.UUID) ([]SprintBlock, error) {
			return []SprintBlock{{ID: first}, {ID: second}}, nil
		}}
		service := NewSprintBlockService(repo, creatingSprintReader(), existingSubjectReader())
		if err := service.Reorder(context.Background(), defaultTestUserID, sprintID, ids); !errors.Is(err, ErrInvalidSprintBlockOrder) {
			t.Fatalf("expected invalid order for %#v, got %v", ids, err)
		}
	}
	t.Run("rejects reorder outside planning", func(t *testing.T) {
		reader := fakeSprintReader{fn: func(context.Context, uuid.UUID) (*sprint.Sprint, error) {
			return &sprint.Sprint{Status: sprint.Finished, UserID: defaultTestUserID}, nil
		}}
		service := NewSprintBlockService(&fakeBlockRepository{}, reader, existingSubjectReader())
		if err := service.Reorder(context.Background(), defaultTestUserID, sprintID, nil); !errors.Is(err, ErrSprintNotInPlanning) {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("rejects access when sprint belongs to another user", func(t *testing.T) {
		otherUserID := uuid.MustParse("00000000-0000-0000-0000-000000000099")
		reader := fakeSprintReader{fn: func(context.Context, uuid.UUID) (*sprint.Sprint, error) {
			return &sprint.Sprint{Status: sprint.Creating, UserID: otherUserID}, nil
		}}
		service := NewSprintBlockService(&fakeBlockRepository{}, reader, existingSubjectReader())
		if _, err := service.GetAll(context.Background(), defaultTestUserID, sprintID); !errors.Is(err, sprint.ErrSprintNotFound) {
			t.Fatalf("expected ErrSprintNotFound, got %v", err)
		}
		if _, err := service.GetByID(context.Background(), defaultTestUserID, sprintID, first); !errors.Is(err, sprint.ErrSprintNotFound) {
			t.Fatalf("expected ErrSprintNotFound, got %v", err)
		}
		if err := service.Delete(context.Background(), defaultTestUserID, sprintID, first); !errors.Is(err, sprint.ErrSprintNotFound) {
			t.Fatalf("expected ErrSprintNotFound, got %v", err)
		}
	})
	t.Run("rejects create when subject belongs to another user", func(t *testing.T) {
		otherUserID := uuid.MustParse("00000000-0000-0000-0000-000000000099")
		service := NewSprintBlockService(&fakeBlockRepository{}, creatingSprintReader(), fakeSubjectReader{fn: func(context.Context, uuid.UUID) (*subject.Subject, error) {
			return &subject.Subject{UserId: otherUserID}, nil
		}})
		_, err := service.Create(context.Background(), defaultTestUserID, sprintID, CreateSprintBlockInput{SubjectID: uuid.New(), Position: 1})
		if !errors.Is(err, subject.ErrSubjectNotFound) {
			t.Fatalf("expected ErrSubjectNotFound, got %v", err)
		}
	})
	t.Run("UpdateCanvasLayout persists layout atomically", func(t *testing.T) {
		block1 := uuid.New()
		block2 := uuid.New()
		called := false
		repo := &fakeBlockRepository{
			getAllFn: func(context.Context, uuid.UUID) ([]SprintBlock, error) {
				return []SprintBlock{
					{ID: block1, Position: 1, Status: Pending},
					{ID: block2, Position: 2, Status: Pending},
				}, nil
			},
			updateCanvasLayoutFn: func(ctx context.Context, sid uuid.UUID, items []CanvasBlockLayoutItem) error {
				called = true
				if len(items) != 2 || items[0].X != 100.5 || items[1].Y != 200.0 {
					t.Fatalf("unexpected canvas layout items: %#v", items)
				}
				return nil
			},
		}
		service := NewSprintBlockService(repo, creatingSprintReader(), existingSubjectReader())
		err := service.UpdateCanvasLayout(context.Background(), defaultTestUserID, sprintID, UpdateCanvasLayoutInput{
			Blocks: []CanvasBlockLayoutItem{
				{ID: block1, Position: 2, X: 100.5, Y: 50.0},
				{ID: block2, Position: 1, X: 300.0, Y: 200.0},
			},
		})
		if err != nil || !called {
			t.Fatalf("expected successful canvas layout update, got %v", err)
		}
	})
	t.Run("UpdateCanvasLayout locks positions of non-pending blocks in in_progress sprint", func(t *testing.T) {
		block1 := uuid.New()
		block2 := uuid.New()
		inProgressReader := fakeSprintReader{fn: func(context.Context, uuid.UUID) (*sprint.Sprint, error) {
			return &sprint.Sprint{Status: sprint.InProgress, UserID: defaultTestUserID}, nil
		}}
		repo := &fakeBlockRepository{
			getAllFn: func(context.Context, uuid.UUID) ([]SprintBlock, error) {
				return []SprintBlock{
					{ID: block1, Position: 1, Status: InProgress},
					{ID: block2, Position: 2, Status: Pending},
				}, nil
			},
		}
		service := NewSprintBlockService(repo, inProgressReader, existingSubjectReader())
		// Trying to change position of block1 (which is InProgress) should fail
		err := service.UpdateCanvasLayout(context.Background(), defaultTestUserID, sprintID, UpdateCanvasLayoutInput{
			Blocks: []CanvasBlockLayoutItem{
				{ID: block1, Position: 2, X: 100.0, Y: 100.0},
				{ID: block2, Position: 1, X: 200.0, Y: 200.0},
			},
		})
		if !errors.Is(err, ErrInvalidSprintBlockOrder) {
			t.Fatalf("expected ErrInvalidSprintBlockOrder when moving in-progress block, got %v", err)
		}

		// Updating x, y of block1 while keeping position 1 should succeed
		called := false
		repo.updateCanvasLayoutFn = func(ctx context.Context, sid uuid.UUID, items []CanvasBlockLayoutItem) error {
			called = true
			return nil
		}
		err = service.UpdateCanvasLayout(context.Background(), defaultTestUserID, sprintID, UpdateCanvasLayoutInput{
			Blocks: []CanvasBlockLayoutItem{
				{ID: block1, Position: 1, X: 500.0, Y: 500.0},
				{ID: block2, Position: 2, X: 600.0, Y: 600.0},
			},
		})
		if err != nil || !called {
			t.Fatalf("expected successful canvas coordinate update preserving position, got %v", err)
		}
	})
}

var _ SprintBlockRepositoryContract = (*fakeBlockRepository)(nil)
