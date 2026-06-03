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

func TestSplitMemberID(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		g, m string
		ok   bool
	}{
		"01grp/02mbr": {"01grp", "02mbr", true},
		"noslash":     {"", "", false},
		"/only":       {"", "", false},
		"only/":       {"", "", false},
	}
	for in, want := range cases {
		g, m, ok := splitMemberID(in)
		if g != want.g || m != want.m || ok != want.ok {
			t.Errorf("splitMemberID(%q) = (%q,%q,%v), want (%q,%q,%v)", in, g, m, ok, want.g, want.m, want.ok)
		}
	}
}
