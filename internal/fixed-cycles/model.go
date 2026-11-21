package fixedcycles

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var (
	ErrFixedCycleNotFound          = errors.New("fixed cycle not found")
	ErrFixedCycleTitleRequired     = errors.New("title is required")
	ErrFixedCycleTitleTooLong      = errors.New("title must be at most 150 characters")
	ErrFixedCycleInvalidSubject    = errors.New("invalid subject id")
	ErrFixedCycleInvalidDayOfWeek  = errors.New("invalid day of week")
	ErrFixedCycleInvalidEstimated  = errors.New("estimated seconds must be greater than zero")
)

type StringList []string

func (s StringList) Value() (driver.Value, error) {
	if s == nil {
		return "[]", nil
	}
	return json.Marshal(s)
}

func (s *StringList) Scan(src any) error {
	if src == nil {
		*s = []string{}
		return nil
	}
	var b []byte
	switch v := src.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		return fmt.Errorf("unsupported type for StringList: %T", src)
	}
	if len(b) == 0 {
		*s = []string{}
		return nil
	}
	return json.Unmarshal(b, s)
}

type FixedCycle struct {
	ID               uuid.UUID  `db:"id" json:"id"`
	UserID           uuid.UUID  `db:"user_id" json:"user_id"`
	SubjectID        uuid.UUID  `db:"subject_id" json:"subject_id"`
	SubjectName      *string    `db:"subject_name" json:"subject_name,omitempty"`
	SubjectColor     *string    `db:"subject_color" json:"subject_color,omitempty"`
	Title            string     `db:"title" json:"title"`
	EstimatedSeconds int        `db:"estimated_seconds" json:"estimated_seconds"`
	DayOfWeek        string     `db:"day_of_week" json:"day_of_week"`
	Tasks            StringList `db:"tasks" json:"tasks"`
	CreatedAt        time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt        time.Time  `db:"updated_at" json:"updated_at"`
	DeletedAt        *time.Time `db:"deleted_at" json:"deleted_at,omitempty"`
}

type CreateFixedCycleInput struct {
	SubjectID        uuid.UUID  `json:"subject_id"`
	Title            string     `json:"title"`
	EstimatedSeconds int        `json:"estimated_seconds"`
	DayOfWeek        string     `json:"day_of_week"`
	Tasks            []string   `json:"tasks"`
}

type UpdateFixedCycleInput struct {
	SubjectID        *uuid.UUID `json:"subject_id,omitempty"`
	Title            *string    `json:"title,omitempty"`
	EstimatedSeconds *int       `json:"estimated_seconds,omitempty"`
	DayOfWeek        *string    `json:"day_of_week,omitempty"`
	Tasks            *[]string  `json:"tasks,omitempty"`
}
