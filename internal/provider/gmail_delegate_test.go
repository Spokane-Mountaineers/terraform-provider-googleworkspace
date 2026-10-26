package provider

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource"
)

func TestGmailDelegateResource_Schema(t *testing.T) {
	t.Parallel()

	var resp resource.SchemaResponse
	NewGmailDelegateResource().Schema(context.Background(), resource.SchemaRequest{}, &resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics: %v", resp.Diagnostics)
	}
	for _, attr := range []string{"id", "user_email", "delegate_email", "verification_status"} {
		if _, ok := resp.Schema.Attributes[attr]; !ok {
			t.Errorf("resource missing attribute %q", attr)
		}
	}
}

func TestSplitDelegateID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id, user, delegate string
		ok                 bool
	}{
		{"treasurer@example.org/jdoe@example.org", "treasurer@example.org", "jdoe@example.org", true},
		{"treasurer@example.org", "", "", false},
		{"treasurer/jdoe", "", "", false},
		{"/jdoe@example.org", "", "", false},
	}
	for _, tt := range tests {
		user, delegate, ok := splitDelegateID(tt.id)
		if user != tt.user || delegate != tt.delegate || ok != tt.ok {
			t.Errorf("splitDelegateID(%q) = %q, %q, %v; want %q, %q, %v", tt.id, user, delegate, ok, tt.user, tt.delegate, tt.ok)
		}
	}
}

func TestWaitForMailbox(t *testing.T) {
	t.Parallel()

	boom := errors.New("boom")
	tests := []struct {
		name    string
		readyAt int   // call that first reports ready; 0 never does
		err     error // returned by every call
		wantErr bool
		calls   int
	}{
		{name: "already set up", readyAt: 1, calls: 1},
		{name: "set up after polling", readyAt: 3, calls: 3},
		{name: "never set up", wantErr: true},
		{name: "lookup fails", err: boom, wantErr: true, calls: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			calls := 0
			err := waitForMailbox(context.Background(), time.Millisecond, 50*time.Millisecond, func(context.Context) (bool, error) {
				calls++
				return tt.readyAt != 0 && calls >= tt.readyAt, tt.err
			})
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.err != nil && !errors.Is(err, tt.err) {
				t.Errorf("err = %v, want %v", err, tt.err)
			}
			if tt.calls != 0 && calls != tt.calls {
				t.Errorf("calls = %d, want %d", calls, tt.calls)
			}
		})
	}
}

func TestWaitForMailbox_Canceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := waitForMailbox(ctx, time.Hour, time.Hour, func(context.Context) (bool, error) { return false, nil })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}
