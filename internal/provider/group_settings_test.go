package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
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
		switch {
		case s.text:
			value = "replies@example.org"
		case !s.isBool():
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

// boolPlan is a planReader holding Bool attributes; missing ones read as null.
type boolPlan map[string]types.Bool

func (p boolPlan) GetAttribute(_ context.Context, at path.Path, target any) diag.Diagnostics {
	v, ok := p[at.String()]
	if !ok {
		v = types.BoolNull()
	}
	*(target.(*types.Bool)) = v
	return nil
}

func TestWarnCollaborativeInboxWithoutHistory(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		inbox, archived types.Bool
		warn            bool
	}{
		{"inbox with history", types.BoolValue(true), types.BoolValue(true), false},
		{"inbox without history", types.BoolValue(true), types.BoolValue(false), true},
		{"inbox, history unknown", types.BoolValue(true), types.BoolUnknown(), false},
		{"inbox, history null", types.BoolValue(true), types.BoolNull(), false},
		{"inbox unknown", types.BoolUnknown(), types.BoolValue(false), false},
		{"inbox off", types.BoolValue(false), types.BoolValue(false), false},
		{"inbox null", types.BoolNull(), types.BoolValue(false), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			diags := warnCollaborativeInboxWithoutHistory(context.Background(), boolPlan{
				"enable_collaborative_inbox": tt.inbox,
				"is_archived":                tt.archived,
			})
			if diags.HasError() {
				t.Fatalf("unexpected error: %v", diags)
			}
			if got := diags.WarningsCount() > 0; got != tt.warn {
				t.Errorf("warning = %v, want %v (%v)", got, tt.warn, diags)
			}
		})
	}
}

// stringPlan is a planReader holding String attributes; missing ones read as null.
type stringPlan map[string]types.String

func (p stringPlan) GetAttribute(_ context.Context, at path.Path, target any) diag.Diagnostics {
	v, ok := p[at.String()]
	if !ok {
		v = types.StringNull()
	}
	*(target.(*types.String)) = v
	return nil
}

func TestCheckCustomReplyTo(t *testing.T) {
	t.Parallel()

	addr := types.StringValue("replies@example.org")
	tests := []struct {
		name                         string
		replyTo, planned, configured types.String
		err, warn                    bool
	}{
		{"custom with address", types.StringValue("REPLY_TO_CUSTOM"), addr, addr, false, false},
		{"custom without address", types.StringValue("REPLY_TO_CUSTOM"), types.StringNull(), types.StringNull(), true, false},
		{"custom with empty address", types.StringValue("REPLY_TO_CUSTOM"), types.StringValue(""), types.StringNull(), true, false},
		{"custom, address unknown", types.StringValue("REPLY_TO_CUSTOM"), types.StringUnknown(), types.StringNull(), false, true},
		{"list with configured address", types.StringValue("REPLY_TO_LIST"), addr, addr, false, true},
		{"list with address kept in state", types.StringValue("REPLY_TO_LIST"), addr, types.StringNull(), false, false},
		{"list without address", types.StringValue("REPLY_TO_LIST"), types.StringNull(), types.StringNull(), false, false},
		{"reply_to unknown", types.StringUnknown(), types.StringNull(), addr, false, false},
		{"reply_to null", types.StringNull(), types.StringNull(), addr, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			diags := checkCustomReplyTo(context.Background(),
				stringPlan{"custom_reply_to": tt.configured},
				stringPlan{"reply_to": tt.replyTo, "custom_reply_to": tt.planned},
			)
			if got := diags.HasError(); got != tt.err {
				t.Errorf("error = %v, want %v (%v)", got, tt.err, diags)
			}
			if got := diags.WarningsCount() > 0; got != tt.warn {
				t.Errorf("warning = %v, want %v (%v)", got, tt.warn, diags)
			}
		})
	}
}
