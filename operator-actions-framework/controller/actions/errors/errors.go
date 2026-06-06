package errors

import (
	"fmt"
	"time"
)

type StopError struct {
	reason error
}

func (e StopError) Error() string {
	return e.reason.Error()
}

func NewStopErrorW(reason error) StopError {
	return StopError{reason}
}

func NewStopError(format string, args ...any) StopError {
	return StopError{
		fmt.Errorf(format, args...),
	}
}

// PauseError stops the action pipeline (like StopError) but signals the
// reconciler to requeue after the given delay instead of using exponential
// backoff.
type PauseError struct {
	reason error
	delay  time.Duration
}

func (e PauseError) Error() string {
	return e.reason.Error()
}

func (e PauseError) Delay() time.Duration {
	return e.delay
}

func NewPauseErrorW(delay time.Duration, reason error) PauseError {
	return PauseError{reason: reason, delay: delay}
}

func NewPauseError(delay time.Duration, format string, args ...any) PauseError {
	return PauseError{
		reason: fmt.Errorf(format, args...),
		delay:  delay,
	}
}
