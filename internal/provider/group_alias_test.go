package provider

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	admin "google.golang.org/api/admin/directory/v1"
)

func TestGroupAliasResource_Schema(t *testing.T) {
	t.Parallel()

	var resp resource.SchemaResponse
	NewGroupAliasResource().Schema(context.Background(), resource.SchemaRequest{}, &resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics: %v", resp.Diagnostics)
	}
	for _, attr := range []string{"id", "group_id", "alias"} {
		if _, ok := resp.Schema.Attributes[attr]; !ok {
			t.Errorf("resource missing attribute %q", attr)
		}
	}
	// There is no update API, so every configurable attribute must force replacement.
	for _, attr := range []string{"group_id", "alias"} {
		a, ok := resp.Schema.Attributes[attr].(schema.StringAttribute)
		if !ok {
			t.Fatalf("%s is not a string attribute", attr)
		}
		if !a.Required || len(a.PlanModifiers) == 0 {
			t.Errorf("%s: want required with a RequiresReplace plan modifier", attr)
		}
	}
}

func TestSplitAliasID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id, group, alias string
		ok               bool
	}{
		{"01grp/trips@example.org", "01grp", "trips@example.org", true},
		{"hiking@example.org/trips@example.org", "hiking@example.org", "trips@example.org", true},
		{"01grp", "", "", false},
		{"/trips@example.org", "", "", false},
		{"01grp/", "", "", false},
		{"01grp/trips", "", "", false},
	}
	for _, tt := range tests {
		group, alias, ok := splitAliasID(tt.id)
		if group != tt.group || alias != tt.alias || ok != tt.ok {
			t.Errorf("splitAliasID(%q) = %q, %q, %v; want %q, %q, %v", tt.id, group, alias, ok, tt.group, tt.alias, tt.ok)
		}
	}
}

// The list response carries its items untyped, as decoded from JSON.
func TestDecodeAliases(t *testing.T) {
	t.Parallel()

	var list admin.Aliases
	body := `{"kind":"admin#directory#aliases","aliases":[
		{"kind":"admin#directory#alias","id":"01grp","primaryEmail":"hiking@example.org","alias":"trips@example.org"},
		{"kind":"admin#directory#alias","id":"01grp","primaryEmail":"hiking@example.org","alias":"outings@example.org"}]}`
	if err := json.Unmarshal([]byte(body), &list); err != nil {
		t.Fatal(err)
	}

	aliases, err := decodeAliases(&list)
	if err != nil {
		t.Fatalf("decodeAliases: %v", err)
	}
	if len(aliases) != 2 || aliases[0].Alias != "trips@example.org" || aliases[1].Id != "01grp" {
		t.Fatalf("decodeAliases = %+v", aliases)
	}
	if !hasAlias(aliases, "Trips@Example.org") {
		t.Error("hasAlias should match without regard to case")
	}
	if hasAlias(aliases, "climbs@example.org") {
		t.Error("hasAlias matched an alias that is not listed")
	}

	empty, err := decodeAliases(&admin.Aliases{})
	if err != nil || len(empty) != 0 {
		t.Errorf("decodeAliases(empty) = %v, %v", empty, err)
	}
	if none, err := decodeAliases(nil); err != nil || none != nil {
		t.Errorf("decodeAliases(nil) = %v, %v", none, err)
	}
}
