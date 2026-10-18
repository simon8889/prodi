package domain

import "time"

type Phase string

const (
	PhaseFocus      Phase = "focus"
	PhaseShortBreak Phase = "shortBreak"
	PhaseLongBreak  Phase = "longBreak"
)

func (p Phase) IsBreak() bool {
	return p == PhaseShortBreak || p == PhaseLongBreak
}

func (p Phase) Valid() bool {
	switch p {
	case PhaseFocus, PhaseShortBreak, PhaseLongBreak:
		return true
	}
	return false
}

type TimerStatus string

const (
	TimerIdle    TimerStatus = "idle"
	TimerRunning TimerStatus = "running"
	TimerPaused  TimerStatus = "paused"
)

type TimerState struct {
	Phase                Phase       `json:"phase"`
	Status               TimerStatus `json:"status"`
	PhaseStartedAt       time.Time   `json:"phaseStartedAt"`
	PhaseDurationSeconds int         `json:"phaseDurationSeconds"`
	RemainingSeconds     int         `json:"remainingSeconds"`
	CycleCount           int         `json:"cycleCount"`
	ActiveTaskID         *string     `json:"activeTaskId"`
}

func NewTimerState(focusSeconds int) TimerState {
	return TimerState{
		Phase:                PhaseFocus,
		Status:               TimerIdle,
		PhaseDurationSeconds: focusSeconds,
		RemainingSeconds:     focusSeconds,
	}
}

func (s TimerState) Running() bool {
	return s.Status == TimerRunning
}

func (s TimerState) TotalSeconds() int {
	if s.PhaseDurationSeconds <= 0 {
		return s.RemainingSeconds
	}
	return s.PhaseDurationSeconds
}

func (s TimerState) RemainingAt(now time.Time) int {
	if s.Status != TimerRunning {
		return s.RemainingSeconds
	}
	elapsed := int(now.Sub(s.PhaseStartedAt).Seconds())
	if elapsed < 0 {
		elapsed = 0
	}
	remaining := s.PhaseDurationSeconds - elapsed
	if remaining < 0 {
		return 0
	}
	return remaining
}

func (s TimerState) ElapsedFraction(now time.Time) float64 {
	total := s.TotalSeconds()
	if total <= 0 {
		return 0
	}
	elapsed := total - s.RemainingAt(now)
	if elapsed < 0 {
		elapsed = 0
	}
	if elapsed > total {
		return 1
	}
	return float64(elapsed) / float64(total)
}

func (s TimerState) WithRemaining(now time.Time) TimerState {
	s.RemainingSeconds = s.RemainingAt(now)
	return s
}
