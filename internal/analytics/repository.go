package analytics

import (
	"context"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type AnalyticsRepositoryContract interface {
	GetSubjectStudyTimes(ctx context.Context, userID uuid.UUID, timeframe string, refDate time.Time) ([]SubjectStudyTime, error)
	GetStreak(ctx context.Context, userID uuid.UUID, refDate time.Time) (*StreakResponse, error)
}

type AnalyticsRepository struct {
	db *sqlx.DB
}

func NewAnalyticsRepository(db *sqlx.DB) *AnalyticsRepository {
	return &AnalyticsRepository{db: db}
}

func (r *AnalyticsRepository) GetSubjectStudyTimes(
	ctx context.Context,
	userID uuid.UUID,
	timeframe string,
	refDate time.Time,
) ([]SubjectStudyTime, error) {
	var startDateStr, endDateStr *string

	if timeframe == "semana" {
		// Week starts on Sunday (civil Brazilian standard: Domingo a Sábado)
		weekday := int(refDate.Weekday())
		weekStart := refDate.AddDate(0, 0, -weekday)
		weekEnd := weekStart.AddDate(0, 0, 6)

		sStart := weekStart.Format("2006-01-02")
		sEnd := weekEnd.Format("2006-01-02")
		startDateStr = &sStart
		endDateStr = &sEnd
	}

	type dbRow struct {
		SubjectID    uuid.UUID `db:"subject_id"`
		SubjectName  string    `db:"subject_name"`
		SubjectColor string    `db:"subject_color"`
		TotalSeconds int       `db:"total_seconds"`
	}

	query := `
		SELECT 
			s.id AS subject_id,
			s.name AS subject_name,
			s.color AS subject_color,
			COALESCE(SUM(
				CASE 
					WHEN sc.elapsed_seconds > 0 THEN sc.elapsed_seconds
					WHEN sb.status = 'finalizado' AND sb.started_at IS NOT NULL AND sb.completed_at IS NOT NULL 
					     THEN EXTRACT(EPOCH FROM (sb.completed_at - sb.started_at))::int
					ELSE 0
				END
			), 0)::int AS total_seconds
		FROM subjects s
		LEFT JOIN sprint_block sb ON sb.subject_id = s.id AND sb.deleted_at IS NULL
		LEFT JOIN sprints sp ON sb.sprint_id = sp.id AND sp.deleted_at IS NULL AND sp.user_id = $1
		LEFT JOIN study_cycles sc ON sc.sprint_block_id = sb.id 
			AND sc.status != 'cancelado' 
			AND (sc.stop_reason IS NULL OR sc.stop_reason != 'cancelado')
			AND sc.elapsed_seconds > 0
		WHERE s.user_id = $1 AND s.deleted_at IS NULL
		  AND ($2::date IS NULL OR (sp.sprint_date >= $2::date AND sp.sprint_date <= $3::date))
		GROUP BY s.id, s.name, s.color
		ORDER BY total_seconds DESC, s.name ASC;
	`

	var rows []dbRow
	err := r.db.SelectContext(ctx, &rows, query, userID, startDateStr, endDateStr)
	if err != nil {
		return nil, err
	}

	totalAllSeconds := 0
	for _, row := range rows {
		totalAllSeconds += row.TotalSeconds
	}

	results := make([]SubjectStudyTime, 0, len(rows))
	for _, row := range rows {
		pct := 0
		if totalAllSeconds > 0 {
			pct = (row.TotalSeconds * 100) / totalAllSeconds
		}
		results = append(results, SubjectStudyTime{
			SubjectID:    row.SubjectID,
			SubjectName:  row.SubjectName,
			SubjectColor: row.SubjectColor,
			TotalSeconds: row.TotalSeconds,
			Percentage:   pct,
		})
	}

	return results, nil
}

func (r *AnalyticsRepository) GetStreak(
	ctx context.Context,
	userID uuid.UUID,
	refDate time.Time,
) (*StreakResponse, error) {
	todayStr := refDate.Format("2006-01-02")

	// Collect all study dates where user finished a sprint, completed a block, or performed a valid non-cancelled study cycle
	query := `
		SELECT DISTINCT to_char(sp.sprint_date, 'YYYY-MM-DD') as sprint_date
		FROM sprints sp
		WHERE sp.user_id = $1 AND sp.deleted_at IS NULL AND sp.status = 'finished'

		UNION

		SELECT DISTINCT to_char(sp.sprint_date, 'YYYY-MM-DD') as sprint_date
		FROM sprints sp
		JOIN sprint_block sb ON sb.sprint_id = sp.id AND sb.deleted_at IS NULL
		JOIN study_cycles sc ON sc.sprint_block_id = sb.id 
			AND sc.status != 'cancelado' 
			AND (sc.stop_reason IS NULL OR sc.stop_reason != 'cancelado')
			AND sc.elapsed_seconds > 0
		WHERE sp.user_id = $1 AND sp.deleted_at IS NULL

		UNION

		SELECT DISTINCT to_char(sp.sprint_date, 'YYYY-MM-DD') as sprint_date
		FROM sprints sp
		JOIN sprint_block sb ON sb.sprint_id = sp.id AND sb.deleted_at IS NULL
		WHERE sp.user_id = $1 AND sp.deleted_at IS NULL AND sb.status = 'finalizado';
	`

	var activeDates []string
	err := r.db.SelectContext(ctx, &activeDates, query, userID)
	if err != nil {
		return nil, err
	}

	activeSet := make(map[string]bool, len(activeDates))
	for _, d := range activeDates {
		activeSet[d] = true
	}

	todayCompleted := activeSet[todayStr]

	// Calculate current streak
	currentStreak := 0
	checkDate := refDate
	if !todayCompleted {
		checkDate = checkDate.AddDate(0, 0, -1)
	}

	for {
		dStr := checkDate.Format("2006-01-02")
		if activeSet[dStr] {
			currentStreak++
			checkDate = checkDate.AddDate(0, 0, -1)
		} else {
			break
		}
	}

	// Calculate longest streak
	sort.Strings(activeDates)
	longestStreak := 0
	currentRun := 0
	var prevDate time.Time

	for i, dStr := range activeDates {
		t, err := time.Parse("2006-01-02", dStr)
		if err != nil {
			continue
		}
		if i == 0 {
			currentRun = 1
		} else {
			if t.Sub(prevDate).Hours() == 24 {
				currentRun++
			} else {
				currentRun = 1
			}
		}
		if currentRun > longestStreak {
			longestStreak = currentRun
		}
		prevDate = t
	}
	if currentStreak > longestStreak {
		longestStreak = currentStreak
	}

	// Determine level and remaining days
	level := "faisca"
	var nextDays *int
	if currentStreak >= 15 {
		level = "supernova"
		nextDays = nil
	} else if currentStreak >= 8 {
		level = "labareda"
		rem := 15 - currentStreak
		nextDays = &rem
	} else if currentStreak >= 4 {
		level = "brasa"
		rem := 8 - currentStreak
		nextDays = &rem
	} else {
		level = "faisca"
		rem := 4 - currentStreak
		nextDays = &rem
	}

	// Generate 7 days of the current week (Sunday to Saturday)
	weekday := int(refDate.Weekday())
	weekStart := refDate.AddDate(0, 0, -weekday)
	shortLabels := []string{"D", "S", "T", "Q", "Q", "S", "S"}

	weekDays := make([]StreakWeekDay, 0, 7)
	for i := 0; i < 7; i++ {
		d := weekStart.AddDate(0, 0, i)
		dStr := d.Format("2006-01-02")
		weekDays = append(weekDays, StreakWeekDay{
			Date:        dStr,
			DayShort:    shortLabels[i],
			DayNumber:   d.Day(),
			IsCompleted: activeSet[dStr],
			IsToday:     dStr == todayStr,
		})
	}

	// Persist / upsert in user_streaks
	upsertStreakQuery := `
		INSERT INTO user_streaks (user_id, current_streak, longest_streak, last_study_date, updated_at)
		VALUES ($1, $2, $3, $4::date, now())
		ON CONFLICT (user_id)
		DO UPDATE SET current_streak = EXCLUDED.current_streak,
		              longest_streak = GREATEST(user_streaks.longest_streak, EXCLUDED.longest_streak),
		              last_study_date = EXCLUDED.last_study_date,
		              updated_at = now();
	`
	var lastDateParam *string
	if todayCompleted {
		lastDateParam = &todayStr
	} else if len(activeDates) > 0 {
		last := activeDates[len(activeDates)-1]
		lastDateParam = &last
	}
	_ = r.db.QueryRowContext(ctx, upsertStreakQuery, userID, currentStreak, longestStreak, lastDateParam)

	return &StreakResponse{
		CurrentStreak:          currentStreak,
		LongestStreak:          longestStreak,
		TodayCompleted:         todayCompleted,
		Level:                  level,
		NextLevelDaysRemaining: nextDays,
		WeekDays:               weekDays,
	}, nil
}
