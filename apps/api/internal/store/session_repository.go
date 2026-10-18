package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"prodi/api/internal/domain"
)

type SessionRepository struct {
	store *Store
}

func NewSessionRepository(s *Store) *SessionRepository {
	return &SessionRepository{store: s}
}

func (r *SessionRepository) Create(ctx context.Context, session *domain.Session) error {
	if session.ID == "" {
		session.ID = uuid.NewString()
	}
	if session.StartedAt.IsZero() {
		session.StartedAt = r.store.now().UTC()
	}
	if session.EndedAt.IsZero() {
		session.EndedAt = r.store.now().UTC()
	}
	_, err := r.store.db.ExecContext(ctx, `
INSERT INTO pomodoro_sessions
	(id, task_id, task_title, phase, started_at, ended_at, duration_seconds, completed, abandoned)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		session.ID, nullStringValue(session.TaskID), session.TaskTitle, string(session.Phase),
		formatTime(session.StartedAt), formatTime(session.EndedAt), session.DurationSeconds,
		session.Completed, session.Abandoned,
	)
	if err != nil {
		return fmt.Errorf("insert session: %w", err)
	}
	return nil
}

func (r *SessionRepository) ListRecent(ctx context.Context, limit int) ([]domain.Session, error) {
	if limit <= 0 {
		limit = 25
	}
	rows, err := r.store.db.QueryContext(ctx, `
SELECT id, task_id, task_title, phase, started_at, ended_at, duration_seconds, completed, abandoned
FROM pomodoro_sessions
ORDER BY ended_at DESC
LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer rows.Close()

	sessions := make([]domain.Session, 0)
	for rows.Next() {
		session, err := scanSession(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}
	return sessions, rows.Err()
}

func (r *SessionRepository) Stats(ctx context.Context, today time.Time) (domain.Stats, error) {
	stats := domain.EmptyStats()
	stats.GeneratedAt = r.store.now().UTC()

	pending, done, err := r.taskCounts(ctx)
	if err != nil {
		return stats, err
	}
	stats.TasksPending = pending
	stats.TasksCompleted = done
	stats.TasksTotal = pending + done

	var focusSeconds sqlSum
	err = r.store.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(duration_seconds), 0), COUNT(*) FROM pomodoro_sessions WHERE phase = 'focus' AND completed = 1`,
	).Scan(&focusSeconds.Seconds, &focusSeconds.Count)
	if err != nil {
		return stats, fmt.Errorf("focus totals: %w", err)
	}
	stats.FocusSeconds = focusSeconds.Seconds
	stats.FocusSessions = focusSeconds.Count

	startOfDay := today.UTC().Truncate(24 * time.Hour)
	var todaySum sqlSum
	err = r.store.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(duration_seconds), 0), COUNT(*) FROM pomodoro_sessions WHERE phase = 'focus' AND completed = 1 AND ended_at >= ?`,
		formatTime(startOfDay),
	).Scan(&todaySum.Seconds, &todaySum.Count)
	if err != nil {
		return stats, fmt.Errorf("today totals: %w", err)
	}
	stats.TodayFocusSeconds = todaySum.Seconds
	stats.TodaySessions = todaySum.Count

	streak, err := r.currentStreak(ctx, startOfDay)
	if err != nil {
		return stats, err
	}
	stats.CurrentStreakDays = streak

	return stats, nil
}

func (r *SessionRepository) taskCounts(ctx context.Context) (pending int, done int, err error) {
	return NewTaskRepository(r.store).CountByStatus(ctx)
}

func (r *SessionRepository) currentStreak(ctx context.Context, startOfDay time.Time) (int, error) {
	rows, err := r.store.db.QueryContext(ctx,
		`SELECT DISTINCT substr(ended_at, 1, 10) FROM pomodoro_sessions WHERE phase = 'focus' AND completed = 1 ORDER BY 1 DESC LIMIT 400`)
	if err != nil {
		return 0, fmt.Errorf("streak days: %w", err)
	}
	defer rows.Close()

	days := make(map[string]struct{})
	for rows.Next() {
		var day string
		if err := rows.Scan(&day); err != nil {
			return 0, err
		}
		days[day] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if len(days) == 0 {
		return 0, nil
	}

	cursor := startOfDay
	if _, ok := days[cursor.Format("2006-01-02")]; !ok {
		cursor = cursor.AddDate(0, 0, -1)
		if _, ok := days[cursor.Format("2006-01-02")]; !ok {
			return 0, nil
		}
	}

	streak := 0
	for {
		if _, ok := days[cursor.Format("2006-01-02")]; !ok {
			break
		}
		streak++
		cursor = cursor.AddDate(0, 0, -1)
	}
	return streak, nil
}

type sqlSum struct {
	Seconds int
	Count   int
}

func scanSession(row rowScanner) (domain.Session, error) {
	var (
		session   domain.Session
		taskID    sql.NullString
		phase     string
		startedAt string
		endedAt   string
		completed int
		abandoned int
	)
	err := row.Scan(
		&session.ID,
		&taskID,
		&session.TaskTitle,
		&phase,
		&startedAt,
		&endedAt,
		&session.DurationSeconds,
		&completed,
		&abandoned,
	)
	if err != nil {
		return domain.Session{}, err
	}
	session.TaskID = nullableString(taskID)
	session.Phase = domain.Phase(phase)
	session.StartedAt = parseTime(startedAt)
	session.EndedAt = parseTime(endedAt)
	session.Completed = completed != 0
	session.Abandoned = abandoned != 0
	return session, nil
}
