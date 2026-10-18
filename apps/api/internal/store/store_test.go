package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"prodi/api/internal/domain"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	database, err := Open(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return database
}

func TestTaskLifecycle(t *testing.T) {
	database := newTestStore(t)
	repo := NewTaskRepository(database)
	ctx := context.Background()

	task := domain.Task{Title: "  Write the engine  ", EstimatedPomodoros: 3}
	if err := repo.Create(ctx, &task); err != nil {
		t.Fatalf("create: %v", err)
	}
	if task.ID == "" {
		t.Fatal("expected generated id")
	}
	if task.Title != "Write the engine" {
		t.Fatalf("expected trimmed title, got %q", task.Title)
	}
	if task.Position != 1 {
		t.Fatalf("expected position 1, got %d", task.Position)
	}

	second := domain.Task{Title: "Wire the display", EstimatedPomodoros: 1}
	if err := repo.Create(ctx, &second); err != nil {
		t.Fatalf("create second: %v", err)
	}
	if second.Position != 2 {
		t.Fatalf("expected position 2, got %d", second.Position)
	}

	fetched, err := repo.Get(ctx, task.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if fetched.RemainingPomodoros() != 3 {
		t.Fatalf("expected 3 remaining, got %d", fetched.RemainingPomodoros())
	}

	if err := repo.Delete(ctx, second.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := repo.Get(ctx, second.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if err := repo.Delete(ctx, "missing"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound on delete, got %v", err)
	}
}

func TestTaskValidation(t *testing.T) {
	database := newTestStore(t)
	repo := NewTaskRepository(database)
	ctx := context.Background()

	blank := domain.Task{Title: "   ", EstimatedPomodoros: 1}
	var validation *domain.ValidationError
	if err := repo.Create(ctx, &blank); !errors.As(err, &validation) {
		t.Fatalf("expected ValidationError, got %v", err)
	}
	if validation.Field != "title" {
		t.Fatalf("expected title field, got %q", validation.Field)
	}

	tooMany := domain.Task{Title: "Too big", EstimatedPomodoros: 999}
	if err := repo.Create(ctx, &tooMany); !errors.As(err, &validation) {
		t.Fatalf("expected ValidationError for pomodoro count, got %v", err)
	}
}

func TestRegisterPomodoroCompletesTask(t *testing.T) {
	database := newTestStore(t)
	repo := NewTaskRepository(database)
	ctx := context.Background()

	task := domain.Task{Title: "Two blocks", EstimatedPomodoros: 2}
	if err := repo.Create(ctx, &task); err != nil {
		t.Fatalf("create: %v", err)
	}

	updated, done, err := repo.RegisterPomodoro(ctx, task.ID)
	if err != nil {
		t.Fatalf("register first: %v", err)
	}
	if done {
		t.Fatal("task should not be done after 1 of 2")
	}
	if updated.CompletedPomodoros != 1 {
		t.Fatalf("expected 1 completed, got %d", updated.CompletedPomodoros)
	}

	updated, done, err = repo.RegisterPomodoro(ctx, task.ID)
	if err != nil {
		t.Fatalf("register second: %v", err)
	}
	if !done {
		t.Fatal("task should be done after 2 of 2")
	}
	if updated.CompletedAt == nil {
		t.Fatal("expected completedAt to be set")
	}

	updated, done, err = repo.RegisterPomodoro(ctx, task.ID)
	if err != nil {
		t.Fatalf("register on done task: %v", err)
	}
	if done {
		t.Fatal("registering on a done task should report done=false")
	}
	if updated.CompletedPomodoros != 2 {
		t.Fatalf("completed count should not exceed estimate, got %d", updated.CompletedPomodoros)
	}
}

func TestReorderTasks(t *testing.T) {
	database := newTestStore(t)
	repo := NewTaskRepository(database)
	ctx := context.Background()

	first := domain.Task{Title: "First", EstimatedPomodoros: 1}
	second := domain.Task{Title: "Second", EstimatedPomodoros: 1}
	third := domain.Task{Title: "Third", EstimatedPomodoros: 1}
	for _, task := range []*domain.Task{&first, &second, &third} {
		if err := repo.Create(ctx, task); err != nil {
			t.Fatalf("create: %v", err)
		}
	}

	if err := repo.Reorder(ctx, []string{third.ID, first.ID, second.ID}); err != nil {
		t.Fatalf("reorder: %v", err)
	}

	tasks, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(tasks))
	}
	if tasks[0].ID != third.ID || tasks[1].ID != first.ID || tasks[2].ID != second.ID {
		t.Fatalf("unexpected order: %s, %s, %s", tasks[0].Title, tasks[1].Title, tasks[2].Title)
	}
}

func TestSettingsRoundTrip(t *testing.T) {
	database := newTestStore(t)
	repo := NewSettingsRepository(database)
	ctx := context.Background()

	defaults, err := repo.Get(ctx)
	if err != nil {
		t.Fatalf("get defaults: %v", err)
	}
	if defaults != domain.DefaultSettings() {
		t.Fatalf("unexpected defaults: %+v", defaults)
	}

	updated := domain.Settings{
		FocusMinutes:         50,
		ShortBreakMinutes:    10,
		LongBreakMinutes:     20,
		LongBreakEveryCycles: 3,
		AutoStartBreaks:      false,
	}
	if err := repo.Save(ctx, updated); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := repo.Get(ctx)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if loaded != updated {
		t.Fatalf("round trip mismatch: %+v", loaded)
	}

	invalid := domain.Settings{FocusMinutes: 0, ShortBreakMinutes: 5, LongBreakMinutes: 15, LongBreakEveryCycles: 4}
	if err := repo.Save(ctx, invalid); err == nil {
		t.Fatal("expected validation error for focusMinutes=0")
	}
}

func TestTimerStateRoundTrip(t *testing.T) {
	database := newTestStore(t)
	repo := NewTimerStateRepository(database)
	tasks := NewTaskRepository(database)
	ctx := context.Background()

	loaded, err := repo.Load(ctx)
	if err != nil {
		t.Fatalf("load seed: %v", err)
	}
	if loaded.Phase != domain.PhaseFocus || loaded.Status != domain.TimerIdle {
		t.Fatalf("unexpected seed state: %+v", loaded)
	}
	if loaded.PhaseDurationSeconds != domain.DefaultSettings().SecondsFor(domain.PhaseFocus) {
		t.Fatalf("unexpected seed duration: %d", loaded.PhaseDurationSeconds)
	}

	task := domain.Task{Title: "Tracked", EstimatedPomodoros: 1}
	if err := tasks.Create(ctx, &task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	state := domain.TimerState{
		Phase:                domain.PhaseLongBreak,
		Status:               domain.TimerPaused,
		PhaseStartedAt:       time.Now().UTC().Truncate(time.Second),
		PhaseDurationSeconds: 900,
		RemainingSeconds:     420,
		CycleCount:           4,
		ActiveTaskID:         &task.ID,
	}
	if err := repo.Save(ctx, state); err != nil {
		t.Fatalf("save: %v", err)
	}

	restored, err := repo.Load(ctx)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if restored.Phase != state.Phase || restored.Status != state.Status {
		t.Fatalf("phase/status mismatch: %+v", restored)
	}
	if restored.RemainingSeconds != 420 || restored.CycleCount != 4 {
		t.Fatalf("value mismatch: %+v", restored)
	}
	if restored.ActiveTaskID == nil || *restored.ActiveTaskID != task.ID {
		t.Fatalf("active task mismatch: %+v", restored.ActiveTaskID)
	}
}

func TestSessionStats(t *testing.T) {
	database := newTestStore(t)
	sessions := NewSessionRepository(database)
	ctx := context.Background()

	task := domain.Task{Title: "Focus block", EstimatedPomodoros: 5}
	if err := NewTaskRepository(database).Create(ctx, &task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	now := time.Now().UTC()

	for i := 0; i < 3; i++ {
		session := domain.Session{
			TaskID:          &task.ID,
			TaskTitle:       "Focus block",
			Phase:           domain.PhaseFocus,
			StartedAt:       now.Add(-time.Duration(i) * time.Hour),
			EndedAt:         now.Add(-time.Duration(i) * time.Hour),
			DurationSeconds: 1500,
			Completed:       true,
		}
		if err := sessions.Create(ctx, &session); err != nil {
			t.Fatalf("create session: %v", err)
		}
	}

	list, err := sessions.ListRecent(ctx, 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(list))
	}
	if list[0].TaskID == nil || *list[0].TaskID != task.ID {
		t.Fatalf("expected task id to be preserved, got %+v", list[0].TaskID)
	}

	stats, err := sessions.Stats(ctx, now)
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if stats.FocusSessions != 3 {
		t.Fatalf("expected 3 focus sessions, got %d", stats.FocusSessions)
	}
	if stats.FocusSeconds != 4500 {
		t.Fatalf("expected 4500 focus seconds, got %d", stats.FocusSeconds)
	}
	if stats.TodaySessions != 3 {
		t.Fatalf("expected 3 sessions today, got %d", stats.TodaySessions)
	}
	if stats.CurrentStreakDays != 1 {
		t.Fatalf("expected streak of 1, got %d", stats.CurrentStreakDays)
	}
	if stats.TasksPending != 1 {
		t.Fatalf("expected 1 pending task in stats, got %d", stats.TasksPending)
	}
}

func TestDeletingTaskDetachesSessionsAndTimerState(t *testing.T) {
	database := newTestStore(t)
	sessions := NewSessionRepository(database)
	tasks := NewTaskRepository(database)
	timerRepo := NewTimerStateRepository(database)
	ctx := context.Background()

	task := domain.Task{Title: "Doomed", EstimatedPomodoros: 1}
	if err := tasks.Create(ctx, &task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	now := time.Now().UTC()
	session := domain.Session{
		TaskID:          &task.ID,
		TaskTitle:       "Doomed",
		Phase:           domain.PhaseFocus,
		StartedAt:       now,
		EndedAt:         now,
		DurationSeconds: 1500,
		Completed:       true,
	}
	if err := sessions.Create(ctx, &session); err != nil {
		t.Fatalf("create session: %v", err)
	}

	if err := timerRepo.Save(ctx, domain.TimerState{
		Phase:                domain.PhaseFocus,
		Status:               domain.TimerIdle,
		PhaseStartedAt:       now,
		PhaseDurationSeconds: 1500,
		RemainingSeconds:     1500,
		ActiveTaskID:         &task.ID,
	}); err != nil {
		t.Fatalf("save timer state: %v", err)
	}

	if err := tasks.Delete(ctx, task.ID); err != nil {
		t.Fatalf("delete task: %v", err)
	}

	list, err := sessions.ListRecent(ctx, 10)
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected the session to survive, got %d", len(list))
	}
	if list[0].TaskID != nil {
		t.Fatalf("expected task id to be nulled, got %+v", list[0].TaskID)
	}
	if list[0].TaskTitle != "Doomed" {
		t.Fatalf("expected the title snapshot to survive, got %q", list[0].TaskTitle)
	}

	state, err := timerRepo.Load(ctx)
	if err != nil {
		t.Fatalf("load timer state: %v", err)
	}
	if state.ActiveTaskID != nil {
		t.Fatalf("expected active task to be nulled, got %+v", state.ActiveTaskID)
	}
}

func TestMigrationsAreIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reopen.db")

	first, err := Open(path)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	tasks := NewTaskRepository(first)
	if err := tasks.Create(context.Background(), &domain.Task{Title: "Persisted", EstimatedPomodoros: 1}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	second, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer second.Close()

	list, err := NewTaskRepository(second).List(context.Background())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || list[0].Title != "Persisted" {
		t.Fatalf("expected the task to survive a reopen, got %+v", list)
	}
}
