package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// failingServer answers the first failures requests with status, then 200.
func failingServer(t *testing.T, failures int32, status int) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) <= failures {
			w.WriteHeader(status)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func testTransport() *retryTransport {
	return &retryTransport{base: http.DefaultTransport, attempts: 3, delay: time.Millisecond, maxDelay: 5 * time.Millisecond}
}

func TestRetryTransport(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		method     string
		failures   int32
		status     int
		wantStatus int
		wantCalls  int32
	}{
		{"read recovers", http.MethodGet, 2, http.StatusServiceUnavailable, http.StatusOK, 3},
		{"read gives up", http.MethodGet, 5, http.StatusServiceUnavailable, http.StatusServiceUnavailable, 3},
		{"bad gateway retried", http.MethodGet, 1, http.StatusBadGateway, http.StatusOK, 2},
		{"rate limit retried", http.MethodGet, 2, http.StatusTooManyRequests, http.StatusOK, 3},
		{"rate limited write not retried", http.MethodPost, 1, http.StatusTooManyRequests, http.StatusTooManyRequests, 1},
		{"not implemented not retried", http.MethodGet, 1, http.StatusNotImplemented, http.StatusNotImplemented, 1},
		{"not found not retried", http.MethodGet, 1, http.StatusNotFound, http.StatusNotFound, 1},
		{"write not retried", http.MethodPost, 1, http.StatusServiceUnavailable, http.StatusServiceUnavailable, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv, calls := failingServer(t, tt.failures, tt.status)
			req, err := http.NewRequest(tt.method, srv.URL, nil)
			if err != nil {
				t.Fatal(err)
			}
			resp, err := testTransport().RoundTrip(req)
			if err != nil {
				t.Fatalf("RoundTrip: %v", err)
			}
			_ = resp.Body.Close()
			if resp.StatusCode != tt.wantStatus {
				t.Errorf("status = %d, want %d", resp.StatusCode, tt.wantStatus)
			}
			if got := calls.Load(); got != tt.wantCalls {
				t.Errorf("calls = %d, want %d", got, tt.wantCalls)
			}
		})
	}
}

func TestRetryTransport_ContextCanceled(t *testing.T) {
	t.Parallel()

	srv, calls := failingServer(t, 5, http.StatusServiceUnavailable)
	ctx, cancel := context.WithCancel(context.Background())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	tr := testTransport()
	tr.delay, tr.maxDelay = time.Hour, time.Hour
	time.AfterFunc(10*time.Millisecond, cancel)

	resp, err := tr.RoundTrip(req)
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("RoundTrip succeeded after cancel, want error")
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("calls = %d, want 1", got)
	}
}

func TestRetryTransport_Backoff(t *testing.T) {
	t.Parallel()

	tr := &retryTransport{delay: time.Second, maxDelay: 16 * time.Second}
	for attempt, base := range map[int]time.Duration{1: time.Second, 2: 2 * time.Second, 4: 8 * time.Second, 10: 16 * time.Second} {
		if d := tr.backoff(attempt, ""); d < base/2 || d > base {
			t.Errorf("backoff(%d) = %v, want between %v and %v", attempt, d, base/2, base)
		}
	}
	if d := tr.backoff(1, "5"); d != 5*time.Second {
		t.Errorf("backoff with Retry-After 5 = %v, want 5s", d)
	}
	if d := tr.backoff(1, "60"); d != 16*time.Second {
		t.Errorf("backoff with Retry-After 60 = %v, want the 16s cap", d)
	}
	if d := tr.backoff(1, "Wed, 21 Oct 2026 07:28:00 GMT"); d > time.Second {
		t.Errorf("backoff with an HTTP-date Retry-After = %v, want the normal backoff", d)
	}
}
