package domain

import (
	"strings"
	"time"
)

type TaskStatus string

const (
	TaskStatusPending TaskStatus = "pending"
	TaskStatusDone    TaskStatus = "done"
)

type Task struct {
	ID                 string     `json:"id"`
	Title              string     `json:"title"`
	EstimatedPomodoros int        `json:"estimatedPomodoros"`
	CompletedPomodoros int        `json:"completedPomodoros"`
	Status             TaskStatus `json:"status"`
	Position           int        `json:"position"`
	CreatedAt          time.Time  `json:"createdAt"`
	CompletedAt        *time.Time `json:"completedAt,omitempty"`
}

const (
	MaxTitleLength        = 200
	MaxEstimatedPomodoros = 24
)

func (t *Task) Normalize() {
	t.Title = strings.TrimSpace(t.Title)
	if t.EstimatedPomodoros == 0 {
		t.EstimatedPomodoros = 1
	}
	if t.CompletedPomodoros < 0 {
		t.CompletedPomodoros = 0
	}
	if t.Status == "" {
		t.Status = TaskStatusPending
	}
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now().UTC()
	}
	t.CreatedAt = t.CreatedAt.UTC()
	if t.CompletedAt != nil {
		utc := t.CompletedAt.UTC()
		t.CompletedAt = &utc
	}
}

func (t Task) Validate() error {
	if strings.TrimSpace(t.Title) == "" {
		return NewValidationError("title", "must not be empty")
	}
	if len(t.Title) > MaxTitleLength {
		return NewValidationError("title", "must be at most 200 characters")
	}
	if t.EstimatedPomodoros < 1 {
		return NewValidationError("estimatedPomodoros", "must be at least 1")
	}
	if t.EstimatedPomodoros > MaxEstimatedPomodoros {
		return NewValidationError("estimatedPomodoros", "must be at most 24")
	}
	return nil
}

func (t Task) IsDone() bool {
	return t.Status == TaskStatusDone
}

func (t Task) RemainingPomodoros() int {
	if t.CompletedPomodoros >= t.EstimatedPomodoros {
		return 0
	}
	return t.EstimatedPomodoros - t.CompletedPomodoros
}

func (t Task) Progress() float64 {
	if t.EstimatedPomodoros <= 0 {
		return 0
	}
	ratio := float64(t.CompletedPomodoros) / float64(t.EstimatedPomodoros)
	if ratio > 1 {
		return 1
	}
	return ratio
}
