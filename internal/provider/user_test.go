package provider

import (
	"context"
	"slices"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	admin "google.golang.org/api/admin/directory/v1"
)

func TestUserResource_Schema(t *testing.T) {
	t.Parallel()

	var resp resource.SchemaResponse
	NewUserResource().Schema(context.Background(), resource.SchemaRequest{}, &resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics: %v", resp.Diagnostics)
	}
	if diags := resp.Schema.ValidateImplementation(context.Background()); diags.HasError() {
		t.Fatalf("schema implementation: %v", diags)
	}
	for _, attr := range []string{
		"id", "primary_email", "given_name", "family_name", "org_unit_path", "suspended", "archived",
		"recovery_email", "recovery_phone", "include_in_global_address_list",
		"change_password_at_next_login", "password_wo", "password_wo_version",
	} {
		if _, ok := resp.Schema.Attributes[attr]; !ok {
			t.Errorf("resource missing attribute %q", attr)
		}
	}
	pw, ok := resp.Schema.Attributes["password_wo"].(schema.StringAttribute)
	if !ok || !pw.WriteOnly || !pw.Sensitive {
		t.Error("password_wo must be a sensitive, write-only string")
	}
}

func TestUserToModel(t *testing.T) {
	t.Parallel()

	prior := userModel{
		ChangePasswordAtNextLogin: types.BoolValue(true),
		PasswordWOVersion:         types.StringValue("1"),
	}

	tests := []struct {
		name string
		user *admin.User
		want userModel
	}{
		{
			name: "full",
			user: &admin.User{
				Id: "123", PrimaryEmail: "jdoe@example.com",
				Name:        &admin.UserName{GivenName: "Jane", FamilyName: "Doe"},
				OrgUnitPath: "/Board", Suspended: true, IncludeInGlobalAddressList: true,
				RecoveryEmail: "jane@example.net", RecoveryPhone: "+15095550100",
			},
			want: userModel{
				ID: types.StringValue("123"), PrimaryEmail: types.StringValue("jdoe@example.com"),
				GivenName: types.StringValue("Jane"), FamilyName: types.StringValue("Doe"),
				OrgUnitPath: types.StringValue("/Board"), Suspended: types.BoolValue(true),
				Archived: types.BoolValue(false), IncludeInGlobalAddressList: types.BoolValue(true),
				RecoveryEmail: types.StringValue("jane@example.net"), RecoveryPhone: types.StringValue("+15095550100"),
				ChangePasswordAtNextLogin: types.BoolValue(true), PasswordWO: types.StringNull(),
				PasswordWOVersion: types.StringValue("1"),
			},
		},
		{
			name: "nil name and empty recovery fields",
			user: &admin.User{Id: "456", PrimaryEmail: "x@example.com", OrgUnitPath: "/"},
			want: userModel{
				ID: types.StringValue("456"), PrimaryEmail: types.StringValue("x@example.com"),
				GivenName: types.StringValue(""), FamilyName: types.StringValue(""),
				OrgUnitPath: types.StringValue("/"), Suspended: types.BoolValue(false),
				Archived: types.BoolValue(false), IncludeInGlobalAddressList: types.BoolValue(false),
				RecoveryEmail: types.StringNull(), RecoveryPhone: types.StringNull(),
				ChangePasswordAtNextLogin: types.BoolValue(true), PasswordWO: types.StringNull(),
				PasswordWOVersion: types.StringValue("1"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := userToModel(tt.user, prior); got != tt.want {
				t.Errorf("userToModel() =\n%+v\nwant\n%+v", got, tt.want)
			}
		})
	}
}

func TestUserToModel_ImportDefaultsChangePasswordFlag(t *testing.T) {
	t.Parallel()

	// After import, prior state holds only the ID.
	got := userToModel(&admin.User{Id: "1"}, userModel{ID: types.StringValue("1")})
	if !got.ChangePasswordAtNextLogin.Equal(types.BoolValue(false)) {
		t.Errorf("change_password_at_next_login = %v, want false", got.ChangePasswordAtNextLogin)
	}
}

func TestModelToUser_ForceSendsClearableFields(t *testing.T) {
	t.Parallel()

	u := modelToUser(userModel{
		Suspended:     types.BoolValue(false),
		RecoveryEmail: types.StringNull(),
	})
	for _, f := range []string{"Suspended", "Archived", "IncludeInGlobalAddressList", "RecoveryEmail", "RecoveryPhone"} {
		if !slices.Contains(u.ForceSendFields, f) {
			t.Errorf("ForceSendFields missing %q; a false/empty value would be dropped", f)
		}
	}
}
