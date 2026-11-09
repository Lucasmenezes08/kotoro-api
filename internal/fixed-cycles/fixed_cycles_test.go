package fixedcycles

import (
	"context"
	"testing"

	"github.com/Lucasmenezes08/kotoro-api.git/internal/subject"
	"github.com/google/uuid"
)

type mockSubjectReader struct {
	subject *subject.Subject
	err     error
}

func (m *mockSubjectReader) GetById(ctx context.Context, id uuid.UUID) (*subject.Subject, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.subject, nil
}

type mockRepository struct {
	created  *FixedCycle
	getAll   []FixedCycle
	getByID  *FixedCycle
	updated  *FixedCycle
	err      error
}

func (m *mockRepository) Create(ctx context.Context, userID uuid.UUID, input CreateFixedCycleInput) (*FixedCycle, error) {
	if m.err != nil {
		return nil, m.err
	}
	m.created = &FixedCycle{
		ID:               uuid.New(),
		UserID:           userID,
		SubjectID:        input.SubjectID,
		Title:            input.Title,
		EstimatedSeconds: input.EstimatedSeconds,
		DayOfWeek:        input.DayOfWeek,
		Tasks:            StringList(input.Tasks),
	}
	return m.created, nil
}

func (m *mockRepository) GetAll(ctx context.Context, userID uuid.UUID, dayOfWeek *string) ([]FixedCycle, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.getAll, nil
}

func (m *mockRepository) GetByID(ctx context.Context, userID, id uuid.UUID) (*FixedCycle, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.getByID != nil {
		return m.getByID, nil
	}
	return nil, ErrFixedCycleNotFound
}

func (m *mockRepository) Update(ctx context.Context, userID, id uuid.UUID, input UpdateFixedCycleInput) (*FixedCycle, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.updated, nil
}

func (m *mockRepository) Delete(ctx context.Context, userID, id uuid.UUID) error {
	return m.err
}

func TestNormalizeDayOfWeek(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		wantErr  bool
	}{
		{"", "all", false},
		{"all", "all", false},
		{"todos", "all", false},
		{"segunda", "segunda", false},
		{"segunda-feira", "segunda", false},
		{"monday", "segunda", false},
		{"1", "segunda", false},
		{"terca", "terca", false},
		{"terça", "terca", false},
		{"quarta", "quarta", false},
		{"quinta", "quinta", false},
		{"sexta", "sexta", false},
		{"sabado", "sabado", false},
		{"sábado", "sabado", false},
		{"domingo", "domingo", false},
		{"sunday", "domingo", false},
		{"invalid-day", "", true},
	}

	for _, tt := range tests {
		got, err := normalizeDayOfWeek(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("normalizeDayOfWeek(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if got != tt.expected {
			t.Errorf("normalizeDayOfWeek(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestService_Create(t *testing.T) {
	userID := uuid.New()
	subID := uuid.New()

	mockSub := &mockSubjectReader{
		subject: &subject.Subject{
			Id:     subID,
			UserId: userID,
			Name:   "Matemática",
		},
	}
	mockRepo := &mockRepository{}
	svc := NewService(mockRepo, mockSub)

	t.Run("success", func(t *testing.T) {
		res, err := svc.Create(context.Background(), userID, CreateFixedCycleInput{
			SubjectID:        subID,
			Title:            "Ciclo diário",
			EstimatedSeconds: 3600,
			DayOfWeek:        "segunda",
			Tasks:            []string{"Exercício 1", "Exercício 2"},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Title != "Ciclo diário" {
			t.Errorf("expected title 'Ciclo diário', got %q", res.Title)
		}
		if res.DayOfWeek != "segunda" {
			t.Errorf("expected day_of_week 'segunda', got %q", res.DayOfWeek)
		}
		if len(res.Tasks) != 2 {
			t.Errorf("expected 2 tasks, got %d", len(res.Tasks))
		}
	})

	t.Run("empty title", func(t *testing.T) {
		_, err := svc.Create(context.Background(), userID, CreateFixedCycleInput{
			SubjectID: subID,
			Title:     "   ",
		})
		if err != ErrFixedCycleTitleRequired {
			t.Errorf("expected ErrFixedCycleTitleRequired, got %v", err)
		}
	})

	t.Run("subject not owned by user", func(t *testing.T) {
		otherUserSub := &mockSubjectReader{
			subject: &subject.Subject{
				Id:     subID,
				UserId: uuid.New(), // different user
			},
		}
		svcOther := NewService(mockRepo, otherUserSub)
		_, err := svcOther.Create(context.Background(), userID, CreateFixedCycleInput{
			SubjectID: subID,
			Title:     "Ciclo",
		})
		if err != ErrFixedCycleInvalidSubject {
			t.Errorf("expected ErrFixedCycleInvalidSubject, got %v", err)
		}
	})
}
