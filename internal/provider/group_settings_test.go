package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"google.golang.org/api/groupssettings/v1"
)

func TestGroupSettingsResource_Schema(t *testing.T) {
	t.Parallel()

	var resp resource.SchemaResponse
	NewGroupSettingsResource().Schema(context.Background(), resource.SchemaRequest{}, &resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics: %v", resp.Diagnostics)
	}
	want := []string{"id", "email"}
	for _, s := range groupSettings {
		want = append(want, s.name)
	}
	for _, attr := range want {
		if _, ok := resp.Schema.Attributes[attr]; !ok {
			t.Errorf("resource missing attribute %q", attr)
		}
	}
}

// Each setting must read and write the same API field, and no two settings may
// share one.
func TestGroupSettings_FieldMapping(t *testing.T) {
	t.Parallel()

	seen := map[string]bool{}
	for _, s := range groupSettings {
		if seen[s.name] {
			t.Errorf("duplicate setting %q", s.name)
		}
		seen[s.name] = true

		g := &groupssettings.Groups{}
		value := "true"
		if !s.isBool() {
			value = s.values[0]
		}
		s.set(g, value)
		if got := s.get(g); got != value {
			t.Errorf("%s: set %q, got %q back", s.name, value, got)
		}
		for _, other := range groupSettings {
			if other.name != s.name && other.get(g) != "" {
				t.Errorf("setting %s also wrote %s", s.name, other.name)
			}
		}
	}
}

func TestAPIBool(t *testing.T) {
	t.Parallel()

	if b := apiBool("true"); b.IsNull() || !b.ValueBool() {
		t.Errorf(`apiBool("true") = %v`, b)
	}
	if b := apiBool("false"); b.IsNull() || b.ValueBool() {
		t.Errorf(`apiBool("false") = %v`, b)
	}
	if b := apiBool(""); !b.IsNull() {
		t.Errorf(`apiBool("") = %v, want null`, b)
	}
}
