package habits

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

type mockHabitsRepo struct {
	waterDay *WaterDayResponse
}

func (m *mockHabitsRepo) GetWaterDay(ctx context.Context, userID uuid.UUID, date string) (*WaterDayResponse, error) {
	return m.waterDay, nil
}

func (m *mockHabitsRepo) AddWaterLog(ctx context.Context, userID uuid.UUID, date string, amountMl int) (*WaterDayResponse, error) {
	m.waterDay.TotalMl += amountMl
	return m.waterDay, nil
}

func (m *mockHabitsRepo) UpdateWaterTarget(ctx context.Context, userID uuid.UUID, date string, targetMl int) (*WaterDayResponse, error) {
	m.waterDay.TargetMl = targetMl
	return m.waterDay, nil
}

func TestHabitsService(t *testing.T) {
	repo := &mockHabitsRepo{
		waterDay: &WaterDayResponse{
			Date:     "2026-11-08",
			TotalMl:  500,
			TargetMl: 2000,
			Logs:     []WaterLogEntry{},
		},
	}

	service := NewHabitsService(repo)
	userID := uuid.New()

	t.Run("GetWaterDay normalizes date", func(t *testing.T) {
		res, err := service.GetWaterDay(context.Background(), userID, "2026-11-08")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.TotalMl != 500 {
			t.Fatalf("expected total 500, got %d", res.TotalMl)
		}
	})

	t.Run("AddWaterLog adds amount", func(t *testing.T) {
		date := "2026-11-08"
		res, err := service.AddWaterLog(context.Background(), userID, AddWaterRequest{
			Date:     &date,
			AmountMl: 250,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.TotalMl != 750 {
			t.Fatalf("expected total 750, got %d", res.TotalMl)
		}
	})

	t.Run("UpdateWaterTarget updates target", func(t *testing.T) {
		date := "2026-11-08"
		res, err := service.UpdateWaterTarget(context.Background(), userID, UpdateWaterTargetRequest{
			Date:     &date,
			TargetMl: 2500,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.TargetMl != 2500 {
			t.Fatalf("expected target 2500, got %d", res.TargetMl)
		}
	})
}
