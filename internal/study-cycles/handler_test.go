package studycycles

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Lucasmenezes08/kotoro-api.git/internal/auth"
	blocktasks "github.com/Lucasmenezes08/kotoro-api.git/internal/block-tasks"
	"github.com/Lucasmenezes08/kotoro-api.git/internal/sprint"
	sprintblocks "github.com/Lucasmenezes08/kotoro-api.git/internal/sprint-blocks"
	"github.com/google/uuid"
)

type fakeStudyCycleService struct {
	playFn      func(ctx context.Context, userID, sprintID uuid.UUID, input PlayStudyCycleInput) (StudyCycle, error)
	pauseFn     func(ctx context.Context, userID, cycleID uuid.UUID) (StudyCycle, error)
	resumeFn    func(ctx context.Context, userID, cycleID uuid.UUID) (StudyCycle, error)
	stopFn      func(ctx context.Context, userID, cycleID uuid.UUID, input StopStudyCycleInput) (StudyCycle, error)
	getActiveFn func(ctx context.Context, userID, sprintID uuid.UUID) (*StudyCycle, error)
	getAllFn    func(ctx context.Context, userID, sprintID uuid.UUID) ([]StudyCycle, error)
}

func (f *fakeStudyCycleService) Play(ctx context.Context, userID, sprintID uuid.UUID, input PlayStudyCycleInput) (StudyCycle, error) {
	if f.playFn == nil {
		panic("unexpected call to Play")
	}
	return f.playFn(ctx, userID, sprintID, input)
}

func (f *fakeStudyCycleService) Pause(ctx context.Context, userID, cycleID uuid.UUID) (StudyCycle, error) {
	if f.pauseFn == nil {
		panic("unexpected call to Pause")
	}
	return f.pauseFn(ctx, userID, cycleID)
}

func (f *fakeStudyCycleService) Resume(ctx context.Context, userID, cycleID uuid.UUID) (StudyCycle, error) {
	if f.resumeFn == nil {
		panic("unexpected call to Resume")
	}
	return f.resumeFn(ctx, userID, cycleID)
}

func (f *fakeStudyCycleService) Stop(ctx context.Context, userID, cycleID uuid.UUID, input StopStudyCycleInput) (StudyCycle, error) {
	if f.stopFn == nil {
		panic("unexpected call to Stop")
	}
	return f.stopFn(ctx, userID, cycleID, input)
}

func (f *fakeStudyCycleService) GetActive(ctx context.Context, userID, sprintID uuid.UUID) (*StudyCycle, error) {
	if f.getActiveFn != nil {
		return f.getActiveFn(ctx, userID, sprintID)
	}
	return nil, nil
}

func (f *fakeStudyCycleService) GetAll(ctx context.Context, userID, sprintID uuid.UUID) ([]StudyCycle, error) {
	if f.getAllFn != nil {
		return f.getAllFn(ctx, userID, sprintID)
	}
	return []StudyCycle{}, nil
}

var _ StudyCycleServiceContract = (*fakeStudyCycleService)(nil)

func authReq(r *http.Request) *http.Request {
	return r.WithContext(auth.WithUserID(r.Context(), defaultTestUserID))
}

func TestStudyCycleHandlerSuccessStatusAndResponseBody(t *testing.T) {
	serverNow := time.Date(2026, 11, 4, 15, 0, 0, 0, time.UTC)
	sprintID := uuid.New()
	blockID := uuid.New()
	cycleID := uuid.New()
	taskID := uuid.New()
	plannedDuration := 1500

	t.Run("Play returns 201 Created and authoritative cycle state", func(t *testing.T) {
		service := &fakeStudyCycleService{
			playFn: func(_ context.Context, gotUser, gotSprint uuid.UUID, input PlayStudyCycleInput) (StudyCycle, error) {
				if gotUser != defaultTestUserID || gotSprint != sprintID || input.ExecutionMode != ExecutionModeSingleTask || input.TimerType != TimerTypeTimed {
					t.Fatalf("unexpected play input: sprint=%v input=%#v", gotSprint, input)
				}
				if input.PlannedDurationSeconds == nil || *input.PlannedDurationSeconds != plannedDuration {
					t.Fatalf("unexpected duration: %v", input.PlannedDurationSeconds)
				}
				if len(input.BlockTaskIDs) != 1 || input.BlockTaskIDs[0] != taskID {
					t.Fatalf("unexpected task ids: %v", input.BlockTaskIDs)
				}
				return StudyCycle{
					ID:                     cycleID,
					SprintID:               sprintID,
					SprintBlockID:          blockID,
					SequenceNumber:         1,
					ExecutionMode:          input.ExecutionMode,
					TimerType:              input.TimerType,
					Status:                 StudyCycleInProgress,
					PlannedDurationSeconds: input.PlannedDurationSeconds,
					StartedAt:              serverNow,
					RunningSince:           &serverNow,
					ElapsedSeconds:         0,
				}, nil
			},
		}

		handler := NewStudyCycleHandlerWithClock(service, func() time.Time { return serverNow })
		body := `{"execution_mode":"tarefa_individual","timer_type":"temporizado","planned_duration_seconds":1500,"block_task_ids":["` + taskID.String() + `"]}`
		req := authReq(httptest.NewRequest(http.MethodPost, "/sprints/"+sprintID.String()+"/study-cycles/play", strings.NewReader(body)))
		req.SetPathValue("sprintID", sprintID.String())
		rec := httptest.NewRecorder()

		handler.Play(rec, req)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
		}

		var resp StudyCycleResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if resp.ID != cycleID || resp.SprintID != sprintID || resp.SprintBlockID != blockID || resp.SequenceNumber != 1 || resp.Status != StudyCycleInProgress || !resp.ServerNow.Equal(serverNow) {
			t.Fatalf("unexpected response payload: %#v", resp)
		}
	})

	t.Run("Pause returns 200 OK", func(t *testing.T) {
		pausedAt := serverNow.Add(5 * time.Minute)
		service := &fakeStudyCycleService{
			pauseFn: func(_ context.Context, gotUser, gotCycle uuid.UUID) (StudyCycle, error) {
				if gotUser != defaultTestUserID || gotCycle != cycleID {
					t.Fatalf("expected cycle %v, got %v", cycleID, gotCycle)
				}
				return StudyCycle{
					ID:             cycleID,
					SprintID:       sprintID,
					SprintBlockID:  blockID,
					SequenceNumber: 1,
					ExecutionMode:  ExecutionModeSubjectBlock,
					TimerType:      TimerTypeFree,
					Status:         StudyCyclePaused,
					StartedAt:      serverNow,
					PausedAt:       &pausedAt,
					ElapsedSeconds: 300,
				}, nil
			},
		}

		handler := NewStudyCycleHandlerWithClock(service, func() time.Time { return pausedAt })
		req := authReq(httptest.NewRequest(http.MethodPost, "/study-cycles/"+cycleID.String()+"/pause", nil))
		req.SetPathValue("cycleID", cycleID.String())
		rec := httptest.NewRecorder()

		handler.Pause(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		var resp StudyCycleResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if resp.Status != StudyCyclePaused || resp.ElapsedSeconds != 300 || resp.RunningSince != nil || resp.PausedAt == nil || !resp.PausedAt.Equal(pausedAt) {
			t.Fatalf("unexpected pause response: %#v", resp)
		}
	})

	t.Run("Resume returns 200 OK", func(t *testing.T) {
		resumedAt := serverNow.Add(10 * time.Minute)
		service := &fakeStudyCycleService{
			resumeFn: func(_ context.Context, gotUser, gotCycle uuid.UUID) (StudyCycle, error) {
				if gotUser != defaultTestUserID || gotCycle != cycleID {
					t.Fatalf("expected cycle %v, got %v", cycleID, gotCycle)
				}
				return StudyCycle{
					ID:             cycleID,
					SprintID:       sprintID,
					SprintBlockID:  blockID,
					SequenceNumber: 1,
					ExecutionMode:  ExecutionModeSubjectBlock,
					TimerType:      TimerTypeFree,
					Status:         StudyCycleInProgress,
					StartedAt:      serverNow,
					RunningSince:   &resumedAt,
					ElapsedSeconds: 300,
				}, nil
			},
		}

		handler := NewStudyCycleHandlerWithClock(service, func() time.Time { return resumedAt })
		req := authReq(httptest.NewRequest(http.MethodPost, "/study-cycles/"+cycleID.String()+"/resume", nil))
		req.SetPathValue("cycleID", cycleID.String())
		rec := httptest.NewRecorder()

		handler.Resume(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		var resp StudyCycleResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if resp.Status != StudyCycleInProgress || resp.ElapsedSeconds != 300 || resp.RunningSince == nil || !resp.RunningSince.Equal(resumedAt) {
			t.Fatalf("unexpected resume response: %#v", resp)
		}
	})

	t.Run("Stop returns 200 OK", func(t *testing.T) {
		endedAt := serverNow.Add(15 * time.Minute)
		reason := CycleStopReasonCompleted
		service := &fakeStudyCycleService{
			stopFn: func(_ context.Context, gotUser, gotCycle uuid.UUID, input StopStudyCycleInput) (StudyCycle, error) {
				if gotUser != defaultTestUserID || gotCycle != cycleID || input.Reason != CycleStopReasonCompleted {
					t.Fatalf("unexpected stop args: %v %#v", gotCycle, input)
				}
				return StudyCycle{
					ID:             cycleID,
					SprintID:       sprintID,
					SprintBlockID:  blockID,
					SequenceNumber: 1,
					ExecutionMode:  ExecutionModeSubjectBlock,
					TimerType:      TimerTypeFree,
					Status:         StudyCycleFinished,
					StartedAt:      serverNow,
					EndedAt:        &endedAt,
					ElapsedSeconds: 600,
					StopReason:     &reason,
				}, nil
			},
		}

		handler := NewStudyCycleHandlerWithClock(service, func() time.Time { return endedAt })
		req := authReq(httptest.NewRequest(http.MethodPost, "/study-cycles/"+cycleID.String()+"/stop", strings.NewReader(`{"reason":"concluido"}`)))
		req.SetPathValue("cycleID", cycleID.String())
		rec := httptest.NewRecorder()

		handler.Stop(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		var resp StudyCycleResponse
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if resp.Status != StudyCycleFinished || resp.ElapsedSeconds != 600 || resp.StopReason == nil || *resp.StopReason != CycleStopReasonCompleted {
			t.Fatalf("unexpected stop response: %#v", resp)
		}
	})
}

func TestStudyCycleHandlerInvalidUUIDs(t *testing.T) {
	handler := NewStudyCycleHandler(&fakeStudyCycleService{})

	t.Run("invalid sprintID on Play", func(t *testing.T) {
		req := authReq(httptest.NewRequest(http.MethodPost, "/sprints/invalid/study-cycles/play", strings.NewReader(`{"execution_mode":"bloco_materia","timer_type":"cronometro_livre"}`)))
		req.SetPathValue("sprintID", "invalid")
		rec := httptest.NewRecorder()
		handler.Play(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("invalid block_task_id on Play", func(t *testing.T) {
		sprintID := uuid.New()
		req := authReq(httptest.NewRequest(http.MethodPost, "/sprints/"+sprintID.String()+"/study-cycles/play", strings.NewReader(`{"execution_mode":"tarefa_individual","timer_type":"cronometro_livre","block_task_ids":["not-a-uuid"]}`)))
		req.SetPathValue("sprintID", sprintID.String())
		rec := httptest.NewRecorder()
		handler.Play(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	for _, tc := range []struct {
		name string
		call func(http.ResponseWriter, *http.Request)
	}{
		{"Pause", handler.Pause},
		{"Resume", handler.Resume},
		{"Stop", handler.Stop},
	} {
		t.Run("invalid cycleID on "+tc.name, func(t *testing.T) {
			req := authReq(httptest.NewRequest(http.MethodPost, "/study-cycles/invalid", strings.NewReader(`{"reason":"concluido"}`)))
			req.SetPathValue("cycleID", "invalid")
			rec := httptest.NewRecorder()
			tc.call(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d", rec.Code)
			}
		})
	}
}

func TestStudyCycleHandlerInvalidJSONAndUnknownFields(t *testing.T) {
	handler := NewStudyCycleHandler(&fakeStudyCycleService{})
	sprintID := uuid.New()
	cycleID := uuid.New()

	playBodies := []string{
		`{`,
		`{"execution_mode":"bloco_materia","timer_type":"cronometro_livre","status":"em_andamento"}`,
		`{"execution_mode":"bloco_materia","timer_type":"cronometro_livre","sequence_number":10}`,
		`{"execution_mode":"bloco_materia","timer_type":"cronometro_livre","started_at":"2026-11-04T00:00:00Z"}`,
		`{"execution_mode":"bloco_materia","timer_type":"cronometro_livre","running_since":"2026-11-04T00:00:00Z"}`,
		`{"execution_mode":"bloco_materia","timer_type":"cronometro_livre","elapsed_seconds":100}`,
		`{"execution_mode":"bloco_materia","timer_type":"cronometro_livre","ended_at":"2026-11-04T00:00:00Z"}`,
		`{"execution_mode":"bloco_materia","timer_type":"cronometro_livre","stop_reason":"concluido"}`,
	}

	for _, body := range playBodies {
		req := authReq(httptest.NewRequest(http.MethodPost, "/sprints/"+sprintID.String()+"/study-cycles/play", strings.NewReader(body)))
		req.SetPathValue("sprintID", sprintID.String())
		rec := httptest.NewRecorder()
		handler.Play(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for Play body %q, got %d", body, rec.Code)
		}
	}

	stopBodies := []string{
		`{`,
		`{"reason":"concluido","status":"finalizado"}`,
		`{"reason":"concluido","elapsed_seconds":500}`,
	}
	for _, body := range stopBodies {
		req := authReq(httptest.NewRequest(http.MethodPost, "/study-cycles/"+cycleID.String()+"/stop", strings.NewReader(body)))
		req.SetPathValue("cycleID", cycleID.String())
		rec := httptest.NewRecorder()
		handler.Stop(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for Stop body %q, got %d", body, rec.Code)
		}
	}

	for _, body := range []string{`{`, `{"status":"pausado"}`} {
		req := authReq(httptest.NewRequest(http.MethodPost, "/study-cycles/"+cycleID.String()+"/pause", strings.NewReader(body)))
		req.SetPathValue("cycleID", cycleID.String())
		rec := httptest.NewRecorder()
		handler.Pause(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for Pause body %q, got %d", body, rec.Code)
		}
	}
}

func TestStudyCycleHandlerErrorMappings(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{"invalid execution mode", ErrInvalidExecutionMode, http.StatusBadRequest},
		{"invalid timer configuration", ErrInvalidTimerConfiguration, http.StatusBadRequest},
		{"invalid selected tasks", ErrInvalidSelectedTasks, http.StatusBadRequest},
		{"invalid stop reason", ErrInvalidStopReason, http.StatusBadRequest},
		{"sprint not found", sprint.ErrSprintNotFound, http.StatusNotFound},
		{"block not found", sprintblocks.ErrSprintBlockNotFound, http.StatusNotFound},
		{"task not found", blocktasks.ErrBlockTaskNotFound, http.StatusNotFound},
		{"cycle not found", ErrStudyCycleNotFound, http.StatusNotFound},
		{"already active cycle", ErrStudyCycleAlreadyActive, http.StatusConflict},
		{"cycle not running", ErrStudyCycleNotRunning, http.StatusConflict},
		{"cycle not paused", ErrStudyCycleNotPaused, http.StatusConflict},
		{"cycle already stopped", ErrStudyCycleAlreadyStopped, http.StatusConflict},
		{"invalid transition", ErrStudyCycleInvalidTransition, http.StatusConflict},
		{"sprint not started", ErrSprintNotStarted, http.StatusConflict},
		{"sprint has no available block", ErrSprintHasNoAvailableBlock, http.StatusConflict},
		{"unexpected infrastructure error", errors.New("pq: fatal database error"), http.StatusInternalServerError},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			service := &fakeStudyCycleService{
				stopFn: func(context.Context, uuid.UUID, uuid.UUID, StopStudyCycleInput) (StudyCycle, error) {
					return StudyCycle{}, tc.err
				},
			}
			handler := NewStudyCycleHandler(service)
			cycleID := uuid.New()
			req := authReq(httptest.NewRequest(http.MethodPost, "/study-cycles/"+cycleID.String()+"/stop", strings.NewReader(`{"reason":"concluido"}`)))
			req.SetPathValue("cycleID", cycleID.String())
			rec := httptest.NewRecorder()

			handler.Stop(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("expected status %d, got %d: %s", tc.wantStatus, rec.Code, rec.Body.String())
			}
			if tc.wantStatus == http.StatusInternalServerError && strings.Contains(rec.Body.String(), "pq: fatal") {
				t.Fatal("internal database error leaked to client")
			}
		})
	}

	t.Run("returns 401 when unauthenticated", func(t *testing.T) {
		handler := NewStudyCycleHandler(&fakeStudyCycleService{})
		cycleID := uuid.New()
		req := httptest.NewRequest(http.MethodPost, "/study-cycles/"+cycleID.String()+"/stop", strings.NewReader(`{"reason":"concluido"}`))
		req.SetPathValue("cycleID", cycleID.String())
		rec := httptest.NewRecorder()
		handler.Stop(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("GetActive returns 200 OK with cycle when active cycle exists", func(t *testing.T) {
		sprintID := uuid.New()
		cycleID := uuid.New()
		service := &fakeStudyCycleService{
			getActiveFn: func(ctx context.Context, userID, sid uuid.UUID) (*StudyCycle, error) {
				return &StudyCycle{
					ID:             cycleID,
					SprintID:       sid,
					Status:         StudyCycleInProgress,
					ElapsedSeconds: 42,
				}, nil
			},
		}
		handler := NewStudyCycleHandler(service)
		req := authReq(httptest.NewRequest(http.MethodGet, "/sprints/"+sprintID.String()+"/study-cycles/active", nil))
		req.SetPathValue("sprintID", sprintID.String())
		rec := httptest.NewRecorder()

		handler.GetActive(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		var resp StudyCycleResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if resp.ID != cycleID || resp.ElapsedSeconds != 42 {
			t.Fatalf("unexpected cycle response: %#v", resp)
		}
	})

	t.Run("GetActive returns 204 No Content when no active cycle exists", func(t *testing.T) {
		sprintID := uuid.New()
		service := &fakeStudyCycleService{
			getActiveFn: func(ctx context.Context, userID, sid uuid.UUID) (*StudyCycle, error) {
				return nil, nil
			},
		}
		handler := NewStudyCycleHandler(service)
		req := authReq(httptest.NewRequest(http.MethodGet, "/sprints/"+sprintID.String()+"/study-cycles/active", nil))
		req.SetPathValue("sprintID", sprintID.String())
		rec := httptest.NewRecorder()

		handler.GetActive(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Fatalf("expected 204, got %d", rec.Code)
		}
	})

	t.Run("GetAll returns 200 OK with slice of cycles", func(t *testing.T) {
		sprintID := uuid.New()
		service := &fakeStudyCycleService{
			getAllFn: func(ctx context.Context, userID, sid uuid.UUID) ([]StudyCycle, error) {
				return []StudyCycle{
					{ID: uuid.New(), SprintID: sid, SequenceNumber: 1, Status: StudyCycleFinished},
					{ID: uuid.New(), SprintID: sid, SequenceNumber: 2, Status: StudyCycleInProgress},
				}, nil
			},
		}
		handler := NewStudyCycleHandler(service)
		req := authReq(httptest.NewRequest(http.MethodGet, "/sprints/"+sprintID.String()+"/study-cycles", nil))
		req.SetPathValue("sprintID", sprintID.String())
		rec := httptest.NewRecorder()

		handler.GetAll(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		var resp []StudyCycleResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if len(resp) != 2 {
			t.Fatalf("expected 2 cycles, got %d", len(resp))
		}
	})
}
