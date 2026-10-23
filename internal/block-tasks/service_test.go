package blocktasks

import (
	"context"
	"errors"
	"testing"

	"github.com/Lucasmenezes08/kotoro-api.git/internal/sprint"
	sprintblocks "github.com/Lucasmenezes08/kotoro-api.git/internal/sprint-blocks"
	"github.com/google/uuid"
)

type fakeTaskRepository struct {
	createFn         func(context.Context, CreateBlockTaskModel) (*BlockTask, error)
	getAllFn         func(context.Context, uuid.UUID) ([]BlockTask, error)
	getByIDFn        func(context.Context, uuid.UUID, uuid.UUID) (*BlockTask, error)
	updateFn         func(context.Context, uuid.UUID, uuid.UUID, BlockTaskUpdateModel) (*BlockTask, error)
	deleteFn         func(context.Context, uuid.UUID, uuid.UUID) error
	reorderFn        func(context.Context, uuid.UUID, []uuid.UUID) error
	existsPositionFn func(context.Context, uuid.UUID, int, *uuid.UUID) (bool, error)
}

func (f *fakeTaskRepository) Create(ctx context.Context, input CreateBlockTaskModel) (*BlockTask, error) {
	if f.createFn == nil {
		panic("unexpected Create")
	}
	return f.createFn(ctx, input)
}
func (f *fakeTaskRepository) GetAllByBlockID(ctx context.Context, id uuid.UUID) ([]BlockTask, error) {
	if f.getAllFn == nil {
		panic("unexpected GetAllByBlockID")
	}
	return f.getAllFn(ctx, id)
}
func (f *fakeTaskRepository) GetByID(ctx context.Context, blockID, taskID uuid.UUID) (*BlockTask, error) {
	if f.getByIDFn == nil {
		panic("unexpected GetByID")
	}
	return f.getByIDFn(ctx, blockID, taskID)
}
func (f *fakeTaskRepository) Update(ctx context.Context, blockID, taskID uuid.UUID, input BlockTaskUpdateModel) (*BlockTask, error) {
	if f.updateFn == nil {
		panic("unexpected Update")
	}
	return f.updateFn(ctx, blockID, taskID, input)
}
func (f *fakeTaskRepository) Delete(ctx context.Context, blockID, taskID uuid.UUID) error {
	if f.deleteFn == nil {
		panic("unexpected Delete")
	}
	return f.deleteFn(ctx, blockID, taskID)
}
func (f *fakeTaskRepository) Reorder(ctx context.Context, blockID uuid.UUID, ids []uuid.UUID) error {
	if f.reorderFn == nil {
		panic("unexpected Reorder")
	}
	return f.reorderFn(ctx, blockID, ids)
}
func (f *fakeTaskRepository) ExistsByPosition(ctx context.Context, blockID uuid.UUID, position int, exclude *uuid.UUID) (bool, error) {
	if f.existsPositionFn == nil {
		panic("unexpected ExistsByPosition")
	}
	return f.existsPositionFn(ctx, blockID, position, exclude)
}

type fakeTaskSprintReader struct {
	fn func(context.Context, uuid.UUID) (*sprint.Sprint, error)
}

func (f fakeTaskSprintReader) GetById(ctx context.Context, id uuid.UUID) (*sprint.Sprint, error) {
	return f.fn(ctx, id)
}

type fakeTaskBlockReader struct {
	fn func(context.Context, uuid.UUID, uuid.UUID) (*sprintblocks.SprintBlock, error)
}

func (f fakeTaskBlockReader) GetByID(ctx context.Context, sprintID, blockID uuid.UUID) (*sprintblocks.SprintBlock, error) {
	return f.fn(ctx, sprintID, blockID)
}

func planningSprint() fakeTaskSprintReader {
	return fakeTaskSprintReader{fn: func(context.Context, uuid.UUID) (*sprint.Sprint, error) {
		return &sprint.Sprint{Status: sprint.Creating}, nil
	}}
}
func pendingBlock() fakeTaskBlockReader {
	return fakeTaskBlockReader{fn: func(context.Context, uuid.UUID, uuid.UUID) (*sprintblocks.SprintBlock, error) {
		return &sprintblocks.SprintBlock{Status: sprintblocks.Pending}, nil
	}}
}

func TestBlockTaskServiceCreate(t *testing.T) {
	sprintID, blockID := uuid.New(), uuid.New()
	description := "  Questões 1 a 20  "
	repo := &fakeTaskRepository{
		existsPositionFn: func(context.Context, uuid.UUID, int, *uuid.UUID) (bool, error) { return false, nil },
		createFn: func(_ context.Context, input CreateBlockTaskModel) (*BlockTask, error) {
			if input.Title != "Resolver exercícios" {
				t.Fatalf("title not normalized: %q", input.Title)
			}
			if input.Description == nil || *input.Description != "Questões 1 a 20" {
				t.Fatalf("description not normalized: %#v", input.Description)
			}
			if input.Status != TaskPending || input.Priority != PriorityMedium {
				t.Fatalf("wrong defaults: %q %q", input.Status, input.Priority)
			}
			return &BlockTask{ID: uuid.New(), SprintBlockID: blockID, Status: input.Status, Priority: input.Priority}, nil
		},
	}
	service := NewBlockTaskService(repo, planningSprint(), pendingBlock())
	_, err := service.Create(context.Background(), sprintID, blockID, CreateBlockTaskInput{Title: "  Resolver exercícios ", Description: &description, Position: 1})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
}

func TestBlockTaskServiceCreateValidation(t *testing.T) {
	zero, negative := 0, -1
	invalidPriority := TaskPriority("urgente")
	tests := []struct {
		name  string
		input CreateBlockTaskInput
		want  error
	}{
		{"empty title", CreateBlockTaskInput{Position: 1}, ErrBlockTaskTitleRequired},
		{"blank title", CreateBlockTaskInput{Title: "   ", Position: 1}, ErrBlockTaskTitleRequired},
		{"zero position", CreateBlockTaskInput{Title: "task"}, ErrBlockTaskPositionInvalid},
		{"negative position", CreateBlockTaskInput{Title: "task", Position: -1}, ErrBlockTaskPositionInvalid},
		{"invalid priority", CreateBlockTaskInput{Title: "task", Position: 1, Priority: &invalidPriority}, ErrBlockTaskPriorityInvalid},
		{"zero estimate", CreateBlockTaskInput{Title: "task", Position: 1, EstimatedSeconds: &zero}, ErrBlockTaskEstimatedSecondsInvalid},
		{"negative estimate", CreateBlockTaskInput{Title: "task", Position: 1, EstimatedSeconds: &negative}, ErrBlockTaskEstimatedSecondsInvalid},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := NewBlockTaskService(&fakeTaskRepository{}, planningSprint(), pendingBlock())
			_, err := service.Create(context.Background(), uuid.New(), uuid.New(), test.input)
			if !errors.Is(err, test.want) {
				t.Fatalf("expected %v, got %v", test.want, err)
			}
		})
	}
}

func TestBlockTaskServiceHierarchyAndState(t *testing.T) {
	valid := CreateBlockTaskInput{Title: "task", Position: 1}
	tests := []struct {
		name    string
		sprints fakeTaskSprintReader
		blocks  fakeTaskBlockReader
		want    error
	}{
		{"sprint missing", fakeTaskSprintReader{fn: func(context.Context, uuid.UUID) (*sprint.Sprint, error) { return nil, sprint.ErrSprintNotFound }}, pendingBlock(), sprint.ErrSprintNotFound},
		{"sprint not planning", fakeTaskSprintReader{fn: func(context.Context, uuid.UUID) (*sprint.Sprint, error) {
			return &sprint.Sprint{Status: sprint.InProgress}, nil
		}}, pendingBlock(), sprintblocks.ErrSprintNotInPlanning},
		{"block missing or foreign", planningSprint(), fakeTaskBlockReader{fn: func(context.Context, uuid.UUID, uuid.UUID) (*sprintblocks.SprintBlock, error) {
			return nil, sprintblocks.ErrSprintBlockNotFound
		}}, sprintblocks.ErrSprintBlockNotFound},
		{"block not planning", planningSprint(), fakeTaskBlockReader{fn: func(context.Context, uuid.UUID, uuid.UUID) (*sprintblocks.SprintBlock, error) {
			return &sprintblocks.SprintBlock{Status: sprintblocks.InProgress}, nil
		}}, ErrSprintBlockNotInPlanning},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := NewBlockTaskService(&fakeTaskRepository{}, test.sprints, test.blocks)
			_, err := service.Create(context.Background(), uuid.New(), uuid.New(), valid)
			if !errors.Is(err, test.want) {
				t.Fatalf("expected %v, got %v", test.want, err)
			}
		})
	}
}

func TestBlockTaskServiceCreateConflictsAndErrors(t *testing.T) {
	failure := errors.New("database unavailable")
	for _, test := range []struct {
		name          string
		exists        bool
		repoErr, want error
	}{
		{"position conflict", true, nil, ErrBlockTaskPositionConflict},
		{"check failure", false, failure, failure},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := &fakeTaskRepository{existsPositionFn: func(context.Context, uuid.UUID, int, *uuid.UUID) (bool, error) { return test.exists, test.repoErr }}
			service := NewBlockTaskService(repo, planningSprint(), pendingBlock())
			_, err := service.Create(context.Background(), uuid.New(), uuid.New(), CreateBlockTaskInput{Title: "task", Position: 1})
			if !errors.Is(err, test.want) {
				t.Fatalf("expected %v, got %v", test.want, err)
			}
		})
	}
}

func TestBlockTaskServiceReads(t *testing.T) {
	sprintID, blockID, taskID := uuid.New(), uuid.New(), uuid.New()
	repo := &fakeTaskRepository{
		getAllFn: func(context.Context, uuid.UUID) ([]BlockTask, error) { return nil, nil },
		getByIDFn: func(_ context.Context, gotBlock, gotTask uuid.UUID) (*BlockTask, error) {
			if gotBlock != blockID || gotTask != taskID {
				t.Fatal("wrong ids")
			}
			return &BlockTask{ID: taskID}, nil
		},
	}
	service := NewBlockTaskService(repo, planningSprint(), pendingBlock())
	tasks, err := service.GetAll(context.Background(), sprintID, blockID)
	if err != nil || tasks == nil || len(tasks) != 0 {
		t.Fatalf("expected empty slice, got %#v %v", tasks, err)
	}
	task, err := service.GetByID(context.Background(), sprintID, blockID, taskID)
	if err != nil || task.ID != taskID {
		t.Fatalf("unexpected task: %#v %v", task, err)
	}
}

func TestBlockTaskServiceReadErrors(t *testing.T) {
	failure := errors.New("read failed")
	t.Run("rejects missing sprint before repository", func(t *testing.T) {
		service := NewBlockTaskService(&fakeTaskRepository{}, fakeTaskSprintReader{fn: func(context.Context, uuid.UUID) (*sprint.Sprint, error) {
			return nil, sprint.ErrSprintNotFound
		}}, pendingBlock())
		_, err := service.GetAll(context.Background(), uuid.New(), uuid.New())
		if !errors.Is(err, sprint.ErrSprintNotFound) {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("propagates repository failure", func(t *testing.T) {
		repo := &fakeTaskRepository{getAllFn: func(context.Context, uuid.UUID) ([]BlockTask, error) { return nil, failure }}
		service := NewBlockTaskService(repo, planningSprint(), pendingBlock())
		_, err := service.GetAll(context.Background(), uuid.New(), uuid.New())
		if !errors.Is(err, failure) {
			t.Fatalf("got %v", err)
		}
	})
}

func TestBlockTaskServiceUpdate(t *testing.T) {
	sprintID, blockID, taskID := uuid.New(), uuid.New(), uuid.New()
	t.Run("empty", func(t *testing.T) {
		service := NewBlockTaskService(&fakeTaskRepository{}, planningSprint(), pendingBlock())
		_, err := service.Update(context.Background(), sprintID, blockID, taskID, BlockTaskUpdateInput{})
		if !errors.Is(err, ErrBlockTaskUpdateEmpty) {
			t.Fatalf("got %v", err)
		}
	})
	t.Run("normalizes fields and preserves absent values", func(t *testing.T) {
		title, description := "  updated ", "   "
		repo := &fakeTaskRepository{
			getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*BlockTask, error) { return &BlockTask{ID: taskID}, nil },
			updateFn: func(_ context.Context, _, _ uuid.UUID, input BlockTaskUpdateModel) (*BlockTask, error) {
				if input.Title == nil || *input.Title != "updated" {
					t.Fatal("title not normalized")
				}
				if !input.DescriptionSet || input.Description != nil {
					t.Fatal("blank description should clear")
				}
				if input.Position != nil || input.Priority != nil || input.EstimatedSeconds != nil {
					t.Fatal("absent field changed")
				}
				return &BlockTask{ID: taskID, Title: *input.Title}, nil
			},
		}
		service := NewBlockTaskService(repo, planningSprint(), pendingBlock())
		_, err := service.Update(context.Background(), sprintID, blockID, taskID, BlockTaskUpdateInput{Title: &title, Description: &description})
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("updates position priority and estimate", func(t *testing.T) {
		position, estimate, priority := 2, 1200, PriorityHigh
		repo := &fakeTaskRepository{
			getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*BlockTask, error) { return &BlockTask{ID: taskID}, nil },
			existsPositionFn: func(_ context.Context, _ uuid.UUID, got int, exclude *uuid.UUID) (bool, error) {
				if got != position || exclude == nil || *exclude != taskID {
					t.Fatal("wrong conflict check")
				}
				return false, nil
			},
			updateFn: func(_ context.Context, _, _ uuid.UUID, input BlockTaskUpdateModel) (*BlockTask, error) {
				if *input.Position != position || *input.Priority != priority || *input.EstimatedSeconds != estimate {
					t.Fatal("wrong update")
				}
				return &BlockTask{ID: taskID}, nil
			},
		}
		service := NewBlockTaskService(repo, planningSprint(), pendingBlock())
		_, err := service.Update(context.Background(), sprintID, blockID, taskID, BlockTaskUpdateInput{Position: &position, Priority: &priority, EstimatedSeconds: &estimate})
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("rejects invalid values", func(t *testing.T) {
		zero := 0
		invalid := TaskPriority("urgent")
		for _, input := range []BlockTaskUpdateInput{{Position: &zero}, {EstimatedSeconds: &zero}, {Priority: &invalid}} {
			repo := &fakeTaskRepository{getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*BlockTask, error) { return &BlockTask{ID: taskID}, nil }}
			service := NewBlockTaskService(repo, planningSprint(), pendingBlock())
			if _, err := service.Update(context.Background(), sprintID, blockID, taskID, input); err == nil {
				t.Fatal("expected validation error")
			}
		}
	})
	t.Run("rejects missing task", func(t *testing.T) {
		title := "updated"
		repo := &fakeTaskRepository{getByIDFn: func(context.Context, uuid.UUID, uuid.UUID) (*BlockTask, error) { return nil, ErrBlockTaskNotFound }}
		service := NewBlockTaskService(repo, planningSprint(), pendingBlock())
		_, err := service.Update(context.Background(), sprintID, blockID, taskID, BlockTaskUpdateInput{Title: &title})
		if !errors.Is(err, ErrBlockTaskNotFound) {
			t.Fatalf("got %v", err)
		}
	})
}

func TestBlockTaskServiceDeleteAndReorder(t *testing.T) {
	sprintID, blockID, first, second := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	t.Run("delete", func(t *testing.T) {
		called := false
		repo := &fakeTaskRepository{deleteFn: func(context.Context, uuid.UUID, uuid.UUID) error { called = true; return nil }}
		service := NewBlockTaskService(repo, planningSprint(), pendingBlock())
		if err := service.Delete(context.Background(), sprintID, blockID, first); err != nil {
			t.Fatal(err)
		}
		if !called {
			t.Fatal("expected delete")
		}
	})
	t.Run("valid reorder", func(t *testing.T) {
		called := false
		repo := &fakeTaskRepository{
			getAllFn: func(context.Context, uuid.UUID) ([]BlockTask, error) {
				return []BlockTask{{ID: first}, {ID: second}}, nil
			},
			reorderFn: func(_ context.Context, _ uuid.UUID, ids []uuid.UUID) error {
				called = ids[0] == second && ids[1] == first
				return nil
			},
		}
		service := NewBlockTaskService(repo, planningSprint(), pendingBlock())
		if err := service.Reorder(context.Background(), sprintID, blockID, []uuid.UUID{second, first}); err != nil {
			t.Fatal(err)
		}
		if !called {
			t.Fatal("expected reorder")
		}
	})
	for _, ids := range [][]uuid.UUID{{first}, {first, first}, {first, uuid.New()}} {
		repo := &fakeTaskRepository{getAllFn: func(context.Context, uuid.UUID) ([]BlockTask, error) {
			return []BlockTask{{ID: first}, {ID: second}}, nil
		}}
		service := NewBlockTaskService(repo, planningSprint(), pendingBlock())
		if err := service.Reorder(context.Background(), sprintID, blockID, ids); !errors.Is(err, ErrBlockTaskOrderInvalid) {
			t.Fatalf("expected invalid order, got %v", err)
		}
	}
	t.Run("rejects mutation outside planning", func(t *testing.T) {
		reader := fakeTaskSprintReader{fn: func(context.Context, uuid.UUID) (*sprint.Sprint, error) {
			return &sprint.Sprint{Status: sprint.Finished}, nil
		}}
		service := NewBlockTaskService(&fakeTaskRepository{}, reader, pendingBlock())
		if err := service.Delete(context.Background(), sprintID, blockID, first); !errors.Is(err, sprintblocks.ErrSprintNotInPlanning) {
			t.Fatalf("delete got %v", err)
		}
		if err := service.Reorder(context.Background(), sprintID, blockID, nil); !errors.Is(err, sprintblocks.ErrSprintNotInPlanning) {
			t.Fatalf("reorder got %v", err)
		}
	})
	t.Run("propagates delete failure", func(t *testing.T) {
		failure := errors.New("delete failed")
		repo := &fakeTaskRepository{deleteFn: func(context.Context, uuid.UUID, uuid.UUID) error { return failure }}
		service := NewBlockTaskService(repo, planningSprint(), pendingBlock())
		if err := service.Delete(context.Background(), sprintID, blockID, first); !errors.Is(err, failure) {
			t.Fatalf("got %v", err)
		}
	})
}

var _ BlockTaskRepositoryContract = (*fakeTaskRepository)(nil)
