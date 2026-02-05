package errors

import (
	"errors"
	"fmt"
)

// StopError is a marker error that the ComponentController uses
// to break out from the action execution loop.
// It can optionally carry a Reason for the condition.
type StopError struct {
	err    error
	Reason string
}

func (e StopError) Error() string {
	return e.err.Error()
}

func (e StopError) Unwrap() error {
	return e.err
}

func NewStopErrorW(err error) StopError {
	return StopError{err: err}
}

func NewStopError(format string, args ...any) StopError {
	return StopError{
		err: fmt.Errorf(format, args...),
	}
}

// NewStopErrorWithReason creates a StopError with a specific condition reason.
// The reason will be used by the reconciler when setting the ProvisioningSucceeded condition.
func NewStopErrorWithReason(reason string, err error) StopError {
	return StopError{
		err:    err,
		Reason: reason,
	}
}

// WrapStopError wraps the error as a StopError if it isn't already one.
// Returns the original error if it's already a StopError.
func WrapStopError(err error) error {
	var stopErr StopError
	if errors.As(err, &stopErr) {
		return err
	}
	return NewStopErrorW(err)
}

// WrapStopErrorWithReason wraps the error as a StopError with a specific reason.
// If err is already a StopError, updates its reason.
func WrapStopErrorWithReason(reason string, err error) error {
	var stopErr StopError
	if errors.As(err, &stopErr) {
		stopErr.Reason = reason
		return stopErr
	}
	return NewStopErrorWithReason(reason, err)
}
