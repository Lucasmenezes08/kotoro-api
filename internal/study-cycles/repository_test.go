package studycycles

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	blocktasks "github.com/Lucasmenezes08/kotoro-api.git/internal/block-tasks"
	"github.com/Lucasmenezes08/kotoro-api.git/internal/database"
	"github.com/Lucasmenezes08/kotoro-api.git/internal/sprint"
	sprintblocks "github.com/Lucasmenezes08/kotoro-api.git/internal/sprint-blocks"
	"github.com/Lucasmenezes08/kotoro-api.git/migrations"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"
)

// inMemoryConcurrentRepo simulates PostgreSQL row locking, conditional updates,
// unique constraints, and transactional rollback for deterministic concurrency testing.
type inMemoryConcurrentRepo struct {
	mu          sync.Mutex
	sprints     map[uuid.UUID]sprint.Sprint
	blocks      map[uuid.UUID]sprintblocks.SprintBlock
	tasks       map[uuid.UUID]blocktasks.BlockTask
	cycles      map[uuid.UUID]StudyCycle
	cycleTasks  map[uuid.UUID][]StudyCycleTask
	failOnBlock bool
}

func newInMemoryConcurrentRepo() *inMemoryConcurrentRepo {
	return &inMemoryConcurrentRepo{
		sprints:    make(map[uuid.UUID]sprint.Sprint),
		blocks:     make(map[uuid.UUID]sprintblocks.SprintBlock),
		tasks:      make(map[uuid.UUID]blocktasks.BlockTask),
		cycles:     make(map[uuid.UUID]StudyCycle),
		cycleTasks: make(map[uuid.UUID][]StudyCycleTask),
	}
}

type inMemoryTx struct {
	parent     *inMemoryConcurrentRepo
	sprints    map[uuid.UUID]sprint.Sprint
	blocks     map[uuid.UUID]sprintblocks.SprintBlock
	tasks      map[uuid.UUID]blocktasks.BlockTask
	cycles     map[uuid.UUID]StudyCycle
	cycleTasks map[uuid.UUID][]StudyCycleTask
}

func (r *inMemoryConcurrentRepo) WithinTransaction(ctx context.Context, fn func(ctx context.Context, tx StudyCycleTxRepositoryContract) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	tx := &inMemoryTx{
		parent:     r,
		sprints:    make(map[uuid.UUID]sprint.Sprint, len(r.sprints)),
		blocks:     make(map[uuid.UUID]sprintblocks.SprintBlock, len(r.blocks)),
		tasks:      make(map[uuid.UUID]blocktasks.BlockTask, len(r.tasks)),
		cycles:     make(map[uuid.UUID]StudyCycle, len(r.cycles)),
		cycleTasks: make(map[uuid.UUID][]StudyCycleTask, len(r.cycleTasks)),
	}
	for k, v := range r.sprints {
		tx.sprints[k] = v
	}
	for k, v := range r.blocks {
		tx.blocks[k] = v
	}
	for k, v := range r.tasks {
		tx.tasks[k] = v
	}
	for k, v := range r.cycles {
		tx.cycles[k] = v
	}
	for k, v := range r.cycleTasks {
		copied := make([]StudyCycleTask, len(v))
		copy(copied, v)
		tx.cycleTasks[k] = copied
	}

	if err := fn(ctx, tx); err != nil {
		// Rollback: discard tx state
		return err
	}

	// Commit: replace parent state with tx state
	r.sprints = tx.sprints
	r.blocks = tx.blocks
	r.tasks = tx.tasks
	r.cycles = tx.cycles
	r.cycleTasks = tx.cycleTasks
	return nil
}

func (r *inMemoryConcurrentRepo) GetByID(_ context.Context, cycleID uuid.UUID) (*StudyCycle, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.cycles[cycleID]
	if !ok {
		return nil, ErrStudyCycleNotFound
	}
	copied := c
	return &copied, nil
}

func (r *inMemoryConcurrentRepo) GetSprintByID(_ context.Context, sprintID uuid.UUID) (*sprint.Sprint, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	sp, ok := r.sprints[sprintID]
	if !ok || sp.DeletedAt != nil {
		return nil, sprint.ErrSprintNotFound
	}
	if sp.UserID == uuid.Nil {
		sp.UserID = defaultTestUserID
	}
	copied := sp
	return &copied, nil
}

func (r *inMemoryConcurrentRepo) PauseConditional(_ context.Context, input PauseStudyCycleModel) (*StudyCycle, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.cycles[input.ID]
	if !ok {
		return nil, ErrStudyCycleNotFound
	}
	if c.Status != input.ExpectedStatus {
		switch c.Status {
		case StudyCyclePaused:
			return nil, ErrStudyCycleNotRunning
		case StudyCycleFinished, StudyCycleCancelled:
			return nil, errPauseStoppedCycle
		default:
			return nil, ErrStudyCycleInvalidTransition
		}
	}
	pausedAt := input.PausedAt.UTC()
	c.ElapsedSeconds = c.ElapsedSeconds + calculateAdditionalElapsedSeconds(c.RunningSince, pausedAt)
	c.Status = input.Status
	c.PausedAt = &pausedAt
	c.RunningSince = nil
	c.UpdatedAt = &pausedAt
	r.cycles[input.ID] = c
	copied := c
	return &copied, nil
}

func (r *inMemoryConcurrentRepo) ResumeConditional(_ context.Context, input ResumeStudyCycleModel) (*StudyCycle, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.cycles[input.ID]
	if !ok {
		return nil, ErrStudyCycleNotFound
	}
	if c.Status != input.ExpectedStatus {
		switch c.Status {
		case StudyCycleInProgress:
			return nil, ErrStudyCycleNotPaused
		case StudyCycleFinished, StudyCycleCancelled:
			return nil, errResumeStoppedCycle
		default:
			return nil, ErrStudyCycleInvalidTransition
		}
	}
	runningSince := input.RunningSince.UTC()
	c.Status = input.Status
	c.RunningSince = &runningSince
	c.PausedAt = nil
	c.UpdatedAt = &runningSince
	r.cycles[input.ID] = c
	copied := c
	return &copied, nil
}

func (tx *inMemoryTx) LockSprintByID(_ context.Context, sprintID uuid.UUID) (*sprint.Sprint, error) {
	sp, ok := tx.sprints[sprintID]
	if !ok || sp.DeletedAt != nil {
		return nil, sprint.ErrSprintNotFound
	}
	if sp.UserID == uuid.Nil {
		sp.UserID = defaultTestUserID
	}
	copied := sp
	return &copied, nil
}

func (tx *inMemoryTx) HasActiveCycleBySprintID(_ context.Context, sprintID uuid.UUID) (bool, error) {
	for _, c := range tx.cycles {
		if c.SprintID == sprintID && (c.Status == StudyCycleInProgress || c.Status == StudyCyclePaused) {
			return true, nil
		}
	}
	return false, nil
}

func (tx *inMemoryTx) LockBlocksBySprintID(_ context.Context, sprintID uuid.UUID) ([]sprintblocks.SprintBlock, error) {
	var list []sprintblocks.SprintBlock
	for _, b := range tx.blocks {
		if b.SprintID == sprintID && b.DeletedAt == nil {
			list = append(list, b)
		}
	}
	return list, nil
}

func (tx *inMemoryTx) GetActiveTasksByBlockID(_ context.Context, blockID uuid.UUID) ([]blocktasks.BlockTask, error) {
	var list []blocktasks.BlockTask
	for _, t := range tx.tasks {
		if t.SprintBlockID == blockID && t.DeletedAt == nil {
			list = append(list, t)
		}
	}
	return list, nil
}

func (tx *inMemoryTx) GetNextSequenceNumber(_ context.Context, sprintID uuid.UUID) (int, error) {
	maxSeq := 0
	for _, c := range tx.cycles {
		if c.SprintID == sprintID && c.SequenceNumber > maxSeq {
			maxSeq = c.SequenceNumber
		}
	}
	return maxSeq + 1, nil
}

func (tx *inMemoryTx) CreateCycle(_ context.Context, input CreateStudyCycleModel) (*StudyCycle, error) {
	for _, c := range tx.cycles {
		if c.SprintID == input.SprintID && (c.Status == StudyCycleInProgress || c.Status == StudyCyclePaused) {
			return nil, ErrStudyCycleAlreadyActive
		}
		if c.SprintID == input.SprintID && c.SequenceNumber == input.SequenceNumber {
			return nil, ErrStudyCycleAlreadyActive
		}
	}
	cycle := StudyCycle{
		ID:                     uuid.New(),
		SprintID:               input.SprintID,
		SprintBlockID:          input.SprintBlockID,
		SequenceNumber:         input.SequenceNumber,
		ExecutionMode:          input.ExecutionMode,
		TimerType:              input.TimerType,
		Status:                 input.Status,
		PlannedDurationSeconds: input.PlannedDurationSeconds,
		StartedAt:              input.StartedAt,
		RunningSince:           input.RunningSince,
		PausedAt:               input.PausedAt,
		EndedAt:                input.EndedAt,
		ElapsedSeconds:         input.ElapsedSeconds,
		StopReason:             input.StopReason,
		CreatedAt:              input.StartedAt,
		UpdatedAt:              &input.StartedAt,
	}
	tx.cycles[cycle.ID] = cycle
	return &cycle, nil
}

func (tx *inMemoryTx) AttachCycleTasks(_ context.Context, cycleID uuid.UUID, taskIDs []uuid.UUID) error {
	now := time.Now().UTC()
	for _, id := range taskIDs {
		tx.cycleTasks[cycleID] = append(tx.cycleTasks[cycleID], StudyCycleTask{
			StudyCycleID: cycleID,
			BlockTaskID:  id,
			CreatedAt:    now,
		})
	}
	return nil
}

func (tx *inMemoryTx) UpdateBlockStatus(_ context.Context, blockID uuid.UUID, status sprintblocks.SprintBlocksStatus, startedAt, completedAt *time.Time) error {
	if tx.parent.failOnBlock {
		return errors.New("simulated block update failure")
	}
	b, ok := tx.blocks[blockID]
	if !ok {
		return sprintblocks.ErrSprintBlockNotFound
	}
	b.Status = status
	if startedAt != nil && b.StartedAt == nil {
		b.StartedAt = startedAt
	}
	if completedAt != nil {
		b.CompletedAt = completedAt
	}
	tx.blocks[blockID] = b
	return nil
}

func (tx *inMemoryTx) UpdateTasksStatus(_ context.Context, blockID uuid.UUID, taskIDs []uuid.UUID, status blocktasks.BlockTaskStatus, completedAt *time.Time) error {
	for _, id := range taskIDs {
		task, ok := tx.tasks[id]
		if !ok || task.SprintBlockID != blockID {
			return blocktasks.ErrBlockTaskNotFound
		}
		task.Status = status
		if completedAt != nil {
			task.CompletedAt = completedAt
		}
		tx.tasks[id] = task
	}
	return nil
}

func (tx *inMemoryTx) LockCycleByID(_ context.Context, cycleID uuid.UUID) (*StudyCycle, error) {
	c, ok := tx.cycles[cycleID]
	if !ok {
		return nil, ErrStudyCycleNotFound
	}
	copied := c
	return &copied, nil
}

func (tx *inMemoryTx) GetCycleTaskIDs(_ context.Context, cycleID uuid.UUID) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	for _, ct := range tx.cycleTasks[cycleID] {
		ids = append(ids, ct.BlockTaskID)
	}
	return ids, nil
}

func (tx *inMemoryTx) CompleteCycleTasks(_ context.Context, cycleID uuid.UUID, completedAt time.Time) error {
	list := tx.cycleTasks[cycleID]
	for i := range list {
		ts := completedAt
		list[i].CompletedAt = &ts
	}
	tx.cycleTasks[cycleID] = list
	return nil
}

func (tx *inMemoryTx) StopCycleConditional(_ context.Context, input StopStudyCycleModel) (*StudyCycle, error) {
	c, ok := tx.cycles[input.ID]
	if !ok {
		return nil, ErrStudyCycleNotFound
	}
	if c.Status != input.ExpectedStatus {
		return nil, ErrStudyCycleAlreadyStopped
	}
	endedAt := input.EndedAt.UTC()
	reason := input.StopReason
	c.Status = input.Status
	c.RunningSince = nil
	c.EndedAt = &endedAt
	c.ElapsedSeconds = input.ElapsedSeconds
	c.StopReason = &reason
	c.UpdatedAt = &endedAt
	tx.cycles[input.ID] = c
	copied := c
	return &copied, nil
}

func TestConstraintTranslationAndMigrations(t *testing.T) {
	t.Run("translates active sprint and sequence unique constraint violations", func(t *testing.T) {
		for _, constraint := range []string{activeSprintCycleUniqueIndex, sprintSequenceUniqueIndex} {
			pgErr := &pgconn.PgError{
				Code:           studyCycleUniqueViolationCode,
				ConstraintName: constraint,
			}
			translated := translateStudyCycleConstraintError(pgErr, "create study cycle")
			if !errors.Is(translated, ErrStudyCycleAlreadyActive) {
				t.Fatalf("for constraint %q expected ErrStudyCycleAlreadyActive, got %v", constraint, translated)
			}
		}

		otherErr := errors.New("connection reset")
		translated := translateStudyCycleConstraintError(otherErr, "create study cycle")
		if !errors.Is(translated, otherErr) || errors.Is(translated, ErrStudyCycleAlreadyActive) {
			t.Fatalf("expected wrapped original error, got %v", translated)
		}
	})

	t.Run("verifies study cycle Goose migrations contain required constraints and indexes", func(t *testing.T) {
		baseContent, err := fs.ReadFile(migrations.Files, "20261024142902_add_study_cycles.sql")
		if err != nil {
			t.Fatalf("read base study cycles migration: %v", err)
		}
		evolveContent, err := fs.ReadFile(migrations.Files, "20261025120000_evolve_study_cycles_execution.sql")
		if err != nil {
			t.Fatalf("read evolve study cycles migration: %v", err)
		}

		combined := string(baseContent) + "\n" + string(evolveContent)
		requiredFragments := []string{
			"CHECK (sequence_number > 0)",
			"CHECK (elapsed_seconds >= 0)",
			"chk_study_cycles_planned_duration_positive",
			"planned_duration_seconds IS NULL",
			"planned_duration_seconds > 0",
			"chk_study_cycles_ended_at_after_started_at",
			"ended_at IS NULL",
			"ended_at >= started_at",
			"CREATE UNIQUE INDEX uq_study_cycles_active_sprint",
			"WHERE status IN ('em_andamento', 'pausado')",
			"CREATE UNIQUE INDEX uq_study_cycles_sprint_sequence",
			"CREATE TABLE study_cycle_tasks",
			"-- +goose Up",
			"-- +goose Down",
		}
		for _, fragment := range requiredFragments {
			if !strings.Contains(combined, fragment) {
				t.Fatalf("expected migration files to contain %q", fragment)
			}
		}
	})
}

func TestEndToEndLifecycleAndConcurrentTransitions(t *testing.T) {
	t.Run("full lifecycle: play -> pause -> resume -> stop with sequence allocation and rollback", func(t *testing.T) {
		repo := newInMemoryConcurrentRepo()
		sprintID := uuid.New()
		block1ID := uuid.New()
		block2ID := uuid.New()
		task1ID := uuid.New()
		task2ID := uuid.New()

		repo.sprints[sprintID] = sprint.Sprint{ID: sprintID, Status: sprint.InProgress}
		repo.blocks[block1ID] = sprintblocks.SprintBlock{ID: block1ID, SprintID: sprintID, Position: 1, Status: sprintblocks.Pending}
		repo.blocks[block2ID] = sprintblocks.SprintBlock{ID: block2ID, SprintID: sprintID, Position: 2, Status: sprintblocks.Pending}
		repo.tasks[task1ID] = blocktasks.BlockTask{ID: task1ID, SprintBlockID: block1ID, Position: 1, Status: blocktasks.TaskPending}
		repo.tasks[task2ID] = blocktasks.BlockTask{ID: task2ID, SprintBlockID: block1ID, Position: 2, Status: blocktasks.TaskPending}

		currentTime := time.Date(2026, 11, 4, 10, 0, 0, 0, time.UTC)
		service := NewStudyCycleServiceWithClock(repo, func() time.Time { return currentTime })

		// 1. Play block 1 with task 1
		cycle1, err := service.Play(context.Background(), defaultTestUserID, sprintID, PlayStudyCycleInput{
			ExecutionMode: ExecutionModeSingleTask,
			TimerType:     TimerTypeFree,
			BlockTaskIDs:  []uuid.UUID{task1ID},
		})
		if err != nil {
			t.Fatalf("play cycle 1 failed: %v", err)
		}
		if cycle1.SequenceNumber != 1 || cycle1.SprintBlockID != block1ID || cycle1.Status != StudyCycleInProgress {
			t.Fatalf("unexpected cycle 1: %#v", cycle1)
		}
		if repo.blocks[block1ID].Status != sprintblocks.InProgress {
			t.Fatalf("expected block 1 to be in progress, got %q", repo.blocks[block1ID].Status)
		}

		// 2. Pause after 120s
		currentTime = currentTime.Add(120 * time.Second)
		paused1, err := service.Pause(context.Background(), defaultTestUserID, cycle1.ID)
		if err != nil {
			t.Fatalf("pause cycle 1 failed: %v", err)
		}
		if paused1.Status != StudyCyclePaused || paused1.ElapsedSeconds != 120 || paused1.RunningSince != nil {
			t.Fatalf("unexpected paused cycle 1: %#v", paused1)
		}

		// 3. Resume after 300s pause (paused interval must not count)
		currentTime = currentTime.Add(300 * time.Second)
		resumed1, err := service.Resume(context.Background(), defaultTestUserID, cycle1.ID)
		if err != nil {
			t.Fatalf("resume cycle 1 failed: %v", err)
		}
		if resumed1.Status != StudyCycleInProgress || resumed1.ElapsedSeconds != 120 || !resumed1.StartedAt.Equal(cycle1.StartedAt) {
			t.Fatalf("unexpected resumed cycle 1: %#v", resumed1)
		}

		// 4. Stop completed after 80s more running (total elapsed = 200s)
		currentTime = currentTime.Add(80 * time.Second)
		stopped1, err := service.Stop(context.Background(), defaultTestUserID, cycle1.ID, StopStudyCycleInput{Reason: CycleStopReasonCompleted})
		if err != nil {
			t.Fatalf("stop cycle 1 failed: %v", err)
		}
		if stopped1.Status != StudyCycleFinished || stopped1.ElapsedSeconds != 200 {
			t.Fatalf("expected elapsed 200 and finished status, got %#v", stopped1)
		}
		if repo.tasks[task1ID].Status != blocktasks.TaskDone {
			t.Fatalf("expected task 1 to be done, got %q", repo.tasks[task1ID].Status)
		}
		if repo.blocks[block1ID].Status != sprintblocks.InProgress {
			t.Fatalf("expected block 1 to stay in progress while task 2 is pending, got %q", repo.blocks[block1ID].Status)
		}

		// 5. Simulate rollback on Play when block update fails
		repo.failOnBlock = true
		block1Temp := repo.blocks[block1ID]
		block1Temp.Status = sprintblocks.Pending
		repo.blocks[block1ID] = block1Temp
		if _, err := service.Play(context.Background(), defaultTestUserID, sprintID, PlayStudyCycleInput{
			ExecutionMode: ExecutionModeSingleTask,
			TimerType:     TimerTypeFree,
			BlockTaskIDs:  []uuid.UUID{task2ID},
		}); err == nil {
			t.Fatal("expected play to fail when block update fails")
		}
		if len(repo.cycles) != 1 {
			t.Fatalf("expected failed play transaction to roll back cycle insertion, got %d cycles", len(repo.cycles))
		}
		repo.failOnBlock = false

		// 6. Play cycle 2 on block 1 with task 2 and complete it -> block 1 finishes
		currentTime = currentTime.Add(10 * time.Second)
		cycle2, err := service.Play(context.Background(), defaultTestUserID, sprintID, PlayStudyCycleInput{
			ExecutionMode: ExecutionModeSingleTask,
			TimerType:     TimerTypeFree,
			BlockTaskIDs:  []uuid.UUID{task2ID},
		})
		if err != nil {
			t.Fatalf("play cycle 2 failed: %v", err)
		}
		if cycle2.SequenceNumber != 2 || cycle2.SprintBlockID != block1ID {
			t.Fatalf("expected sequence 2 on block 1, got %#v", cycle2)
		}
		currentTime = currentTime.Add(60 * time.Second)
		if _, err := service.Stop(context.Background(), defaultTestUserID, cycle2.ID, StopStudyCycleInput{Reason: CycleStopReasonCompleted}); err != nil {
			t.Fatalf("stop cycle 2 failed: %v", err)
		}
		if repo.blocks[block1ID].Status != sprintblocks.Finished {
			t.Fatalf("expected block 1 to be finished after all tasks done, got %q", repo.blocks[block1ID].Status)
		}

		// 7. Next Play selects block 2 with sequence 3
		currentTime = currentTime.Add(10 * time.Second)
		cycle3, err := service.Play(context.Background(), defaultTestUserID, sprintID, PlayStudyCycleInput{
			ExecutionMode: ExecutionModeSubjectBlock,
			TimerType:     TimerTypeFree,
		})
		if err != nil {
			t.Fatalf("play cycle 3 failed: %v", err)
		}
		if cycle3.SequenceNumber != 3 || cycle3.SprintBlockID != block2ID {
			t.Fatalf("expected sequence 3 on block 2, got %#v", cycle3)
		}
	})

	t.Run("two concurrent play requests for the same sprint allow exactly one to succeed", func(t *testing.T) {
		repo := newInMemoryConcurrentRepo()
		sprintID := uuid.New()
		blockID := uuid.New()
		repo.sprints[sprintID] = sprint.Sprint{ID: sprintID, UserID: defaultTestUserID, Status: sprint.InProgress}
		repo.blocks[blockID] = sprintblocks.SprintBlock{ID: blockID, SprintID: sprintID, Position: 1, Status: sprintblocks.Pending}

		fixedNow := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
		service := NewStudyCycleServiceWithClock(repo, func() time.Time { return fixedNow })

		start := make(chan struct{})
		var wg sync.WaitGroup
		errs := make([]error, 2)

		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				<-start
				_, errs[idx] = service.Play(context.Background(), defaultTestUserID, sprintID, PlayStudyCycleInput{
					ExecutionMode: ExecutionModeSubjectBlock,
					TimerType:     TimerTypeFree,
				})
			}(i)
		}

		close(start)
		wg.Wait()

		successCount := 0
		conflictCount := 0
		for _, err := range errs {
			switch {
			case err == nil:
				successCount++
			case errors.Is(err, ErrStudyCycleAlreadyActive):
				conflictCount++
			default:
				t.Fatalf("unexpected error in concurrent play: %v", err)
			}
		}

		if successCount != 1 || conflictCount != 1 {
			t.Fatalf("expected 1 success and 1 ErrStudyCycleAlreadyActive, got success=%d conflict=%d errs=%v", successCount, conflictCount, errs)
		}
	})

	t.Run("two concurrent pause requests for the same cycle allow exactly one to succeed", func(t *testing.T) {
		repo := newInMemoryConcurrentRepo()
		sprintID := uuid.New()
		blockID := uuid.New()
		repo.sprints[sprintID] = sprint.Sprint{ID: sprintID, UserID: defaultTestUserID, Status: sprint.InProgress}
		repo.blocks[blockID] = sprintblocks.SprintBlock{ID: blockID, SprintID: sprintID, Position: 1, Status: sprintblocks.Pending}

		t0 := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
		service := NewStudyCycleServiceWithClock(repo, func() time.Time { return t0 })
		cycle, err := service.Play(context.Background(), defaultTestUserID, sprintID, PlayStudyCycleInput{
			ExecutionMode: ExecutionModeSubjectBlock,
			TimerType:     TimerTypeFree,
		})
		if err != nil {
			t.Fatalf("play failed: %v", err)
		}

		pauseService := NewStudyCycleServiceWithClock(repo, func() time.Time { return t0.Add(45 * time.Second) })
		start := make(chan struct{})
		var wg sync.WaitGroup
		errs := make([]error, 2)

		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				<-start
				_, errs[idx] = pauseService.Pause(context.Background(), defaultTestUserID, cycle.ID)
			}(i)
		}

		close(start)
		wg.Wait()

		successCount := 0
		conflictCount := 0
		for _, err := range errs {
			switch {
			case err == nil:
				successCount++
			case errors.Is(err, ErrStudyCycleNotRunning) && errors.Is(err, ErrStudyCycleInvalidTransition):
				conflictCount++
			default:
				t.Fatalf("unexpected error in concurrent pause: %v", err)
			}
		}

		if successCount != 1 || conflictCount != 1 {
			t.Fatalf("expected 1 success and 1 ErrStudyCycleNotRunning, got success=%d conflict=%d errs=%v", successCount, conflictCount, errs)
		}
	})
}

func openIntegrationDB(t *testing.T) *sqlx.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL or DATABASE_URL not set; skipping PostgreSQL integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	db, err := database.ConnectDatabase(ctx, database.Config{
		URL:             dsn,
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
		ConnMaxIdleTime: 1 * time.Minute,
	})
	if err != nil {
		t.Skipf("PostgreSQL database not reachable (%v); skipping PostgreSQL integration test", err)
	}

	if err := database.RunMigrations(db, migrations.Files, "."); err != nil {
		_ = db.Close()
		t.Fatalf("run migrations: %v", err)
	}

	t.Cleanup(func() {
		_ = db.Close()
	})
	return db
}

func TestStudyCycleRepositoryPostgresIntegration(t *testing.T) {
	db := openIntegrationDB(t)
	ctx := context.Background()
	repo := NewStudyCycleRepository(db)

	t0 := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	currentNow := t0
	service := NewStudyCycleServiceWithClock(repo, func() time.Time { return currentNow })

	var testUserID uuid.UUID
	testUserEmail := "test-" + uuid.NewString() + "@example.com"
	if err := db.GetContext(ctx, &testUserID, `INSERT INTO users (name, email, password_hash) VALUES ('Test User', $1, 'hash') RETURNING id`, testUserEmail); err != nil {
		t.Fatalf("insert test user: %v", err)
	}

	var subjectID uuid.UUID
	subjectName := "Subject-" + uuid.NewString()
	if err := db.GetContext(ctx, &subjectID, `INSERT INTO subjects (user_id, name, color) VALUES ($1, $2, 'blue') RETURNING id`, testUserID, subjectName); err != nil {
		t.Fatalf("insert test subject: %v", err)
	}

	var sprintID uuid.UUID
	sprintDate := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, int(uuid.New().ID()%10000))
	if err := db.GetContext(ctx, &sprintID, `INSERT INTO sprints (user_id, name, sprint_date, status, started_at) VALUES ($1, 'Test Sprint', $2::date, 'in_progress', $3) RETURNING id`, testUserID, sprintDate, t0); err != nil {
		t.Fatalf("insert test sprint: %v", err)
	}

	var blockID uuid.UUID
	if err := db.GetContext(ctx, &blockID, `INSERT INTO sprint_block (sprint_id, subject_id, title, position, status) VALUES ($1, $2, 'Block 1', 1, 'pendente') RETURNING id`, sprintID, subjectID); err != nil {
		t.Fatalf("insert test block: %v", err)
	}

	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM study_cycles WHERE sprint_id = $1`, sprintID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM sprints WHERE id = $1`, sprintID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM subjects WHERE id = $1`, subjectID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM users WHERE id = $1`, testUserID)
	})

	t.Run("concurrent play requests in PostgreSQL allow exactly one active cycle", func(t *testing.T) {
		start := make(chan struct{})
		var wg sync.WaitGroup
		errs := make([]error, 2)
		cycles := make([]StudyCycle, 2)

		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				<-start
				cycles[idx], errs[idx] = service.Play(ctx, testUserID, sprintID, PlayStudyCycleInput{
					ExecutionMode: ExecutionModeSubjectBlock,
					TimerType:     TimerTypeFree,
				})
			}(i)
		}
		close(start)
		wg.Wait()

		var activeCycle StudyCycle
		success := 0
		conflicts := 0
		for i, err := range errs {
			switch {
			case err == nil:
				success++
				activeCycle = cycles[i]
			case errors.Is(err, ErrStudyCycleAlreadyActive):
				conflicts++
			default:
				t.Fatalf("unexpected play error: %v", err)
			}
		}
		if success != 1 || conflicts != 1 {
			t.Fatalf("expected 1 success and 1 conflict, got success=%d conflicts=%d", success, conflicts)
		}

		// Concurrent pause requests on the created cycle
		currentNow = t0.Add(60 * time.Second)
		startPause := make(chan struct{})
		pauseErrs := make([]error, 2)
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				<-startPause
				_, pauseErrs[idx] = service.Pause(ctx, testUserID, activeCycle.ID)
			}(i)
		}
		close(startPause)
		wg.Wait()

		pauseSuccess := 0
		pauseConflict := 0
		for _, err := range pauseErrs {
			switch {
			case err == nil:
				pauseSuccess++
			case errors.Is(err, ErrStudyCycleNotRunning):
				pauseConflict++
			default:
				t.Fatalf("unexpected pause error: %v", err)
			}
		}
		if pauseSuccess != 1 || pauseConflict != 1 {
			t.Fatalf("expected 1 pause success and 1 conflict, got success=%d conflict=%d", pauseSuccess, pauseConflict)
		}

		// Resume and Stop
		currentNow = t0.Add(120 * time.Second)
		resumed, err := service.Resume(ctx, testUserID, activeCycle.ID)
		if err != nil || resumed.ElapsedSeconds != 60 {
			t.Fatalf("unexpected resume result: %#v %v", resumed, err)
		}

		currentNow = t0.Add(150 * time.Second)
		stopped, err := service.Stop(ctx, testUserID, activeCycle.ID, StopStudyCycleInput{Reason: CycleStopReasonCompleted})
		if err != nil || stopped.ElapsedSeconds != 90 || stopped.Status != StudyCycleFinished {
			t.Fatalf("unexpected stop result: %#v %v", stopped, err)
		}
	})
}
