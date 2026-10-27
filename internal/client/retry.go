package client

import (
	"io"
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"

	"golang.org/x/oauth2"
)

// retryTransport retries reads (GET and HEAD) that fail with a server error or are
// rate limited, so a brief Google outage or a 429 doesn't fail a whole plan. It waits with exponential backoff
// and jitter, or longer if the response asks for it with Retry-After. Writes are
// not retried here; resources retry those themselves where it is safe.
type retryTransport struct {
	base     http.RoundTripper
	attempts int           // total tries, including the first
	delay    time.Duration // first backoff; doubles on each retry
	maxDelay time.Duration // cap on a single backoff
}

// newHTTPClient returns an HTTP client that authorizes requests with ts and
// retries reads that fail with a server error or a 429: up to 7 tries, waiting
// about 1s, 2s, 4s, 8s, 16s, then 16s, so a retry can outlast a per-minute quota.
func newHTTPClient(ts oauth2.TokenSource) *http.Client {
	return &http.Client{Transport: &retryTransport{
		base:     &oauth2.Transport{Source: ts, Base: http.DefaultTransport},
		attempts: 7,
		delay:    time.Second,
		maxDelay: 16 * time.Second,
	}}
}

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		return t.base.RoundTrip(req)
	}
	for attempt := 1; ; attempt++ {
		resp, err := t.base.RoundTrip(req)
		if err != nil || !isRetryable(resp.StatusCode) || attempt >= t.attempts {
			return resp, err
		}
		wait, ok := t.backoff(attempt, resp.Header.Get("Retry-After"))
		if !ok {
			return resp, nil
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()

		timer := time.NewTimer(wait)
		select {
		case <-req.Context().Done():
			timer.Stop()
			return nil, req.Context().Err()
		case <-timer.C:
		}
	}
}

// isRetryable reports whether status is worth retrying: a rate limit or a server
// error. 501 Not Implemented won't change on a retry, so it is left out.
func isRetryable(status int) bool {
	switch status {
	case http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	}
	return false
}

// backoff returns how long to wait after the given failed attempt: delay doubled
// per attempt with up to half of it as random jitter, capped at maxDelay, and at
// least what Retry-After asks for in seconds. When Retry-After asks for longer than
// maxDelay it returns false: retrying sooner than the server allows only adds load,
// so the caller fails instead.
func (t *retryTransport) backoff(attempt int, retryAfter string) (time.Duration, bool) {
	d := t.delay << (attempt - 1)
	if d <= 0 || d > t.maxDelay {
		d = t.maxDelay
	}
	if half := int64(d / 2); half > 0 {
		d = time.Duration(half + rand.Int64N(half+1))
	}
	if secs, err := strconv.Atoi(retryAfter); err == nil && secs > 0 {
		ra := time.Duration(secs) * time.Second
		if ra > t.maxDelay {
			return 0, false
		}
		d = max(d, ra)
	}
	return d, true
}
