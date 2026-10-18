package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"prodi/api/internal/config"
	"prodi/api/internal/domain"
	"prodi/api/internal/hub"
	"prodi/api/internal/protocol"
	"prodi/api/internal/store"
	"prodi/api/internal/timer"
)

type Server struct {
	cfg          config.Config
	store        *store.Store
	tasks        *store.TaskRepository
	sessions     *store.SessionRepository
	settingsRepo *store.SettingsRepository
	engine       *timer.Engine
	hub          *hub.Hub
	logger       *slog.Logger
	router       chi.Router
	startedAt    time.Time
}

func New(
	cfg config.Config,
	database *store.Store,
	tasks *store.TaskRepository,
	sessions *store.SessionRepository,
	settingsRepo *store.SettingsRepository,
	engine *timer.Engine,
	wsHub *hub.Hub,
	logger *slog.Logger,
) *Server {
	s := &Server{
		cfg:          cfg,
		store:        database,
		tasks:        tasks,
		sessions:     sessions,
		settingsRepo: settingsRepo,
		engine:       engine,
		hub:          wsHub,
		logger:       logger,
		startedAt:    time.Now().UTC(),
	}
	s.router = s.buildRouter()
	return s
}

func (s *Server) buildRouter() chi.Router {
	r := chi.NewRouter()
	r.Use(requestLogger(s.logger))
	r.Use(recoverer(s.logger))
	r.Use(cors(s.cfg.CORSOrigins))

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "route not found")
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed for this route")
	})

	r.Get(s.cfg.WSPath, s.handleWebSocket)

	r.Route("/api", func(api chi.Router) {
		api.Get("/health", s.handleHealth)
		api.Get("/state", s.handleState)
		api.Get("/clients", s.handleClients)
		api.Get("/stats", s.handleStats)
		api.Get("/sessions", s.handleListSessions)

		api.Route("/tasks", func(tasks chi.Router) {
			tasks.Get("/", s.handleListTasks)
			tasks.Post("/", s.handleCreateTask)
			tasks.Post("/reorder", s.handleReorderTasks)
			tasks.Patch("/{id}", s.handleUpdateTask)
			tasks.Delete("/{id}", s.handleDeleteTask)
		})

		api.Get("/settings", s.handleGetSettings)
		api.Put("/settings", s.handleUpdateSettings)
		api.Patch("/settings", s.handleUpdateSettings)

		api.Post("/timer/start", s.handleTimerStart)
		api.Post("/timer/pause", s.handleTimerPause)
		api.Post("/timer/reset", s.handleTimerReset)
		api.Post("/timer/skip", s.handleTimerSkip)
		api.Post("/timer/active-task", s.handleSetActiveTask)
	})

	return r
}

func (s *Server) Routes() http.Handler {
	return s.router
}

func (s *Server) Snapshot(ctx context.Context) (domain.Snapshot, error) {
	tasks, err := s.tasks.List(ctx)
	if err != nil {
		return domain.Snapshot{}, err
	}

	settings, err := s.settingsRepo.Get(ctx)
	if err != nil && !errors.Is(err, domain.ErrMissingSetting) {
		return domain.Snapshot{}, err
	}
	if errors.Is(err, domain.ErrMissingSetting) {
		settings = domain.DefaultSettings()
	}

	stats, err := s.sessions.Stats(ctx, time.Now().UTC())
	if err != nil {
		return domain.Snapshot{}, err
	}

	sessions, err := s.sessions.ListRecent(ctx, 20)
	if err != nil {
		return domain.Snapshot{}, err
	}

	timerState := s.engine.State()
	snapshot := domain.Snapshot{
		Tasks:      tasks,
		Timer:      timerState,
		Settings:   settings,
		Stats:      stats,
		Sessions:   sessions,
		ServerTime: time.Now().UTC(),
	}

	for i := range tasks {
		if timerState.ActiveTaskID != nil && tasks[i].ID == *timerState.ActiveTaskID {
			active := tasks[i]
			snapshot.ActiveTask = &active
			break
		}
	}

	return snapshot, nil
}

func (s *Server) BroadcastSnapshot(ctx context.Context) {
	snapshot, err := s.Snapshot(ctx)
	if err != nil {
		s.logger.Error("build snapshot", "error", err)
		return
	}
	s.hub.Broadcast(protocol.ServerSnapshot, snapshot, time.Now().UTC())
}

func (s *Server) BroadcastTasks(ctx context.Context) {
	tasks, err := s.tasks.List(ctx)
	if err != nil {
		s.logger.Error("list tasks for broadcast", "error", err)
		return
	}
	s.hub.Broadcast(protocol.ServerTasks, tasks, time.Now().UTC())
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	status := "ok"
	if err := s.store.Ping(r.Context()); err != nil {
		status = "degraded"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":     status,
		"uptime":     time.Since(s.startedAt).Round(time.Second).String(),
		"serverTime": time.Now().UTC(),
		"clients":    s.hub.Counts(),
	})
}

func (s *Server) handleClients(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.hub.Counts())
}

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	snapshot, err := s.Snapshot(r.Context())
	if err != nil {
		s.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}

func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	tasks, err := s.tasks.List(r.Context())
	if err != nil {
		s.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tasks)
}

func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	var payload protocol.CreateTaskPayload
	if err := decodeJSON(r, &payload); err != nil {
		s.writeDomainError(w, err)
		return
	}
	if payload.EstimatedPomodoros == 0 {
		payload.EstimatedPomodoros = 1
	}

	task := domain.Task{
		Title:              payload.Title,
		EstimatedPomodoros: payload.EstimatedPomodoros,
		Status:             domain.TaskStatusPending,
		CreatedAt:          time.Now().UTC(),
	}
	if err := s.tasks.Create(r.Context(), &task); err != nil {
		s.writeDomainError(w, err)
		return
	}

	if err := s.engine.EnsureActiveTask(r.Context()); err != nil {
		s.logger.Error("ensure active task", "error", err)
	}
	s.BroadcastTasks(r.Context())

	writeJSON(w, http.StatusCreated, task)
}

func (s *Server) handleUpdateTask(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	current, err := s.tasks.Get(r.Context(), id)
	if err != nil {
		s.writeDomainError(w, err)
		return
	}

	var payload struct {
		Title              *string `json:"title"`
		EstimatedPomodoros *int    `json:"estimatedPomodoros"`
		CompletedPomodoros *int    `json:"completedPomodoros"`
		Status             *string `json:"status"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		s.writeDomainError(w, err)
		return
	}

	if payload.Title != nil {
		current.Title = *payload.Title
	}
	if payload.EstimatedPomodoros != nil {
		current.EstimatedPomodoros = *payload.EstimatedPomodoros
	}
	if payload.CompletedPomodoros != nil {
		current.CompletedPomodoros = *payload.CompletedPomodoros
	}
	if payload.Status != nil {
		status := domain.TaskStatus(*payload.Status)
		if status != domain.TaskStatusPending && status != domain.TaskStatusDone {
			s.writeDomainError(w, domain.NewValidationError("status", "must be pending or done"))
			return
		}
		current.Status = status
		now := time.Now().UTC()
		if status == domain.TaskStatusDone {
			current.CompletedAt = &now
			if current.CompletedPomodoros < current.EstimatedPomodoros {
				current.CompletedPomodoros = current.EstimatedPomodoros
			}
		} else {
			current.CompletedAt = nil
			if current.CompletedPomodoros > current.EstimatedPomodoros {
				current.CompletedPomodoros = current.EstimatedPomodoros
			}
		}
	}

	if err := s.tasks.Update(r.Context(), &current); err != nil {
		s.writeDomainError(w, err)
		return
	}

	if err := s.engine.EnsureActiveTask(r.Context()); err != nil {
		s.logger.Error("ensure active task", "error", err)
	}
	s.BroadcastTasks(r.Context())

	writeJSON(w, http.StatusOK, current)
}

func (s *Server) handleDeleteTask(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.tasks.Delete(r.Context(), id); err != nil {
		s.writeDomainError(w, err)
		return
	}
	if err := s.engine.EnsureActiveTask(r.Context()); err != nil {
		s.logger.Error("ensure active task", "error", err)
	}
	s.BroadcastTasks(r.Context())
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleReorderTasks(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		IDs []string `json:"ids"`
	}
	if err := decodeJSON(r, &payload); err != nil {
		s.writeDomainError(w, err)
		return
	}
	if err := s.tasks.Reorder(r.Context(), payload.IDs); err != nil {
		s.writeDomainError(w, err)
		return
	}
	s.BroadcastTasks(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	limit := 25
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit > 200 {
		limit = 200
	}

	sessions, err := s.sessions.ListRecent(r.Context(), limit)
	if err != nil {
		s.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sessions)
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.sessions.Stats(r.Context(), time.Now().UTC())
	if err != nil {
		s.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.settingsRepo.Get(r.Context())
	if err != nil {
		if errors.Is(err, domain.ErrMissingSetting) {
			writeJSON(w, http.StatusOK, domain.DefaultSettings())
			return
		}
		s.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	incoming := domain.DefaultSettings()
	if err := decodeJSON(r, &incoming); err != nil {
		s.writeDomainError(w, err)
		return
	}

	updated, err := s.engine.UpdateSettings(r.Context(), incoming)
	if err != nil {
		s.writeDomainError(w, err)
		return
	}

	s.hub.Broadcast(protocol.ServerSettings, updated, time.Now().UTC())
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) handleTimerStart(w http.ResponseWriter, r *http.Request) {
	state, err := s.engine.Start(r.Context())
	if err != nil {
		s.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, state)
}

func (s *Server) handleTimerPause(w http.ResponseWriter, r *http.Request) {
	state, err := s.engine.Pause(r.Context())
	if err != nil {
		s.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, state)
}

func (s *Server) handleTimerReset(w http.ResponseWriter, r *http.Request) {
	state, err := s.engine.Reset(r.Context())
	if err != nil {
		s.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, state)
}

func (s *Server) handleTimerSkip(w http.ResponseWriter, r *http.Request) {
	state, err := s.engine.Skip(r.Context())
	if err != nil {
		s.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, state)
}

func (s *Server) handleSetActiveTask(w http.ResponseWriter, r *http.Request) {
	var payload protocol.SetActiveTaskPayload
	if err := decodeJSON(r, &payload); err != nil {
		s.writeDomainError(w, err)
		return
	}

	var taskID *string
	if payload.ID != "" {
		task, err := s.tasks.Get(r.Context(), payload.ID)
		if err != nil {
			s.writeDomainError(w, err)
			return
		}
		taskID = &task.ID
	}

	if err := s.engine.SetActiveTask(r.Context(), taskID); err != nil {
		s.writeDomainError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.engine.State())
}

func (s *Server) writeDomainError(w http.ResponseWriter, err error) {
	var validation *domain.ValidationError
	switch {
	case errors.As(err, &validation):
		writeFieldError(w, http.StatusUnprocessableEntity, "validation_error", validation.Error(), validation.Field)
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "resource not found")
	case errors.Is(err, domain.ErrInvalidState):
		writeError(w, http.StatusConflict, "invalid_state", err.Error())
	case errors.Is(err, domain.ErrConflict):
		writeError(w, http.StatusConflict, "conflict", err.Error())
	default:
		s.logger.Error("request failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "an unexpected error occurred")
	}
}
