package habits

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type HabitsRepositoryContract interface {
	GetWaterDay(ctx context.Context, userID uuid.UUID, date string) (*WaterDayResponse, error)
	AddWaterLog(ctx context.Context, userID uuid.UUID, date string, amountMl int) (*WaterDayResponse, error)
	UpdateWaterTarget(ctx context.Context, userID uuid.UUID, date string, targetMl int) (*WaterDayResponse, error)
}

type HabitsRepository struct {
	db *sqlx.DB
}

func NewHabitsRepository(db *sqlx.DB) *HabitsRepository {
	return &HabitsRepository{db: db}
}

func (r *HabitsRepository) GetWaterDay(ctx context.Context, userID uuid.UUID, date string) (*WaterDayResponse, error) {
	var record WaterDayRecord
	query := `SELECT id, user_id, to_char(date, 'YYYY-MM-DD') as date, total_ml, target_ml, created_at, updated_at 
	          FROM water_day_records 
	          WHERE user_id = $1 AND date = $2::date`
	err := r.db.GetContext(ctx, &record, query, userID, date)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &WaterDayResponse{
				Date:     date,
				TotalMl:  0,
				TargetMl: 2000,
				Logs:     []WaterLogEntry{},
			}, nil
		}
		return nil, err
	}

	var logs []WaterLogEntry
	logsQuery := `SELECT id, water_record_id, user_id, amount_ml, logged_at 
	              FROM water_logs 
	              WHERE water_record_id = $1 
	              ORDER BY logged_at DESC`
	err = r.db.SelectContext(ctx, &logs, logsQuery, record.ID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if logs == nil {
		logs = []WaterLogEntry{}
	}

	return &WaterDayResponse{
		Date:     record.Date,
		TotalMl:  record.TotalMl,
		TargetMl: record.TargetMl,
		Logs:     logs,
	}, nil
}

func (r *HabitsRepository) AddWaterLog(ctx context.Context, userID uuid.UUID, date string, amountMl int) (*WaterDayResponse, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() //nolint:errcheck

	var record WaterDayRecord
	upsertQuery := `
		INSERT INTO water_day_records (user_id, date, total_ml, target_ml, updated_at)
		VALUES ($1, $2::date, $3, 2000, now())
		ON CONFLICT (user_id, date)
		DO UPDATE SET total_ml = GREATEST(0, water_day_records.total_ml + EXCLUDED.total_ml), updated_at = now()
		RETURNING id, user_id, to_char(date, 'YYYY-MM-DD') as date, total_ml, target_ml, created_at, updated_at;
	`
	err = tx.GetContext(ctx, &record, upsertQuery, userID, date, amountMl)
	if err != nil {
		return nil, err
	}

	logInsert := `
		INSERT INTO water_logs (water_record_id, user_id, amount_ml, logged_at)
		VALUES ($1, $2, $3, $4);
	`
	_, err = tx.ExecContext(ctx, logInsert, record.ID, userID, amountMl, time.Now())
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return r.GetWaterDay(ctx, userID, date)
}

func (r *HabitsRepository) UpdateWaterTarget(ctx context.Context, userID uuid.UUID, date string, targetMl int) (*WaterDayResponse, error) {
	upsertQuery := `
		INSERT INTO water_day_records (user_id, date, total_ml, target_ml, updated_at)
		VALUES ($1, $2::date, 0, $3, now())
		ON CONFLICT (user_id, date)
		DO UPDATE SET target_ml = EXCLUDED.target_ml, updated_at = now()
		RETURNING id, user_id, to_char(date, 'YYYY-MM-DD') as date, total_ml, target_ml, created_at, updated_at;
	`
	var record WaterDayRecord
	err := r.db.GetContext(ctx, &record, upsertQuery, userID, date, targetMl)
	if err != nil {
		return nil, err
	}

	return r.GetWaterDay(ctx, userID, date)
}
