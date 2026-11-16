package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	admin "google.golang.org/api/admin/directory/v1"
)

func TestUserAliasResource_Schema(t *testing.T) {
	t.Parallel()

	var resp resource.SchemaResponse
	NewUserAliasResource().Schema(context.Background(), resource.SchemaRequest{}, &resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics: %v", resp.Diagnostics)
	}
	if diags := resp.Schema.ValidateImplementation(context.Background()); diags.HasError() {
		t.Fatalf("schema implementation: %v", diags)
	}
	for _, attr := range []string{"id", "user_id", "alias"} {
		if _, ok := resp.Schema.Attributes[attr]; !ok {
			t.Errorf("resource missing attribute %q", attr)
		}
	}
	// There is no update API, so every configurable attribute must force replacement.
	for _, attr := range []string{"user_id", "alias"} {
		a, ok := resp.Schema.Attributes[attr].(schema.StringAttribute)
		if !ok {
			t.Fatalf("%s is not a string attribute", attr)
		}
		if !a.Required || len(a.PlanModifiers) == 0 {
			t.Errorf("%s: want required with a RequiresReplace plan modifier", attr)
		}
	}
}

func TestUserAliasToModel(t *testing.T) {
	t.Parallel()

	got := userAliasToModel("123", "jane@example.org")
	want := userAliasModel{
		ID:     types.StringValue("123/jane@example.org"),
		UserID: types.StringValue("123"),
		Alias:  types.StringValue("jane@example.org"),
	}
	if got != want {
		t.Errorf("userAliasToModel = %+v, want %+v", got, want)
	}
}

// A user's email works in place of its ID on import, like a group's.
func TestSplitAliasID_UserEmail(t *testing.T) {
	t.Parallel()

	user, alias, ok := splitAliasID("jdoe@example.org/jane@example.org")
	if !ok || user != "jdoe@example.org" || alias != "jane@example.org" {
		t.Errorf("splitAliasID = %q, %q, %v", user, alias, ok)
	}
}

func TestUserToDataModel_Aliases(t *testing.T) {
	t.Parallel()

	with := userToDataModel(&admin.User{
		Id: "123", PrimaryEmail: "jdoe@example.org",
		Aliases:            []string{"jane@example.org", "j.doe@example.org"},
		NonEditableAliases: []string{"jdoe@example.org.test-google-a.com"},
	})
	want := []types.String{types.StringValue("jane@example.org"), types.StringValue("j.doe@example.org")}
	if len(with.Aliases) != len(want) || with.Aliases[0] != want[0] || with.Aliases[1] != want[1] {
		t.Errorf("aliases = %v, want %v", with.Aliases, want)
	}

	// No aliases is an empty list, not null, so configuration can iterate it.
	without := userToDataModel(&admin.User{Id: "456", PrimaryEmail: "asmith@example.org"})
	if without.Aliases == nil || len(without.Aliases) != 0 {
		t.Errorf("aliases = %#v, want an empty list", without.Aliases)
	}
}
