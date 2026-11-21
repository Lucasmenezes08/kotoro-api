package habits

import (
	"time"

	"github.com/google/uuid"
)

type WaterDayRecord struct {
	ID        uuid.UUID `db:"id" json:"id"`
	UserID    uuid.UUID `db:"user_id" json:"user_id"`
	Date      string    `db:"date" json:"date"`
	TotalMl   int       `db:"total_ml" json:"total_ml"`
	TargetMl  int       `db:"target_ml" json:"target_ml"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

type WaterLogEntry struct {
	ID            uuid.UUID `db:"id" json:"id"`
	WaterRecordID uuid.UUID `db:"water_record_id" json:"water_record_id"`
	UserID        uuid.UUID `db:"user_id" json:"user_id"`
	AmountMl      int       `db:"amount_ml" json:"amount_ml"`
	LoggedAt      time.Time `db:"logged_at" json:"timestamp"`
}

type WaterDayResponse struct {
	Date     string          `json:"date"`
	TotalMl  int             `json:"total_ml"`
	TargetMl int             `json:"target_ml"`
	Logs     []WaterLogEntry `json:"logs"`
}

type AddWaterRequest struct {
	Date     *string `json:"date"`
	AmountMl int     `json:"amount_ml"`
}

type UpdateWaterTargetRequest struct {
	Date     *string `json:"date"`
	TargetMl int     `json:"target_ml"`
}
