package analytics

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

type mockAnalyticsRepo struct {
	subjectTimes []SubjectStudyTime
	streak       *StreakResponse
}

func (m *mockAnalyticsRepo) GetSubjectStudyTimes(ctx context.Context, userID uuid.UUID, timeframe string, refDate time.Time) ([]SubjectStudyTime, error) {
	return m.subjectTimes, nil
}

func (m *mockAnalyticsRepo) GetStreak(ctx context.Context, userID uuid.UUID, refDate time.Time) (*StreakResponse, error) {
	return m.streak, nil
}

func TestAnalyticsService(t *testing.T) {
	repo := &mockAnalyticsRepo{
		subjectTimes: []SubjectStudyTime{
			{
				SubjectID:    uuid.New(),
				SubjectName:  "Golang",
				SubjectColor: "ocean",
				TotalSeconds: 3600,
				Percentage:   100,
			},
		},
		streak: &StreakResponse{
			CurrentStreak:  2,
			LongestStreak:  4,
			TodayCompleted: true,
			Level:          "faisca",
			WeekDays: []StreakWeekDay{
				{Date: "2026-11-08", DayShort: "D", DayNumber: 8, IsCompleted: true, IsToday: true},
			},
		},
	}

	service := NewAnalyticsService(repo)
	userID := uuid.New()

	t.Run("GetSubjectStudyTimes defaults to semana when invalid", func(t *testing.T) {
		times, err := service.GetSubjectStudyTimes(context.Background(), userID, "invalid", "2026-11-08")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(times) != 1 || times[0].SubjectName != "Golang" {
			t.Fatalf("expected Golang subject, got %+v", times)
		}
	})

	t.Run("GetStreak returns streak response", func(t *testing.T) {
		res, err := service.GetStreak(context.Background(), userID, "2026-11-08")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.CurrentStreak != 2 || !res.TodayCompleted {
			t.Fatalf("unexpected streak response: %+v", res)
		}
	})
}
