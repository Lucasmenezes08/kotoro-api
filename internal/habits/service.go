package habits

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type HabitsServiceContract interface {
	GetWaterDay(ctx context.Context, userID uuid.UUID, dateStr string) (*WaterDayResponse, error)
	AddWaterLog(ctx context.Context, userID uuid.UUID, req AddWaterRequest) (*WaterDayResponse, error)
	UpdateWaterTarget(ctx context.Context, userID uuid.UUID, req UpdateWaterTargetRequest) (*WaterDayResponse, error)
}

type HabitsService struct {
	repo HabitsRepositoryContract
}

func NewHabitsService(repo HabitsRepositoryContract) *HabitsService {
	return &HabitsService{repo: repo}
}

func normalizeDate(d *string) string {
	if d == nil || *d == "" {
		return time.Now().Format("2006-01-02")
	}
	t, err := time.Parse("2006-01-02", *d)
	if err != nil {
		return time.Now().Format("2006-01-02")
	}
	return t.Format("2006-01-02")
}

func (s *HabitsService) GetWaterDay(ctx context.Context, userID uuid.UUID, dateStr string) (*WaterDayResponse, error) {
	date := normalizeDate(&dateStr)
	return s.repo.GetWaterDay(ctx, userID, date)
}

func (s *HabitsService) AddWaterLog(ctx context.Context, userID uuid.UUID, req AddWaterRequest) (*WaterDayResponse, error) {
	date := normalizeDate(req.Date)
	if req.AmountMl == 0 {
		return s.repo.GetWaterDay(ctx, userID, date)
	}
	return s.repo.AddWaterLog(ctx, userID, date, req.AmountMl)
}

func (s *HabitsService) UpdateWaterTarget(ctx context.Context, userID uuid.UUID, req UpdateWaterTargetRequest) (*WaterDayResponse, error) {
	date := normalizeDate(req.Date)
	target := req.TargetMl
	if target <= 0 {
		target = 2000
	}
	return s.repo.UpdateWaterTarget(ctx, userID, date, target)
}
