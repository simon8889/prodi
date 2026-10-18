package store

import (
	"context"
	"database/sql"
	"fmt"

	"prodi/api/internal/domain"
)

type migration struct {
	name string
	stmt string
}

var migrations = []migration{
	{
		name: "0001_init",
		stmt: `
CREATE TABLE IF NOT EXISTS tasks (
	id                   TEXT PRIMARY KEY,
	title                TEXT NOT NULL,
	estimated_pomodoros  INTEGER NOT NULL,
	completed_pomodoros  INTEGER NOT NULL DEFAULT 0,
	status               TEXT NOT NULL DEFAULT 'pending',
	position             INTEGER NOT NULL,
	created_at           TEXT NOT NULL,
	completed_at         TEXT
);

CREATE INDEX IF NOT EXISTS idx_tasks_position ON tasks(position);
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);

CREATE TABLE IF NOT EXISTS pomodoro_sessions (
	id                TEXT PRIMARY KEY,
	task_id           TEXT REFERENCES tasks(id) ON DELETE SET NULL,
	task_title        TEXT NOT NULL DEFAULT '',
	phase             TEXT NOT NULL,
	started_at        TEXT NOT NULL,
	ended_at          TEXT NOT NULL,
	duration_seconds  INTEGER NOT NULL,
	completed         INTEGER NOT NULL DEFAULT 0,
	abandoned         INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_sessions_ended_at ON pomodoro_sessions(ended_at DESC);

CREATE TABLE IF NOT EXISTS settings (
	id                      INTEGER PRIMARY KEY CHECK (id = 1),
	focus_minutes           INTEGER NOT NULL,
	short_break_minutes     INTEGER NOT NULL,
	long_break_minutes      INTEGER NOT NULL,
	long_break_every_cycles INTEGER NOT NULL,
	auto_start_breaks       INTEGER NOT NULL DEFAULT 1,
	updated_at              TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS timer_state (
	id                      INTEGER PRIMARY KEY CHECK (id = 1),
	phase                   TEXT NOT NULL,
	status                  TEXT NOT NULL,
	phase_started_at        TEXT NOT NULL,
	phase_duration_seconds  INTEGER NOT NULL,
	remaining_seconds       INTEGER NOT NULL,
	cycle_count             INTEGER NOT NULL,
	active_task_id          TEXT REFERENCES tasks(id) ON DELETE SET NULL
);
`,
	},
}

func (s *Store) migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
	name       TEXT PRIMARY KEY,
	applied_at TEXT NOT NULL
);`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	applied := make(map[string]bool)
	rows, err := s.db.QueryContext(ctx, `SELECT name FROM schema_migrations`)
	if err != nil {
		return fmt.Errorf("read schema_migrations: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return fmt.Errorf("scan schema_migrations: %w", err)
		}
		applied[name] = true
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate schema_migrations: %w", err)
	}

	for _, m := range migrations {
		if applied[m.name] {
			continue
		}
		err := withTx(ctx, s, func(tx *sql.Tx) error {
			if _, err := tx.ExecContext(ctx, m.stmt); err != nil {
				return err
			}
			_, err := tx.ExecContext(ctx,
				`INSERT INTO schema_migrations (name, applied_at) VALUES (?, ?)`,
				m.name, formatTime(s.now()))
			return err
		})
		if err != nil {
			return fmt.Errorf("apply migration %s: %w", m.name, err)
		}
	}

	return s.ensureSeedRows(ctx)
}

func (s *Store) ensureSeedRows(ctx context.Context) error {
	defaults := domain.DefaultSettings()
	now := formatTime(s.now())
	focusSeconds := defaults.SecondsFor(domain.PhaseFocus)

	return withTx(ctx, s, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO settings
			 (id, focus_minutes, short_break_minutes, long_break_minutes, long_break_every_cycles, auto_start_breaks, updated_at)
			 VALUES (1, ?, ?, ?, ?, ?, ?)`,
			defaults.FocusMinutes, defaults.ShortBreakMinutes, defaults.LongBreakMinutes,
			defaults.LongBreakEveryCycles, defaults.AutoStartBreaks, now,
		); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO timer_state
			 (id, phase, status, phase_started_at, phase_duration_seconds, remaining_seconds, cycle_count, active_task_id)
			 VALUES (1, 'focus', 'idle', ?, ?, ?, 0, NULL)`,
			now, focusSeconds, focusSeconds,
		)
		return err
	})
}
