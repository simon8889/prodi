package timer

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"prodi/api/internal/domain"
	"prodi/api/internal/protocol"
	"prodi/api/internal/store"
)

type Engine struct {
	mu       sync.Mutex
	pubMu    sync.RWMutex
	store    *store.Store
	tasks    *store.TaskRepository
	sessions *store.SessionRepository
	timerRow *store.TimerStateRepository
	settings *store.SettingsRepository

	state     domain.TimerState
	config    domain.Settings
	now       func() time.Time
	broadcast func(eventType string, payload any)
	logger    *slog.Logger
}

func New(
	database *store.Store,
	tasks *store.TaskRepository,
	sessions *store.SessionRepository,
	timerRow *store.TimerStateRepository,
	settingsRepo *store.SettingsRepository,
	logger *slog.Logger,
) *Engine {
	return &Engine{
		store:    database,
		tasks:    tasks,
		sessions: sessions,
		timerRow: timerRow,
		settings: settingsRepo,
		now:      time.Now,
		logger:   logger,
	}
}

func (e *Engine) SetBroadcast(fn func(eventType string, payload any)) {
	e.pubMu.Lock()
	defer e.pubMu.Unlock()
	e.broadcast = fn
}

func (e *Engine) Bootstrap(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	config, err := e.settings.Get(ctx)
	if err != nil && !errors.Is(err, domain.ErrMissingSetting) {
		return err
	}
	e.config = config.Normalize()

	state, err := e.timerRow.Load(ctx)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return err
	}
	if errors.Is(err, domain.ErrNotFound) {
		state = domain.NewTimerState(e.config.SecondsFor(domain.PhaseFocus))
	}
	if !state.Phase.Valid() {
		state.Phase = domain.PhaseFocus
	}
	if state.PhaseDurationSeconds <= 0 {
		state.PhaseDurationSeconds = e.config.SecondsFor(state.Phase)
	}

	now := e.now().UTC()
	if state.Status == domain.TimerRunning {
		remaining := state.RemainingAt(now)
		if remaining > 0 {
			state.RemainingSeconds = remaining
			state.Status = domain.TimerPaused
		} else {
			state.Status = domain.TimerIdle
			state.PhaseStartedAt = now
			state.RemainingSeconds = state.PhaseDurationSeconds
		}
	}
	if state.PhaseStartedAt.IsZero() {
		state.PhaseStartedAt = now
	}
	if state.RemainingSeconds <= 0 || state.RemainingSeconds > state.PhaseDurationSeconds {
		state.RemainingSeconds = state.PhaseDurationSeconds
	}

	e.state = state
	return e.persistLocked(ctx)
}

func (e *Engine) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.Tick()
		}
	}
}

func (e *Engine) Tick() {
	e.mu.Lock()
	changed := false

	if e.state.Status == domain.TimerRunning && e.state.RemainingAt(e.now().UTC()) <= 0 {
		changed = e.completePhaseLocked(context.Background(), CompletionReasonElapsed) != nil
	}

	e.state = e.state.WithRemaining(e.now().UTC())
	payload := e.tickPayloadLocked()
	e.mu.Unlock()

	if changed {
		e.publishState()
	}
	e.emit(protocol.ServerTick, payload)
}

func (e *Engine) State() domain.TimerState {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.state.WithRemaining(e.now().UTC())
}

func (e *Engine) Settings() domain.Settings {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.config
}

func (e *Engine) UpdateSettings(ctx context.Context, incoming domain.Settings) (domain.Settings, error) {
	e.mu.Lock()
	if err := incoming.Validate(); err != nil {
		e.mu.Unlock()
		return domain.Settings{}, err
	}
	if err := e.settings.Save(ctx, incoming); err != nil {
		e.mu.Unlock()
		return domain.Settings{}, err
	}
	e.config = incoming

	if e.state.Status != domain.TimerRunning {
		e.state.PhaseDurationSeconds = incoming.SecondsFor(e.state.Phase)
		e.state.RemainingSeconds = e.state.PhaseDurationSeconds
		e.state.PhaseStartedAt = e.now().UTC()
	}
	_ = e.persistLocked(ctx)
	e.mu.Unlock()

	e.publishState()
	return incoming, nil
}

func (e *Engine) Start(ctx context.Context) (domain.TimerState, error) {
	e.mu.Lock()
	now := e.now().UTC()

	switch e.state.Status {
	case domain.TimerRunning:
		state := e.state.WithRemaining(now)
		e.mu.Unlock()
		return state, nil
	case domain.TimerPaused:
		elapsed := e.state.PhaseDurationSeconds - e.state.RemainingSeconds
		if elapsed < 0 {
			elapsed = 0
		}
		e.state.PhaseStartedAt = now.Add(-time.Duration(elapsed) * time.Second)
		e.state.Status = domain.TimerRunning
	default:
		if e.state.RemainingSeconds <= 0 || e.state.RemainingSeconds > e.state.PhaseDurationSeconds {
			e.state.RemainingSeconds = e.state.PhaseDurationSeconds
		}
		e.state.PhaseStartedAt = now
		e.state.Status = domain.TimerRunning
	}

	state := e.state.WithRemaining(now)
	err := e.persistLocked(ctx)
	e.mu.Unlock()

	if err != nil {
		return domain.TimerState{}, err
	}
	e.publishState()
	return state, nil
}

func (e *Engine) Pause(ctx context.Context) (domain.TimerState, error) {
	e.mu.Lock()
	if e.state.Status != domain.TimerRunning {
		state := e.state
		e.mu.Unlock()
		return state, nil
	}
	now := e.now().UTC()
	e.state.RemainingSeconds = e.state.RemainingAt(now)
	e.state.Status = domain.TimerPaused
	state := e.state
	err := e.persistLocked(ctx)
	e.mu.Unlock()

	if err != nil {
		return domain.TimerState{}, err
	}
	e.publishState()
	return state, nil
}

func (e *Engine) Reset(ctx context.Context) (domain.TimerState, error) {
	e.mu.Lock()
	now := e.now().UTC()
	wasRunning := e.state.Status == domain.TimerRunning
	elapsed := e.state.TotalSeconds() - e.state.RemainingAt(now)
	if elapsed < 0 {
		elapsed = 0
	}
	previousPhase := e.state.Phase
	previousTaskID := e.state.ActiveTaskID
	previousTaskTitle := e.activeTaskTitleLocked()

	e.state.Status = domain.TimerIdle
	e.state.RemainingSeconds = e.state.PhaseDurationSeconds
	e.state.PhaseStartedAt = now
	state := e.state
	err := e.persistLocked(ctx)
	e.mu.Unlock()

	if wasRunning && elapsed > 0 {
		e.recordSession(context.Background(), domain.Session{
			TaskID:          previousTaskID,
			TaskTitle:       previousTaskTitle,
			Phase:           previousPhase,
			StartedAt:       now.Add(-time.Duration(elapsed) * time.Second),
			EndedAt:         now,
			DurationSeconds: elapsed,
			Completed:       false,
			Abandoned:       true,
		})
	}

	if err != nil {
		return domain.TimerState{}, err
	}
	e.publishState()
	return state, nil
}

func (e *Engine) Skip(ctx context.Context) (domain.TimerState, error) {
	e.mu.Lock()
	state := e.completePhaseLocked(ctx, CompletionReasonSkipped)
	e.mu.Unlock()

	if state == nil {
		return domain.TimerState{}, domain.ErrInvalidState
	}
	e.publishState()
	return *state, nil
}

func (e *Engine) SetActiveTask(ctx context.Context, taskID *string) error {
	e.mu.Lock()
	err := e.persistActiveTaskLocked(ctx, taskID)
	e.mu.Unlock()

	e.publishState()
	return err
}

func (e *Engine) EnsureActiveTask(ctx context.Context) error {
	e.mu.Lock()
	current := e.state.ActiveTaskID
	e.mu.Unlock()

	if current != nil {
		task, err := e.tasks.Get(ctx, *current)
		if err == nil && !task.IsDone() {
			return nil
		}
		if err != nil && !errors.Is(err, domain.ErrNotFound) {
			return err
		}
	}

	pending, err := e.tasks.ListByStatus(ctx, domain.TaskStatusPending)
	if err != nil {
		return err
	}
	if len(pending) == 0 {
		return e.SetActiveTask(ctx, nil)
	}
	return e.SetActiveTask(ctx, &pending[0].ID)
}

type completionReason string

const (
	CompletionReasonElapsed  completionReason = "elapsed"
	CompletionReasonSkipped  completionReason = "skipped"
)

func (e *Engine) completePhaseLocked(ctx context.Context, reason completionReason) *domain.TimerState {
	now := e.now().UTC()
	previous := e.state
	elapsed := previous.TotalSeconds() - previous.RemainingAt(now)
	if elapsed < 0 {
		elapsed = 0
	}
	completed := previous.Status == domain.TimerRunning && reason == CompletionReasonElapsed

	var completedTask *domain.Task
	if completed {
		e.recordSessionLocked(ctx, domain.Session{
			TaskID:          previous.ActiveTaskID,
			TaskTitle:       e.activeTaskTitleLocked(),
			Phase:           previous.Phase,
			StartedAt:       now.Add(-time.Duration(elapsed) * time.Second),
			EndedAt:         now,
			DurationSeconds: elapsed,
			Completed:       true,
		})

		if previous.Phase == domain.PhaseFocus {
			completedTask = e.creditFocusLocked(ctx, previous.ActiveTaskID)
		}
	}

	nextPhase := e.config.NextPhase(previous.Phase, e.state.CycleCount)
	if previous.Phase == domain.PhaseFocus {
		e.state.CycleCount++
	}

	autoStart := e.config.AutoStartBreaks && nextPhase.IsBreak()
	e.state.Phase = nextPhase
	e.state.PhaseDurationSeconds = e.config.SecondsFor(nextPhase)
	e.state.RemainingSeconds = e.state.PhaseDurationSeconds
	e.state.PhaseStartedAt = now
	if autoStart {
		e.state.Status = domain.TimerRunning
	} else {
		e.state.Status = domain.TimerIdle
	}

	_ = e.persistLocked(ctx)

	e.emit(protocol.ServerPhaseChanged, protocol.PhaseChangedPayload{
		From:       previous.Phase,
		To:         nextPhase,
		Reason:     string(reason),
		AutoStart:  autoStart,
		CycleCount: e.state.CycleCount,
	})

	if completedTask != nil {
		completedAt := time.Now().UTC()
		if completedTask.CompletedAt != nil {
			completedAt = *completedTask.CompletedAt
		}
		e.emit(protocol.ServerTaskCompleted, protocol.TaskCompletedPayload{
			ID:          completedTask.ID,
			Title:       completedTask.Title,
			CompletedAt: completedAt,
		})
		e.emit(protocol.ServerNotify, protocol.NotifyPayload{
			Level:   "success",
			Title:   "Task completed",
			Message: completedTask.Title,
		})
	}

	state := e.state
	return &state
}

func (e *Engine) creditFocusLocked(ctx context.Context, taskID *string) *domain.Task {
	if taskID == nil || *taskID == "" {
		return nil
	}
	task, done, err := e.tasks.RegisterPomodoro(ctx, *taskID)
	if err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			e.logger.Error("register pomodoro", "error", err, "task", *taskID)
		}
		return nil
	}

	e.emitTasks(ctx)

	if done {
		next := e.firstPendingTaskLocked(ctx)
		_ = e.persistActiveTaskLocked(ctx, next)
		return &task
	}
	return nil
}

func (e *Engine) emitTasks(ctx context.Context) {
	tasks, err := e.tasks.List(ctx)
	if err != nil {
		e.logger.Error("list tasks for broadcast", "error", err)
		return
	}
	e.emit(protocol.ServerTasks, tasks)
}

func (e *Engine) firstPendingTaskLocked(ctx context.Context) *string {
	pending, err := e.tasks.ListByStatus(ctx, domain.TaskStatusPending)
	if err != nil || len(pending) == 0 {
		return nil
	}
	return &pending[0].ID
}

func (e *Engine) recordSession(ctx context.Context, session domain.Session) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.recordSessionLocked(ctx, session)
}

func (e *Engine) recordSessionLocked(ctx context.Context, session domain.Session) {
	if err := e.sessions.Create(ctx, &session); err != nil {
		e.logger.Error("record session", "error", err)
	}
}

func (e *Engine) persistLocked(ctx context.Context) error {
	return e.timerRow.Save(ctx, e.state)
}

func (e *Engine) persistActiveTaskLocked(ctx context.Context, taskID *string) error {
	e.state.ActiveTaskID = taskID
	return e.persistLocked(ctx)
}

func (e *Engine) activeTaskTitle() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.activeTaskTitleLocked()
}

func (e *Engine) activeTaskTitleLocked() string {
	if e.state.ActiveTaskID == nil {
		return ""
	}
	task, err := e.tasks.Get(context.Background(), *e.state.ActiveTaskID)
	if err != nil {
		return ""
	}
	return task.Title
}

func (e *Engine) tickPayload() protocol.TickPayload {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.tickPayloadLocked()
}

func (e *Engine) tickPayloadLocked() protocol.TickPayload {
	now := e.now().UTC()
	payload := protocol.TickPayload{
		Phase:                e.state.Phase,
		Status:               e.state.Status,
		RemainingSeconds:     e.state.RemainingAt(now),
		PhaseDurationSeconds: e.state.PhaseDurationSeconds,
		Progress:             e.state.ElapsedFraction(now),
		CycleCount:           e.state.CycleCount,
		ActiveTaskID:         e.state.ActiveTaskID,
		ServerTime:           now,
	}
	if e.state.ActiveTaskID != nil {
		if task, err := e.tasks.Get(context.Background(), *e.state.ActiveTaskID); err == nil {
			payload.ActiveTaskTitle = task.Title
			payload.TaskCompleted = task.CompletedPomodoros
			payload.TaskEstimated = task.EstimatedPomodoros
			payload.TaskProgress = task.Progress()
		}
	}
	return payload
}

func (e *Engine) publishState() {
	e.mu.Lock()
	payload := e.tickPayloadLocked()
	state := e.state.WithRemaining(e.now().UTC())
	e.mu.Unlock()

	e.emit(protocol.ServerTick, payload)
	e.emit(protocol.ServerTimerState, state)
}

func (e *Engine) emit(eventType string, payload any) {
	e.pubMu.RLock()
	fn := e.broadcast
	e.pubMu.RUnlock()
	if fn != nil {
		fn(eventType, payload)
	}
}
