package client

import (
	"context"
	"testing"
)

func TestFirstNonEmpty(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   []string
		want string
	}{
		{name: "all empty", in: []string{"", ""}, want: ""},
		{name: "first wins", in: []string{"a", "b"}, want: "a"},
		{name: "skips empty", in: []string{"", "b", "c"}, want: "b"},
		{name: "none", in: nil, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := firstNonEmpty(tt.in...); got != tt.want {
				t.Fatalf("firstNonEmpty(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestNew_StaticToken(t *testing.T) {
	c, err := New(context.Background(), Config{AccessToken: "test-token"})
	if err != nil {
		t.Fatalf("New with access token: %v", err)
	}
	if c.CustomerID != DefaultCustomerID {
		t.Errorf("CustomerID = %q, want %q", c.CustomerID, DefaultCustomerID)
	}
	if c.Directory == nil {
		t.Error("Directory client is nil")
	}
}

func TestNew_RequiresDelegationConfig(t *testing.T) {
	t.Setenv("GOOGLEWORKSPACE_ACCESS_TOKEN", "")
	t.Setenv("GOOGLEWORKSPACE_SERVICE_ACCOUNT", "")
	t.Setenv("GOOGLEWORKSPACE_IMPERSONATED_USER_EMAIL", "")

	if _, err := New(context.Background(), Config{}); err == nil {
		t.Fatal("expected an error when no delegation config is provided")
	}
}
