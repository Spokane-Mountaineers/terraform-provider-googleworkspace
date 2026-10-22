package provider

import (
	"context"
	"testing"

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
