package hc

import (
	"errors"
	"time"
)

var (
	contextCanceled            = errors.New("canceled or timeout")
	ErrIgnorable               = errors.New("")
	ErrSkip                    = errors.New("skip")
	ErrUntilExceedMaximumRetry = errors.New("exceed until maximum retry")
)

type ErrUntilAssert struct {
	Interval time.Duration
}

func (e *ErrUntilAssert) Error() string {
	return "until assertion failed"
}
