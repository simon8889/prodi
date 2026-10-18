package domain

const (
	MinMinutes        = 1
	MaxMinutes        = 180
	MinCycles         = 2
	MaxCycles         = 12
	DefaultFocus      = 25
	DefaultShortBreak = 5
	DefaultLongBreak  = 15
	DefaultCycleLen   = 4
)

type Settings struct {
	FocusMinutes         int  `json:"focusMinutes"`
	ShortBreakMinutes    int  `json:"shortBreakMinutes"`
	LongBreakMinutes     int  `json:"longBreakMinutes"`
	LongBreakEveryCycles int  `json:"longBreakEveryCycles"`
	AutoStartBreaks      bool `json:"autoStartBreaks"`
}

func DefaultSettings() Settings {
	return Settings{
		FocusMinutes:         DefaultFocus,
		ShortBreakMinutes:    DefaultShortBreak,
		LongBreakMinutes:     DefaultLongBreak,
		LongBreakEveryCycles: DefaultCycleLen,
		AutoStartBreaks:      true,
	}
}

func (s Settings) Normalize() Settings {
	if s.FocusMinutes < MinMinutes || s.FocusMinutes > MaxMinutes {
		s.FocusMinutes = DefaultFocus
	}
	if s.ShortBreakMinutes < MinMinutes || s.ShortBreakMinutes > MaxMinutes {
		s.ShortBreakMinutes = DefaultShortBreak
	}
	if s.LongBreakMinutes < MinMinutes || s.LongBreakMinutes > MaxMinutes {
		s.LongBreakMinutes = DefaultLongBreak
	}
	if s.LongBreakEveryCycles < MinCycles || s.LongBreakEveryCycles > MaxCycles {
		s.LongBreakEveryCycles = DefaultCycleLen
	}
	return s
}

func (s Settings) Validate() error {
	if s.FocusMinutes < MinMinutes || s.FocusMinutes > MaxMinutes {
		return NewValidationError("focusMinutes", "must be between 1 and 180")
	}
	if s.ShortBreakMinutes < MinMinutes || s.ShortBreakMinutes > MaxMinutes {
		return NewValidationError("shortBreakMinutes", "must be between 1 and 180")
	}
	if s.LongBreakMinutes < MinMinutes || s.LongBreakMinutes > MaxMinutes {
		return NewValidationError("longBreakMinutes", "must be between 1 and 180")
	}
	if s.LongBreakEveryCycles < MinCycles || s.LongBreakEveryCycles > MaxCycles {
		return NewValidationError("longBreakEveryCycles", "must be between 2 and 12")
	}
	return nil
}

func (s Settings) SecondsFor(phase Phase) int {
	switch phase {
	case PhaseFocus:
		return s.FocusMinutes * 60
	case PhaseShortBreak:
		return s.ShortBreakMinutes * 60
	case PhaseLongBreak:
		return s.LongBreakMinutes * 60
	}
	return s.FocusMinutes * 60
}

func (s Settings) NextPhase(current Phase, cycleCount int) Phase {
	if !current.IsBreak() {
		if cycleCount > 0 && cycleCount%s.LongBreakEveryCycles == 0 {
			return PhaseLongBreak
		}
		return PhaseShortBreak
	}
	return PhaseFocus
}
