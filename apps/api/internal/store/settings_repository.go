package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"prodi/api/internal/domain"
)

type SettingsRepository struct {
	store *Store
}

func NewSettingsRepository(s *Store) *SettingsRepository {
	return &SettingsRepository{store: s}
}

func (r *SettingsRepository) Get(ctx context.Context) (domain.Settings, error) {
	var (
		settings       domain.Settings
		autoStartBreaks int
	)
	err := r.store.db.QueryRowContext(ctx, `
SELECT focus_minutes, short_break_minutes, long_break_minutes, long_break_every_cycles, auto_start_breaks
FROM settings WHERE id = 1`,
	).Scan(
		&settings.FocusMinutes,
		&settings.ShortBreakMinutes,
		&settings.LongBreakMinutes,
		&settings.LongBreakEveryCycles,
		&autoStartBreaks,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.DefaultSettings(), domain.ErrMissingSetting
	}
	if err != nil {
		return domain.DefaultSettings(), fmt.Errorf("get settings: %w", err)
	}
	settings.AutoStartBreaks = autoStartBreaks != 0
	return settings, nil
}

func (r *SettingsRepository) Save(ctx context.Context, settings domain.Settings) error {
	if err := settings.Validate(); err != nil {
		return err
	}
	settings = settings.Normalize()
	_, err := r.store.db.ExecContext(ctx, `
UPDATE settings
SET focus_minutes = ?, short_break_minutes = ?, long_break_minutes = ?, long_break_every_cycles = ?, auto_start_breaks = ?, updated_at = ?
WHERE id = 1`,
		settings.FocusMinutes, settings.ShortBreakMinutes, settings.LongBreakMinutes,
		settings.LongBreakEveryCycles, settings.AutoStartBreaks, formatTime(r.store.now()),
	)
	if err != nil {
		return fmt.Errorf("save settings: %w", err)
	}
	return nil
}

type TimerStateRepository struct {
	store *Store
}

func NewTimerStateRepository(s *Store) *TimerStateRepository {
	return &TimerStateRepository{store: s}
}

func (r *TimerStateRepository) Load(ctx context.Context) (domain.TimerState, error) {
	var (
		state       domain.TimerState
		phase       string
		status      string
		phaseStart  string
		activeTask  sql.NullString
	)
	err := r.store.db.QueryRowContext(ctx, `
SELECT phase, status, phase_started_at, phase_duration_seconds, remaining_seconds, cycle_count, active_task_id
FROM timer_state WHERE id = 1`,
	).Scan(
		&phase,
		&status,
		&phaseStart,
		&state.PhaseDurationSeconds,
		&state.RemainingSeconds,
		&state.CycleCount,
		&activeTask,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.NewTimerState(domain.DefaultSettings().SecondsFor(domain.PhaseFocus)), domain.ErrNotFound
	}
	if err != nil {
		return domain.TimerState{}, fmt.Errorf("load timer state: %w", err)
	}

	state.Phase = domain.Phase(phase)
	state.Status = domain.TimerStatus(status)
	state.PhaseStartedAt = parseTime(phaseStart)
	state.ActiveTaskID = nullableString(activeTask)
	return state, nil
}

func (r *TimerStateRepository) Save(ctx context.Context, state domain.TimerState) error {
	_, err := r.store.db.ExecContext(ctx, `
UPDATE timer_state
SET phase = ?, status = ?, phase_started_at = ?, phase_duration_seconds = ?, remaining_seconds = ?, cycle_count = ?, active_task_id = ?
WHERE id = 1`,
		string(state.Phase), string(state.Status), formatTime(state.PhaseStartedAt),
		state.PhaseDurationSeconds, state.RemainingSeconds, state.CycleCount,
		nullStringValue(state.ActiveTaskID),
	)
	if err != nil {
		return fmt.Errorf("save timer state: %w", err)
	}
	return nil
}
