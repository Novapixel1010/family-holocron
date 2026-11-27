package store

import (
	"errors"
	"fmt"
)

var (
	// ErrNotFound is returned when a record does not exist.
	ErrNotFound = errors.New("not found")
	// ErrConflict is returned on uniqueness or state conflicts.
	ErrConflict = errors.New("conflict")
)

// ValidationError describes invalid input, optionally per field.
type ValidationError struct {
	Message string
	Fields  map[string]string
}

func (e *ValidationError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("invalid input: %v", e.Fields)
}

// Invalid builds a ValidationError for a single field.
func Invalid(field, msg string) error {
	return &ValidationError{Message: msg, Fields: map[string]string{field: msg}}
}

// ConflictError wraps ErrConflict with a human message.
func ConflictError(msg string) error { return fmt.Errorf("%w: %s", ErrConflict, msg) }
