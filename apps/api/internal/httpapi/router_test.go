package httpapi_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"prodi/api/internal/config"
	"prodi/api/internal/domain"
	"prodi/api/internal/httpapi"
	"prodi/api/internal/hub"
	"prodi/api/internal/protocol"
	"prodi/api/internal/store"
	"prodi/api/internal/timer"
)

type harness struct {
	server *httptest.Server
	client *http.Client
}

func (h *harness) wsURL() string {
	return "ws" + strings.TrimPrefix(h.server.URL, "http") + "/ws"
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	path := filepath.Join(t.TempDir(), "api.db")

	database, err := store.Open(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	taskRepo := store.NewTaskRepository(database)
	sessionRepo := store.NewSessionRepository(database)
	timerRepo := store.NewTimerStateRepository(database)
	settingsRepo := store.NewSettingsRepository(database)

	engine := timer.New(database, taskRepo, sessionRepo, timerRepo, settingsRepo, logger)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	if err := engine.Bootstrap(ctx); err != nil {
		t.Fatalf("bootstrap engine: %v", err)
	}

	cfg := config.Load()
	cfg.CORSOrigins = []string{"http://localhost:5173"}

	wsHub := hub.New(logger, nil)
	server := httpapi.New(cfg, database, taskRepo, sessionRepo, settingsRepo, engine, wsHub, logger)
	wsHub.SetHooks(hub.Hooks{
		OnConnect:    server.OnClientConnect,
		OnDisconnect: server.OnClientDisconnect,
	})
	wsHub.SetMessageHandler(server.HandleInbound)
	engine.SetBroadcast(func(eventType string, payload any) {
		wsHub.Broadcast(eventType, payload, time.Now().UTC())
	})

	go wsHub.Run(ctx)
	go engine.Run(ctx)

	ts := httptest.NewServer(server.Routes())
	t.Cleanup(ts.Close)

	return &harness{server: ts, client: ts.Client()}
}

func (h *harness) do(t *testing.T, method, path string, body any) (*http.Response, []byte) {
	t.Helper()

	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = strings.NewReader(string(raw))
	}

	request, err := http.NewRequest(method, h.server.URL+path, reader)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := h.client.Do(request)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer response.Body.Close()

	data, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return response, data
}

func TestHealth(t *testing.T) {
	h := newHarness(t)

	response, body := h.do(t, http.MethodGet, "/api/health", nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.StatusCode, body)
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload["status"] != "ok" {
		t.Fatalf("unexpected status: %v", payload["status"])
	}
}

func TestTaskRESTFlow(t *testing.T) {
	h := newHarness(t)

	response, body := h.do(t, http.MethodPost, "/api/tasks", map[string]any{
		"title":              "Build the floating windows",
		"estimatedPomodoros": 2,
	})
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", response.StatusCode, body)
	}

	var created domain.Task
	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatalf("decode task: %v", err)
	}
	if created.ID == "" || created.Status != domain.TaskStatusPending {
		t.Fatalf("unexpected created task: %+v", created)
	}

	response, body = h.do(t, http.MethodGet, "/api/tasks", nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.StatusCode, body)
	}
	var listed []domain.Task
	if err := json.Unmarshal(body, &listed); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected 1 task, got %d", len(listed))
	}

	response, body = h.do(t, http.MethodPatch, "/api/tasks/"+created.ID, map[string]any{
		"estimatedPomodoros": 5,
	})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.StatusCode, body)
	}
	var updated domain.Task
	if err := json.Unmarshal(body, &updated); err != nil {
		t.Fatalf("decode updated: %v", err)
	}
	if updated.EstimatedPomodoros != 5 {
		t.Fatalf("expected 5 estimated, got %d", updated.EstimatedPomodoros)
	}

	response, body = h.do(t, http.MethodPatch, "/api/tasks/"+created.ID, map[string]any{
		"status": "done",
	})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.StatusCode, body)
	}
	if err := json.Unmarshal(body, &updated); err != nil {
		t.Fatalf("decode completed: %v", err)
	}
	if !updated.IsDone() || updated.CompletedAt == nil {
		t.Fatalf("expected a completed task, got %+v", updated)
	}

	response, _ = h.do(t, http.MethodDelete, "/api/tasks/"+created.ID, nil)
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", response.StatusCode)
	}

	response, body = h.do(t, http.MethodDelete, "/api/tasks/"+created.ID, nil)
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 on a second delete, got %d: %s", response.StatusCode, body)
	}
}

func TestTaskValidationErrors(t *testing.T) {
	h := newHarness(t)

	response, body := h.do(t, http.MethodPost, "/api/tasks", map[string]any{
		"title":              "   ",
		"estimatedPomodoros": 1,
	})
	if response.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", response.StatusCode, body)
	}

	var payload struct {
		Error struct {
			Code  string `json:"code"`
			Field string `json:"field"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if payload.Error.Code != "validation_error" || payload.Error.Field != "title" {
		t.Fatalf("unexpected error payload: %+v", payload.Error)
	}

	response, body = h.do(t, http.MethodPost, "/api/tasks", map[string]any{
		"title":              "Too many",
		"estimatedPomodoros": 500,
	})
	if response.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for a pomodoro count of 500, got %d: %s", response.StatusCode, body)
	}

	response, _ = h.do(t, http.MethodPost, "/api/tasks", nil)
	if response.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for a missing body, got %d", response.StatusCode)
	}
}

func TestUnknownFieldsAreRejected(t *testing.T) {
	h := newHarness(t)

	response, body := h.do(t, http.MethodPost, "/api/tasks", map[string]any{
		"title":      "Typo field",
		"nope":       true,
		"pomodoroes": 3,
	})
	if response.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for an unknown field, got %d: %s", response.StatusCode, body)
	}
}

func TestSettingsRoundTrip(t *testing.T) {
	h := newHarness(t)

	response, body := h.do(t, http.MethodGet, "/api/settings", nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.StatusCode, body)
	}
	var defaults domain.Settings
	if err := json.Unmarshal(body, &defaults); err != nil {
		t.Fatalf("decode defaults: %v", err)
	}
	if defaults != domain.DefaultSettings() {
		t.Fatalf("unexpected defaults: %+v", defaults)
	}

	desired := domain.Settings{
		FocusMinutes:         30,
		ShortBreakMinutes:    4,
		LongBreakMinutes:     12,
		LongBreakEveryCycles: 6,
		AutoStartBreaks:      false,
	}
	response, body = h.do(t, http.MethodPut, "/api/settings", desired)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.StatusCode, body)
	}

	_, body = h.do(t, http.MethodGet, "/api/settings", nil)
	var stored domain.Settings
	if err := json.Unmarshal(body, &stored); err != nil {
		t.Fatalf("decode stored: %v", err)
	}
	if stored != desired {
		t.Fatalf("round trip mismatch: %+v", stored)
	}

	invalid := desired
	invalid.FocusMinutes = 0
	response, _ = h.do(t, http.MethodPut, "/api/settings", invalid)
	if response.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for focusMinutes=0, got %d", response.StatusCode)
	}
}

func TestTimerEndpoints(t *testing.T) {
	h := newHarness(t)

	response, body := h.do(t, http.MethodPost, "/api/timer/start", nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.StatusCode, body)
	}
	var started domain.TimerState
	if err := json.Unmarshal(body, &started); err != nil {
		t.Fatalf("decode start: %v", err)
	}
	if started.Status != domain.TimerRunning {
		t.Fatalf("expected running, got %s", started.Status)
	}

	_, body = h.do(t, http.MethodPost, "/api/timer/pause", nil)
	var paused domain.TimerState
	if err := json.Unmarshal(body, &paused); err != nil {
		t.Fatalf("decode pause: %v", err)
	}
	if paused.Status != domain.TimerPaused {
		t.Fatalf("expected paused, got %s", paused.Status)
	}

	_, body = h.do(t, http.MethodPost, "/api/timer/reset", nil)
	var reset domain.TimerState
	if err := json.Unmarshal(body, &reset); err != nil {
		t.Fatalf("decode reset: %v", err)
	}
	if reset.Status != domain.TimerIdle || reset.RemainingSeconds != reset.PhaseDurationSeconds {
		t.Fatalf("unexpected reset state: %+v", reset)
	}

	_, body = h.do(t, http.MethodPost, "/api/timer/skip", nil)
	var skipped domain.TimerState
	if err := json.Unmarshal(body, &skipped); err != nil {
		t.Fatalf("decode skip: %v", err)
	}
	if skipped.Phase != domain.PhaseShortBreak {
		t.Fatalf("expected a short break after skip, got %s", skipped.Phase)
	}
}

func TestStateSnapshotIncludesEverything(t *testing.T) {
	h := newHarness(t)

	h.do(t, http.MethodPost, "/api/tasks", map[string]any{
		"title":              "Snapshot me",
		"estimatedPomodoros": 3,
	})

	response, body := h.do(t, http.MethodGet, "/api/state", nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.StatusCode, body)
	}

	var snapshot domain.Snapshot
	if err := json.Unmarshal(body, &snapshot); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	if len(snapshot.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(snapshot.Tasks))
	}
	if snapshot.Settings.FocusMinutes != domain.DefaultFocus {
		t.Fatalf("unexpected settings: %+v", snapshot.Settings)
	}
	if snapshot.Timer.Phase != domain.PhaseFocus {
		t.Fatalf("unexpected timer phase: %s", snapshot.Timer.Phase)
	}
	if snapshot.ActiveTask == nil || snapshot.ActiveTask.Title != "Snapshot me" {
		t.Fatalf("expected the only task to be active, got %+v", snapshot.ActiveTask)
	}
	if snapshot.Stats.TasksPending != 1 {
		t.Fatalf("expected 1 pending task in stats, got %d", snapshot.Stats.TasksPending)
	}
	if snapshot.ServerTime.IsZero() {
		t.Fatal("expected serverTime to be set")
	}
}

func TestCORSHeaders(t *testing.T) {
	h := newHarness(t)

	request, err := http.NewRequest(http.MethodGet, h.server.URL+"/api/health", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	request.Header.Set("Origin", "http://localhost:5173")

	response, err := h.client.Do(request)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer response.Body.Close()

	if got := response.Header.Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Fatalf("unexpected CORS origin: %q", got)
	}
}

func TestNotFoundRoute(t *testing.T) {
	h := newHarness(t)

	response, body := h.do(t, http.MethodGet, "/api/nope", nil)
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", response.StatusCode, body)
	}
}

type wsClient struct {
	t      *testing.T
	conn   *websocket.Conn
	messages chan protocol.Envelope
}

func dialWS(t *testing.T, baseURL, query string) *wsClient {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, baseURL+query, nil)
	if err != nil {
		t.Fatalf("dial %s: %v", query, err)
	}
	conn.SetReadLimit(1 << 20)

	client := &wsClient{
		t:        t,
		conn:     conn,
		messages: make(chan protocol.Envelope, 256),
	}

	go func() {
		defer close(client.messages)
		for {
			_, data, err := conn.Read(context.Background())
			if err != nil {
				return
			}
			var envelope protocol.Envelope
			if err := json.Unmarshal(data, &envelope); err != nil {
				continue
			}
			client.messages <- envelope
		}
	}()

	t.Cleanup(func() { conn.CloseNow() })
	return client
}

func (c *wsClient) send(t *testing.T, msgType string, payload any) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	data, err := protocol.Encode(msgType, payload, time.Now().UTC())
	if err != nil {
		t.Fatalf("encode %s: %v", msgType, err)
	}
	if err := c.conn.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatalf("write %s: %v", msgType, err)
	}
}

func (c *wsClient) sendRaw(t *testing.T, data []byte) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := c.conn.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatalf("write raw: %v", err)
	}
}

func (c *wsClient) waitFor(t *testing.T, msgType string) protocol.Envelope {
	t.Helper()
	return c.waitForMatching(t, msgType, func(protocol.Envelope) bool { return true })
}

func (c *wsClient) waitForMatching(t *testing.T, msgType string, match func(protocol.Envelope) bool) protocol.Envelope {
	t.Helper()

	deadline := time.After(12 * time.Second)
	for {
		select {
		case envelope, ok := <-c.messages:
			if !ok {
				t.Fatalf("connection closed while waiting for %s", msgType)
			}
			if envelope.Type == msgType && match(envelope) {
				return envelope
			}
		case <-deadline:
			t.Fatalf("timed out waiting for %s", msgType)
		}
	}
}

func decodeSnapshot(t *testing.T, envelope protocol.Envelope) domain.Snapshot {
	t.Helper()
	var snapshot domain.Snapshot
	if err := json.Unmarshal(envelope.Payload, &snapshot); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	return snapshot
}

func decodeTick(t *testing.T, envelope protocol.Envelope) protocol.TickPayload {
	t.Helper()
	var tick protocol.TickPayload
	if err := json.Unmarshal(envelope.Payload, &tick); err != nil {
		t.Fatalf("decode tick: %v", err)
	}
	return tick
}

func TestWebSocketReceivesHelloAndSnapshot(t *testing.T) {
	h := newHarness(t)

	h.do(t, http.MethodPost, "/api/tasks", map[string]any{
		"title":              "Over the wire",
		"estimatedPomodoros": 1,
	})

	client := dialWS(t, h.wsURL(), "?kind=web&name=test")

	hello := client.waitFor(t, protocol.ServerHello)
	var helloPayload protocol.HelloPayload
	if err := json.Unmarshal(hello.Payload, &helloPayload); err != nil {
		t.Fatalf("decode hello: %v", err)
	}
	if helloPayload.Kind != protocol.ClientKindWeb {
		t.Fatalf("unexpected kind: %s", helloPayload.Kind)
	}
	if helloPayload.ServerTime.IsZero() {
		t.Fatal("expected serverTime in the hello payload")
	}

	snapshot := decodeSnapshot(t, client.waitFor(t, protocol.ServerSnapshot))
	if len(snapshot.Tasks) != 1 {
		t.Fatalf("expected 1 task in the snapshot, got %d", len(snapshot.Tasks))
	}
	if snapshot.Settings.FocusMinutes != domain.DefaultFocus {
		t.Fatalf("unexpected settings in snapshot: %+v", snapshot.Settings)
	}
}

func TestWebSocketTaskCreateBroadcastsToOtherClients(t *testing.T) {
	h := newHarness(t)

	creator := dialWS(t, h.wsURL(), "?kind=web&name=creator")
	observer := dialWS(t, h.wsURL(), "?kind=web&name=observer")

	creator.waitFor(t, protocol.ServerSnapshot)
	observer.waitFor(t, protocol.ServerSnapshot)

	creator.send(t, protocol.ClientTaskCreate, protocol.CreateTaskPayload{
		Title:              "Created from the socket",
		EstimatedPomodoros: 2,
	})

	envelope := observer.waitForMatching(t, protocol.ServerTasks, func(e protocol.Envelope) bool {
		var tasks []domain.Task
		if err := json.Unmarshal(e.Payload, &tasks); err != nil {
			return false
		}
		return len(tasks) == 1
	})

	var tasks []domain.Task
	if err := json.Unmarshal(envelope.Payload, &tasks); err != nil {
		t.Fatalf("decode tasks: %v", err)
	}
	if tasks[0].Title != "Created from the socket" {
		t.Fatalf("unexpected task: %+v", tasks[0])
	}

	created := creator.waitFor(t, "task.created")
	var task domain.Task
	if err := json.Unmarshal(created.Payload, &task); err != nil {
		t.Fatalf("decode created task: %v", err)
	}
	if task.ID != tasks[0].ID {
		t.Fatalf("sender and observer disagree on the task id")
	}
}

func TestWebSocketTimerStartReachesDeviceClient(t *testing.T) {
	h := newHarness(t)

	device := dialWS(t, h.wsURL(), "?kind=device&name=esp32")
	device.waitFor(t, protocol.ServerSnapshot)

	device.send(t, protocol.ClientTimerStart, nil)

	envelope := device.waitForMatching(t, protocol.ServerTick, func(e protocol.Envelope) bool {
		return decodeTick(t, e).Status == domain.TimerRunning
	})

	tick := decodeTick(t, envelope)
	if tick.RemainingSeconds > tick.PhaseDurationSeconds {
		t.Fatalf("remaining cannot exceed the phase duration: %+v", tick)
	}
}

func TestWebSocketUnknownMessageReturnsError(t *testing.T) {
	h := newHarness(t)

	client := dialWS(t, h.wsURL(), "?kind=web")
	client.waitFor(t, protocol.ServerSnapshot)

	client.send(t, "definitely.not.a.command", nil)

	errorEnvelope := client.waitFor(t, protocol.ServerError)
	var payload protocol.ErrorPayload
	if err := json.Unmarshal(errorEnvelope.Payload, &payload); err != nil {
		t.Fatalf("decode error payload: %v", err)
	}
	if payload.Code != "unknown_message" {
		t.Fatalf("unexpected error code: %s", payload.Code)
	}
}

func TestWebSocketRejectsGarbage(t *testing.T) {
	h := newHarness(t)

	client := dialWS(t, h.wsURL(), "?kind=web")
	client.waitFor(t, protocol.ServerSnapshot)

	client.sendRaw(t, []byte("not json"))

	errorEnvelope := client.waitFor(t, protocol.ServerError)
	var payload protocol.ErrorPayload
	if err := json.Unmarshal(errorEnvelope.Payload, &payload); err != nil {
		t.Fatalf("decode error payload: %v", err)
	}
	if payload.Code != "invalid_json" {
		t.Fatalf("unexpected error code: %s", payload.Code)
	}
}

func TestWebSocketPingKeepsConnectionAlive(t *testing.T) {
	h := newHarness(t)

	client := dialWS(t, h.wsURL(), "?kind=web")
	client.waitFor(t, protocol.ServerSnapshot)

	client.send(t, protocol.ClientPing, nil)

	pinged := client.waitForMatching(t, protocol.ServerHello, func(e protocol.Envelope) bool {
		var payload protocol.HelloPayload
		return json.Unmarshal(e.Payload, &payload) == nil
	})
	if pinged.Type != protocol.ServerHello {
		t.Fatalf("expected a hello reply, got %s", pinged.Type)
	}
}

func TestClientCountsIncludeDevices(t *testing.T) {
	h := newHarness(t)

	web := dialWS(t, h.wsURL(), "?kind=web")
	device := dialWS(t, h.wsURL(), "?kind=device")

	web.waitFor(t, protocol.ServerSnapshot)
	device.waitFor(t, protocol.ServerSnapshot)

	counts := waitForCounts(t, h, 1, 1)
	if counts.Total != 2 {
		t.Fatalf("expected 2 total clients, got %+v", counts)
	}
}

func waitForCounts(t *testing.T, h *harness, wantWeb, wantDevices int) hub.ClientCounts {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	var counts hub.ClientCounts
	for time.Now().Before(deadline) {
		response, body := h.do(t, http.MethodGet, "/api/clients", nil)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", response.StatusCode, body)
		}
		if err := json.Unmarshal(body, &counts); err != nil {
			t.Fatalf("decode counts: %v", err)
		}
		if counts.Web == wantWeb && counts.Devices == wantDevices {
			return counts
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for counts, last seen %+v", counts)
	return counts
}
