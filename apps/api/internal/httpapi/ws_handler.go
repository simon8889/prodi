package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/coder/websocket"

	"prodi/api/internal/domain"
	"prodi/api/internal/protocol"
)

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	if !isWebSocketUpgrade(r) {
		writeError(w, http.StatusUpgradeRequired, "upgrade_required", "this endpoint requires a websocket upgrade")
		return
	}

	kind := resolveClientKind(r)
	if !s.authorize(r, kind) {
		writeError(w, http.StatusUnauthorized, "unauthorized", "a valid device token is required")
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"*"},
		CompressionMode: websocket.CompressionContextTakeover,
		InsecureSkipVerify: false,
	})
	if err != nil {
		s.logger.Debug("websocket accept failed", "error", err)
		return
	}

	s.hub.Register(r.Context(), conn, kind, clientLabel(r))
}

func resolveClientKind(r *http.Request) protocol.ClientKind {
	raw := r.URL.Query().Get("kind")
	if protocol.ClientKind(raw) == protocol.ClientKindDevice {
		return protocol.ClientKindDevice
	}
	return protocol.ClientKindWeb
}

func clientLabel(r *http.Request) string {
	if label := r.URL.Query().Get("name"); label != "" {
		return label
	}
	return r.RemoteAddr
}

func (s *Server) authorize(r *http.Request, kind protocol.ClientKind) bool {
	if s.cfg.DeviceToken == "" {
		return true
	}
	if kind != protocol.ClientKindDevice {
		return true
	}
	return r.URL.Query().Get("token") == s.cfg.DeviceToken
}

func (s *Server) HandleInbound(ctx context.Context, clientID uint64, inbound protocol.Inbound) {
	switch inbound.Type {
	case protocol.ClientTimerStart:
		s.runEngineCommand(ctx, clientID, func(c context.Context) error { _, err := s.engine.Start(c); return err })
	case protocol.ClientTimerPause:
		s.runEngineCommand(ctx, clientID, func(c context.Context) error { _, err := s.engine.Pause(c); return err })
	case protocol.ClientTimerReset:
		s.runEngineCommand(ctx, clientID, func(c context.Context) error { _, err := s.engine.Reset(c); return err })
	case protocol.ClientTimerSkip:
		s.runEngineCommand(ctx, clientID, func(c context.Context) error { _, err := s.engine.Skip(c); return err })
	case protocol.ClientTaskCreate:
		s.handleInboundTaskCreate(ctx, clientID, inbound)
	case protocol.ClientTaskSetActive:
		s.handleInboundTaskSetActive(ctx, clientID, inbound)
	case protocol.ClientTaskComplete:
		s.handleInboundTaskComplete(ctx, clientID, inbound)
	case protocol.ClientSettingsUpdate:
		s.handleInboundSettingsUpdate(ctx, clientID, inbound)
	default:
		s.replyError(ctx, clientID, "unknown_message", "unsupported message type: "+inbound.Type)
	}
}

func (s *Server) runEngineCommand(ctx context.Context, clientID uint64, fn func(context.Context) error) {
	if err := fn(ctx); err != nil {
		s.logger.Debug("timer command failed", "error", err, "client", clientID)
		s.replyError(ctx, clientID, "command_failed", err.Error())
	}
}

func (s *Server) handleInboundTaskCreate(ctx context.Context, clientID uint64, inbound protocol.Inbound) {
	var payload protocol.CreateTaskPayload
	if err := json.Unmarshal(nonEmpty(inbound.Payload), &payload); err != nil {
		s.replyError(ctx, clientID, "invalid_payload", "task.create payload is invalid")
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
	if err := s.tasks.Create(ctx, &task); err != nil {
		s.replyError(ctx, clientID, "task_create_failed", err.Error())
		return
	}
	if err := s.engine.EnsureActiveTask(ctx); err != nil {
		s.logger.Error("ensure active task", "error", err)
	}
	s.BroadcastTasks(ctx)
	s.hub.SendTo(ctx, clientID, "task.created", task, time.Now().UTC())
}

func (s *Server) handleInboundTaskSetActive(ctx context.Context, clientID uint64, inbound protocol.Inbound) {
	var payload protocol.SetActiveTaskPayload
	if err := json.Unmarshal(nonEmpty(inbound.Payload), &payload); err != nil {
		s.replyError(ctx, clientID, "invalid_payload", "task.setActive payload is invalid")
		return
	}
	task, err := s.tasks.Get(ctx, payload.ID)
	if err != nil {
		s.replyError(ctx, clientID, "task_not_found", "no task matches that id")
		return
	}
	if err := s.engine.SetActiveTask(ctx, &task.ID); err != nil {
		s.replyError(ctx, clientID, "set_active_failed", err.Error())
	}
}

func (s *Server) handleInboundTaskComplete(ctx context.Context, clientID uint64, inbound protocol.Inbound) {
	var payload protocol.SetActiveTaskPayload
	if err := json.Unmarshal(nonEmpty(inbound.Payload), &payload); err != nil {
		s.replyError(ctx, clientID, "invalid_payload", "task.complete payload is invalid")
		return
	}
	task, err := s.tasks.Get(ctx, payload.ID)
	if err != nil {
		s.replyError(ctx, clientID, "task_not_found", "no task matches that id")
		return
	}

	now := time.Now().UTC()
	task.Status = domain.TaskStatusDone
	task.CompletedAt = &now
	if task.CompletedPomodoros < task.EstimatedPomodoros {
		task.CompletedPomodoros = task.EstimatedPomodoros
	}
	if err := s.tasks.Update(ctx, &task); err != nil {
		s.replyError(ctx, clientID, "task_complete_failed", err.Error())
		return
	}
	if err := s.engine.EnsureActiveTask(ctx); err != nil {
		s.logger.Error("ensure active task", "error", err)
	}
	s.BroadcastTasks(ctx)
	s.hub.Broadcast(protocol.ServerTaskCompleted, protocol.TaskCompletedPayload{
		ID:          task.ID,
		Title:       task.Title,
		CompletedAt: now,
	}, time.Now().UTC())
}

func (s *Server) handleInboundSettingsUpdate(ctx context.Context, clientID uint64, inbound protocol.Inbound) {
	var payload protocol.SettingsPayload
	if err := json.Unmarshal(nonEmpty(inbound.Payload), &payload); err != nil {
		s.replyError(ctx, clientID, "invalid_payload", "settings.update payload is invalid")
		return
	}
	updated, err := s.engine.UpdateSettings(ctx, payload.Settings)
	if err != nil {
		s.replyError(ctx, clientID, "settings_update_failed", err.Error())
		return
	}
	s.hub.Broadcast(protocol.ServerSettings, updated, time.Now().UTC())
}

func (s *Server) OnClientConnect(ctx context.Context, clientID uint64, kind protocol.ClientKind) {
	now := time.Now().UTC()

	if err := s.hub.SendTo(ctx, clientID, protocol.ServerHello, protocol.HelloPayload{
		ClientID:   clientIdentity(clientID),
		Kind:       kind,
		ServerTime: now,
	}, now); err != nil {
		s.logger.Debug("send hello", "error", err)
	}

	snapshot, err := s.Snapshot(ctx)
	if err != nil {
		s.logger.Error("build snapshot for client", "error", err)
		return
	}
	if err := s.hub.SendTo(ctx, clientID, protocol.ServerSnapshot, snapshot, now); err != nil {
		s.logger.Debug("send snapshot", "error", err)
	}
}

func (s *Server) OnClientDisconnect(ctx context.Context, kind protocol.ClientKind) {}

func (s *Server) replyError(ctx context.Context, clientID uint64, code, message string) {
	if err := s.hub.SendTo(ctx, clientID, protocol.ServerError, protocol.ErrorPayload{
		Code:    code,
		Message: message,
	}, time.Now().UTC()); err != nil && !errors.Is(err, context.Canceled) {
		s.logger.Debug("send error", "error", err)
	}
}

func clientIdentity(clientID uint64) string {
	return "client-" + formatUint(clientID)
}

func nonEmpty(payload json.RawMessage) json.RawMessage {
	if len(payload) == 0 {
		return json.RawMessage("{}")
	}
	return payload
}

func formatUint(value uint64) string {
	if value == 0 {
		return "0"
	}
	var buffer [20]byte
	index := len(buffer)
	for value > 0 {
		index--
		buffer[index] = byte('0' + value%10)
		value /= 10
	}
	return string(buffer[index:])
}
