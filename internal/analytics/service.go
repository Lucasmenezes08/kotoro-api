package analytics

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type AnalyticsServiceContract interface {
	GetSubjectStudyTimes(ctx context.Context, userID uuid.UUID, timeframe string, dateStr string) ([]SubjectStudyTime, error)
	GetStreak(ctx context.Context, userID uuid.UUID, dateStr string) (*StreakResponse, error)
}

type AnalyticsService struct {
	repo AnalyticsRepositoryContract
}

func NewAnalyticsService(repo AnalyticsRepositoryContract) *AnalyticsService {
	return &AnalyticsService{repo: repo}
}

func parseReferenceDate(d string) time.Time {
	if d == "" {
		return time.Now()
	}
	t, err := time.Parse("2006-01-02", d)
	if err != nil {
		return time.Now()
	}
	return t
}

func (s *AnalyticsService) GetSubjectStudyTimes(
	ctx context.Context,
	userID uuid.UUID,
	timeframe string,
	dateStr string,
) ([]SubjectStudyTime, error) {
	if timeframe != "geral" {
		timeframe = "semana"
	}
	refDate := parseReferenceDate(dateStr)
	return s.repo.GetSubjectStudyTimes(ctx, userID, timeframe, refDate)
}

func (s *AnalyticsService) GetStreak(
	ctx context.Context,
	userID uuid.UUID,
	dateStr string,
) (*StreakResponse, error) {
	refDate := parseReferenceDate(dateStr)
	return s.repo.GetStreak(ctx, userID, refDate)
}
