package timer

import (
	"context"
	"log/slog"
	"io"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"prodi/api/internal/domain"
	"prodi/api/internal/protocol"
	"prodi/api/internal/store"
)

type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Date(2026, 3, 14, 9, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

type recordedEvent struct {
	Type    string
	Payload any
}

type recorder struct {
	mu     sync.Mutex
	events []recordedEvent
}

func (r *recorder) record(eventType string, payload any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, recordedEvent{Type: eventType, Payload: payload})
}

func (r *recorder) types() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, 0, len(r.events))
	for _, event := range r.events {
		out = append(out, event.Type)
	}
	return out
}

func (r *recorder) find(eventType string) (any, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := len(r.events) - 1; i >= 0; i-- {
		if r.events[i].Type == eventType {
			return r.events[i].Payload, true
		}
	}
	return nil, false
}

func (r *recorder) count(eventType string) int {
	total := 0
	for _, t := range r.types() {
		if t == eventType {
			total++
		}
	}
	return total
}

func newTestEngine(t *testing.T, clock *fakeClock) (*Engine, *store.TaskRepository, *store.SessionRepository, *store.SettingsRepository, *recorder) {
	t.Helper()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	path := filepath.Join(t.TempDir(), "engine.db")
	database, err := store.Open(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	tasks := store.NewTaskRepository(database)
	sessions := store.NewSessionRepository(database)
	timerRepo := store.NewTimerStateRepository(database)
	settingsRepo := store.NewSettingsRepository(database)

	engine := New(database, tasks, sessions, timerRepo, settingsRepo, logger)
	engine.now = clock.Now

	rec := &recorder{}
	engine.SetBroadcast(rec.record)

	if err := engine.Bootstrap(context.Background()); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	return engine, tasks, sessions, settingsRepo, rec
}

func TestBootstrapIdleState(t *testing.T) {
	clock := newFakeClock()
	engine, _, _, _, _ := newTestEngine(t, clock)

	state := engine.State()
	if state.Phase != domain.PhaseFocus {
		t.Fatalf("expected focus phase, got %s", state.Phase)
	}
	if state.Status != domain.TimerIdle {
		t.Fatalf("expected idle status, got %s", state.Status)
	}
	if state.PhaseDurationSeconds != domain.DefaultFocus*60 {
		t.Fatalf("expected %d seconds, got %d", domain.DefaultFocus*60, state.PhaseDurationSeconds)
	}
	if state.RemainingSeconds != state.PhaseDurationSeconds {
		t.Fatalf("remaining should equal duration when idle")
	}
}

func TestStartPauseResume(t *testing.T) {
	clock := newFakeClock()
	engine, _, _, _, _ := newTestEngine(t, clock)
	ctx := context.Background()

	if _, err := engine.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	if !engine.State().Running() {
		t.Fatal("expected running state")
	}

	clock.Advance(5 * time.Second)
	engine.Tick()
	if got := engine.State().RemainingSeconds; got != domain.DefaultFocus*60-5 {
		t.Fatalf("expected %d remaining, got %d", domain.DefaultFocus*60-5, got)
	}

	if _, err := engine.Pause(ctx); err != nil {
		t.Fatalf("pause: %v", err)
	}
	paused := engine.State()
	if paused.Status != domain.TimerPaused {
		t.Fatalf("expected paused, got %s", paused.Status)
	}
	if paused.RemainingSeconds != domain.DefaultFocus*60-5 {
		t.Fatalf("pause should freeze remaining at %d, got %d", domain.DefaultFocus*60-5, paused.RemainingSeconds)
	}

	clock.Advance(60 * time.Second)
	engine.Tick()
	if got := engine.State().RemainingSeconds; got != domain.DefaultFocus*60-5 {
		t.Fatalf("paused timer must not advance, got %d", got)
	}

	if _, err := engine.Start(ctx); err != nil {
		t.Fatalf("resume: %v", err)
	}
	clock.Advance(3 * time.Second)
	engine.Tick()
	if got := engine.State().RemainingSeconds; got != domain.DefaultFocus*60-8 {
		t.Fatalf("expected %d remaining after resume, got %d", domain.DefaultFocus*60-8, got)
	}
}

func TestFocusCompletionCreditsActiveTask(t *testing.T) {
	clock := newFakeClock()
	engine, tasks, _, _, rec := newTestEngine(t, clock)
	ctx := context.Background()

	task := domain.Task{Title: "Ship the dashboard", EstimatedPomodoros: 2}
	if err := tasks.Create(ctx, &task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := engine.EnsureActiveTask(ctx); err != nil {
		t.Fatalf("ensure active task: %v", err)
	}
	if engine.State().ActiveTaskID == nil || *engine.State().ActiveTaskID != task.ID {
		t.Fatal("expected the created task to become active")
	}

	if _, err := engine.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}

	clock.Advance(time.Duration(domain.DefaultFocus*60) * time.Second)
	engine.Tick()

	updated, err := tasks.Get(ctx, task.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if updated.CompletedPomodoros != 1 {
		t.Fatalf("expected 1 completed pomodoro, got %d", updated.CompletedPomodoros)
	}
	if updated.IsDone() {
		t.Fatal("task should not be done after 1 of 2")
	}

	state := engine.State()
	if state.Phase != domain.PhaseShortBreak {
		t.Fatalf("expected short break after focus, got %s", state.Phase)
	}
	if state.CycleCount != 1 {
		t.Fatalf("expected cycle count 1, got %d", state.CycleCount)
	}
	if !state.Running() {
		t.Fatal("expected autoStartBreaks to start the break")
	}

	clock.Advance(time.Duration(domain.DefaultShortBreak*60) * time.Second)
	engine.Tick()

	afterBreak := engine.State()
	if afterBreak.Phase != domain.PhaseFocus {
		t.Fatalf("expected focus after break, got %s", afterBreak.Phase)
	}
	if afterBreak.Status != domain.TimerIdle {
		t.Fatalf("focus after a break should wait for the user, got %s", afterBreak.Status)
	}

	stillPending, err := tasks.Get(ctx, task.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if stillPending.IsDone() {
		t.Fatal("a break must not complete the task")
	}

	if _, err := engine.Start(ctx); err != nil {
		t.Fatalf("start second focus: %v", err)
	}
	clock.Advance(time.Duration(domain.DefaultFocus*60) * time.Second)
	engine.Tick()

	updated, err = tasks.Get(ctx, task.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if !updated.IsDone() {
		t.Fatal("task should be done after 2 of 2")
	}
	if updated.CompletedPomodoros != 2 {
		t.Fatalf("expected 2 completed pomodoros, got %d", updated.CompletedPomodoros)
	}
	if rec.count(protocol.ServerTaskCompleted) != 1 {
		t.Fatalf("expected exactly one task.completed event, got %d", rec.count(protocol.ServerTaskCompleted))
	}

	completed, ok := rec.find(protocol.ServerTaskCompleted)
	if !ok {
		t.Fatal("expected a task.completed payload")
	}
	if completed.(protocol.TaskCompletedPayload).Title != "Ship the dashboard" {
		t.Fatalf("unexpected payload: %+v", completed)
	}

	final := engine.State()
	if final.Phase != domain.PhaseShortBreak {
		t.Fatalf("expected a break after the second focus, got %s", final.Phase)
	}
	if final.ActiveTaskID != nil {
		t.Fatalf("expected the active task to move on after completion, got %+v", final.ActiveTaskID)
	}
}

func TestLongBreakEveryConfiguredCycles(t *testing.T) {
	clock := newFakeClock()
	engine, _, _, settingsRepo, _ := newTestEngine(t, clock)
	ctx := context.Background()

	updated := domain.DefaultSettings()
	updated.LongBreakEveryCycles = 2
	updated.AutoStartBreaks = true
	if _, err := engine.UpdateSettings(ctx, updated); err != nil {
		t.Fatalf("update settings: %v", err)
	}
	if settingsRepo == nil {
		t.Fatal("unexpected nil settings repository")
	}

	seen := map[domain.Phase]bool{}
	for i := 0; i < 3; i++ {
		if _, err := engine.Start(ctx); err != nil {
			t.Fatalf("start %d: %v", i, err)
		}
		clock.Advance(time.Duration(domain.DefaultFocus*60) * time.Second)
		engine.Tick()
		seen[engine.State().Phase] = true

		if engine.State().Phase == domain.PhaseLongBreak {
			clock.Advance(time.Duration(domain.DefaultLongBreak*60) * time.Second)
			engine.Tick()
		} else {
			clock.Advance(time.Duration(domain.DefaultShortBreak*60) * time.Second)
			engine.Tick()
		}
	}

	if !seen[domain.PhaseLongBreak] {
		t.Fatalf("expected a long break within 3 cycles, saw %v", seen)
	}
}

func TestSkipMovesToBreakWithoutCreditingTask(t *testing.T) {
	clock := newFakeClock()
	engine, tasks, _, _, _ := newTestEngine(t, clock)
	ctx := context.Background()

	task := domain.Task{Title: "Not credited", EstimatedPomodoros: 1}
	if err := tasks.Create(ctx, &task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := engine.EnsureActiveTask(ctx); err != nil {
		t.Fatalf("ensure active task: %v", err)
	}
	if _, err := engine.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}

	if _, err := engine.Skip(ctx); err != nil {
		t.Fatalf("skip: %v", err)
	}

	updated, err := tasks.Get(ctx, task.ID)
	if err != nil {
		t.Fatalf("get task: %v", err)
	}
	if updated.CompletedPomodoros != 0 {
		t.Fatalf("skipping must not credit the task, got %d", updated.CompletedPomodoros)
	}
	if engine.State().Phase != domain.PhaseShortBreak {
		t.Fatalf("expected short break, got %s", engine.State().Phase)
	}
}

func TestResetRecordsAbandonedSession(t *testing.T) {
	clock := newFakeClock()
	engine, _, sessions, _, _ := newTestEngine(t, clock)
	ctx := context.Background()

	if _, err := engine.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	clock.Advance(120 * time.Second)
	if _, err := engine.Reset(ctx); err != nil {
		t.Fatalf("reset: %v", err)
	}

	state := engine.State()
	if state.Status != domain.TimerIdle {
		t.Fatalf("expected idle after reset, got %s", state.Status)
	}
	if state.RemainingSeconds != state.PhaseDurationSeconds {
		t.Fatalf("reset should restore the full duration, got %d", state.RemainingSeconds)
	}

	list, err := sessions.ListRecent(ctx, 5)
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 abandoned session, got %d", len(list))
	}
	if !list[0].Abandoned || list[0].Completed {
		t.Fatalf("expected an abandoned session, got %+v", list[0])
	}
	if list[0].DurationSeconds != 120 {
		t.Fatalf("expected 120 elapsed seconds, got %d", list[0].DurationSeconds)
	}
}

func TestBootstrapPausesARunningTimerAfterRestart(t *testing.T) {
	clock := newFakeClock()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	path := filepath.Join(t.TempDir(), "restart.db")

	database, err := store.Open(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	tasks := store.NewTaskRepository(database)
	sessions := store.NewSessionRepository(database)
	timerRepo := store.NewTimerStateRepository(database)
	settingsRepo := store.NewSettingsRepository(database)

	first := New(database, tasks, sessions, timerRepo, settingsRepo, logger)
	first.now = clock.Now
	if err := first.Bootstrap(context.Background()); err != nil {
		t.Fatalf("bootstrap first: %v", err)
	}
	if _, err := first.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	clock.Advance(300 * time.Second)
	if err := database.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	reopened, err := store.Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer reopened.Close()

	second := New(
		reopened,
		store.NewTaskRepository(reopened),
		store.NewSessionRepository(reopened),
		store.NewTimerStateRepository(reopened),
		store.NewSettingsRepository(reopened),
		logger,
	)
	second.now = clock.Now
	if err := second.Bootstrap(context.Background()); err != nil {
		t.Fatalf("bootstrap second: %v", err)
	}

	state := second.State()
	if state.Status != domain.TimerPaused {
		t.Fatalf("a restarted running timer should pause, got %s", state.Status)
	}
	if state.RemainingSeconds != domain.DefaultFocus*60-300 {
		t.Fatalf("expected %d remaining after restart, got %d", domain.DefaultFocus*60-300, state.RemainingSeconds)
	}
}

func TestSettingsChangeResizesIdleTimer(t *testing.T) {
	clock := newFakeClock()
	engine, _, _, _, _ := newTestEngine(t, clock)
	ctx := context.Background()

	updated := domain.DefaultSettings()
	updated.FocusMinutes = 45
	if _, err := engine.UpdateSettings(ctx, updated); err != nil {
		t.Fatalf("update settings: %v", err)
	}

	state := engine.State()
	if state.PhaseDurationSeconds != 2700 {
		t.Fatalf("expected 2700 seconds, got %d", state.PhaseDurationSeconds)
	}

	if _, err := engine.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	running := engine.Settings()
	running.FocusMinutes = 50
	if _, err := engine.UpdateSettings(ctx, running); err != nil {
		t.Fatalf("update settings while running: %v", err)
	}
	if got := engine.State().PhaseDurationSeconds; got != 2700 {
		t.Fatalf("a running timer must keep its duration, got %d", got)
	}
}

func TestTickPayloadCarriesActiveTask(t *testing.T) {
	clock := newFakeClock()
	engine, tasks, _, _, rec := newTestEngine(t, clock)
	ctx := context.Background()

	task := domain.Task{Title: "Mirror to the OLED", EstimatedPomodoros: 4}
	if err := tasks.Create(ctx, &task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	if err := engine.EnsureActiveTask(ctx); err != nil {
		t.Fatalf("ensure active task: %v", err)
	}

	engine.Tick()

	payload, ok := rec.find(protocol.ServerTick)
	if !ok {
		t.Fatal("expected a tick event")
	}
	tick := payload.(protocol.TickPayload)
	if tick.ActiveTaskTitle != "Mirror to the OLED" {
		t.Fatalf("unexpected active task title: %q", tick.ActiveTaskTitle)
	}
	if tick.TaskEstimated != 4 || tick.TaskCompleted != 0 {
		t.Fatalf("unexpected task counters: %+v", tick)
	}
	if tick.Progress != 0 {
		t.Fatalf("expected zero progress on an idle timer, got %f", tick.Progress)
	}
}
