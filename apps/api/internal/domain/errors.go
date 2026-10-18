package domain

import (
	"errors"
	"fmt"
)

type ValidationError struct {
	Field   string
	Message string
}

func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{Field: field, Message: message}
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

var (
	ErrNotFound       = errors.New("resource not found")
	ErrInvalidState   = errors.New("operation not allowed in current state")
	ErrConflict       = errors.New("resource conflict")
	ErrNoActiveTask   = errors.New("no active task")
	ErrMissingSetting = errors.New("missing settings row")
)
