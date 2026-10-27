package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
)

func TestGroupMemberResource_Schema(t *testing.T) {
	t.Parallel()

	var resp resource.SchemaResponse
	NewGroupMemberResource().Schema(context.Background(), resource.SchemaRequest{}, &resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics: %v", resp.Diagnostics)
	}
	for _, attr := range []string{"id", "group_id", "email", "role", "delivery_settings", "type", "member_id"} {
		if _, ok := resp.Schema.Attributes[attr]; !ok {
			t.Errorf("resource missing attribute %q", attr)
		}
	}
}

func TestMemberKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id, email string
		group     string
		key       string
		ok        bool
	}{
		{"01grp/02mbr", "a@example.org", "01grp", "02mbr", true},
		{"01grp/", "a@example.org", "01grp", "a@example.org", true},
		{"01grp/", "", "", "", false},
		{"noslash", "a@example.org", "", "", false},
		{"/02mbr", "a@example.org", "", "", false},
	}
	for _, tt := range tests {
		g, k, ok := memberKey(tt.id, tt.email)
		if g != tt.group || k != tt.key || ok != tt.ok {
			t.Errorf("memberKey(%q, %q) = (%q, %q, %v), want (%q, %q, %v)", tt.id, tt.email, g, k, ok, tt.group, tt.key, tt.ok)
		}
	}
}
