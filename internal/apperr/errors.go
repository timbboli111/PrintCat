// Package apperr defines errors whose messages are safe to show to a PrintCat user.
package apperr

import "fmt"

// Error identifies an operation failure without exposing platform implementation details.
type Error struct {
	Operation string
	Err       error
}

func (e *Error) Error() string {
	if e.Err == nil {
		return e.Operation
	}
	return fmt.Sprintf("%s: %v", e.Operation, e.Err)
}

func (e *Error) Unwrap() error { return e.Err }

// Wrap adds operation context. A nil error remains nil.
func Wrap(operation string, err error) error {
	if err == nil {
		return nil
	}
	return &Error{Operation: operation, Err: err}
}
