package protocol

import (
	"encoding/json"
	"time"

	"prodi/api/internal/domain"
)

const (
	ServerSnapshot      = "snapshot"
	ServerTick          = "tick"
	ServerTimerState    = "timer.state"
	ServerTasks         = "tasks"
	ServerSettings      = "settings"
	ServerTaskCompleted = "task.completed"
	ServerPhaseChanged  = "phase.changed"
	ServerNotify        = "notify"
	ServerHello         = "hello"
	ServerError         = "error"
)

const (
	ClientTimerStart      = "timer.start"
	ClientTimerPause      = "timer.pause"
	ClientTimerReset      = "timer.reset"
	ClientTimerSkip       = "timer.skip"
	ClientTaskCreate      = "task.create"
	ClientTaskSetActive   = "task.setActive"
	ClientTaskComplete    = "task.complete"
	ClientSettingsUpdate  = "settings.update"
	ClientPing            = "ping"
)

type ClientKind string

const (
	ClientKindWeb    ClientKind = "web"
	ClientKindDevice ClientKind = "device"
)

type Envelope struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
	At      time.Time       `json:"at"`
}

type Inbound struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type TickPayload struct {
	Phase                domain.Phase      `json:"phase"`
	Status               domain.TimerStatus `json:"status"`
	RemainingSeconds     int               `json:"remainingSeconds"`
	PhaseDurationSeconds int               `json:"phaseDurationSeconds"`
	Progress             float64           `json:"progress"`
	CycleCount           int               `json:"cycleCount"`
	ActiveTaskID         *string           `json:"activeTaskId"`
	ActiveTaskTitle      string            `json:"activeTaskTitle"`
	TaskCompleted        int               `json:"taskCompletedPomodoros"`
	TaskEstimated        int               `json:"taskEstimatedPomodoros"`
	TaskProgress         float64           `json:"taskProgress"`
	ServerTime           time.Time         `json:"serverTime"`
}

type PhaseChangedPayload struct {
	From       domain.Phase      `json:"from"`
	To         domain.Phase      `json:"to"`
	Reason     string            `json:"reason"`
	AutoStart  bool              `json:"autoStart"`
	CycleCount int               `json:"cycleCount"`
}

type TaskCompletedPayload struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	CompletedAt time.Time `json:"completedAt"`
}

type NotifyPayload struct {
	Level   string `json:"level"`
	Title   string `json:"title"`
	Message string `json:"message"`
}

type HelloPayload struct {
	ClientID  string     `json:"clientId"`
	Kind      ClientKind `json:"kind"`
	ServerTime time.Time `json:"serverTime"`
}

type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type CreateTaskPayload struct {
	Title              string `json:"title"`
	EstimatedPomodoros int    `json:"estimatedPomodoros"`
}

type SetActiveTaskPayload struct {
	ID string `json:"id"`
}

type SettingsPayload struct {
	Settings domain.Settings `json:"settings"`
}

func Encode(eventType string, payload any, at time.Time) ([]byte, error) {
	envelope := Envelope{Type: eventType, At: at}
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		envelope.Payload = raw
	}
	return json.Marshal(envelope)
}
