package domain

import "time"

type Session struct {
	ID              string    `json:"id"`
	TaskID          *string   `json:"taskId"`
	TaskTitle       string    `json:"taskTitle"`
	Phase           Phase     `json:"phase"`
	StartedAt       time.Time `json:"startedAt"`
	EndedAt         time.Time `json:"endedAt"`
	DurationSeconds int       `json:"durationSeconds"`
	Completed       bool      `json:"completed"`
	Abandoned       bool      `json:"abandoned"`
}

type Stats struct {
	FocusSessions     int        `json:"focusSessions"`
	FocusSeconds      int        `json:"focusSeconds"`
	TasksCompleted    int        `json:"tasksCompleted"`
	TasksTotal        int        `json:"tasksTotal"`
	TasksPending      int        `json:"tasksPending"`
	CurrentStreakDays int        `json:"currentStreakDays"`
	TodayFocusSeconds int        `json:"todayFocusSeconds"`
	TodaySessions     int        `json:"todaySessions"`
	GeneratedAt       time.Time  `json:"generatedAt"`
}

func EmptyStats() Stats {
	return Stats{GeneratedAt: time.Now().UTC()}
}

type Snapshot struct {
	Tasks      []Task      `json:"tasks"`
	Timer      TimerState  `json:"timer"`
	Settings   Settings    `json:"settings"`
	Stats      Stats       `json:"stats"`
	Sessions   []Session   `json:"sessions"`
	ServerTime time.Time   `json:"serverTime"`
	ActiveTask *Task       `json:"activeTask,omitempty"`
}
