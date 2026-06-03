package provider

import (
	"context"
	"errors"
	"time"

	"google.golang.org/api/googleapi"
)

// isTransient reports whether err is a retryable Google API error, including the
// 404 a just-created parent returns until it propagates.
func isTransient(err error) bool {
	var ae *googleapi.Error
	if !errors.As(err, &ae) {
		return false
	}
	switch ae.Code {
	case 404, 500, 502, 503:
		return true
	}
	return false
}

// withRetry runs fn, retrying on transient errors with linear backoff. It is used
// for operations that can race a recently-created dependency (e.g. adding a member
// to a brand-new group), not for steady-state reads where a 404 is meaningful.
func withRetry(ctx context.Context, fn func() error) error {
	var err error
	for attempt := 0; attempt < 4; attempt++ {
		if err = fn(); err == nil || !isTransient(err) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(attempt+1) * 2 * time.Second):
		}
	}
	return err
}
