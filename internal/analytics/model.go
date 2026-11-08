package analytics

import "github.com/google/uuid"

type SubjectStudyTime struct {
	SubjectID    uuid.UUID `json:"subject_id"`
	SubjectName  string    `json:"subject_name"`
	SubjectColor string    `json:"subject_color"`
	TotalSeconds int       `json:"total_seconds"`
	Percentage   int       `json:"percentage"`
}

type StreakWeekDay struct {
	Date        string `json:"date"`
	DayShort    string `json:"day_short"`
	DayNumber   int    `json:"day_number"`
	IsCompleted bool   `json:"is_completed"`
	IsToday     bool   `json:"is_today"`
}

type StreakResponse struct {
	CurrentStreak          int             `json:"current_streak"`
	LongestStreak          int             `json:"longest_streak"`
	TodayCompleted         bool            `json:"today_completed"`
	Level                  string          `json:"level"`
	NextLevelDaysRemaining *int            `json:"next_level_days_remaining"`
	WeekDays               []StreakWeekDay `json:"week_days"`
}
