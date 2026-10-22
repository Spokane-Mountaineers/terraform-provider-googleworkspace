package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"google.golang.org/api/cloudidentity/v1"
	"google.golang.org/api/googleapi"
)

func TestPoliciesAndRoleAssignmentsDataSources_Schema(t *testing.T) {
	t.Parallel()

	for name, ds := range map[string]datasource.DataSource{
		"policies":         NewPoliciesDataSource(),
		"role_assignments": NewRoleAssignmentsDataSource(),
	} {
		var resp datasource.SchemaResponse
		ds.Schema(context.Background(), datasource.SchemaRequest{}, &resp)
		if resp.Diagnostics.HasError() {
			t.Errorf("%s schema diagnostics: %v", name, resp.Diagnostics)
		}
		if _, ok := resp.Schema.Attributes[name]; !ok {
			t.Errorf("%s data source missing attribute %q", name, name)
		}
	}
}

func TestNormalizeOrgUnitID(t *testing.T) {
	t.Parallel()

	for in, want := range map[string]string{
		"orgUnits/03ph8a2z": "id:03ph8a2z",
		"03ph8a2z":          "id:03ph8a2z",
		"id:03ph8a2z":       "id:03ph8a2z",
		"":                  "",
	} {
		if got := normalizeOrgUnitID(in); got != want {
			t.Errorf("normalizeOrgUnitID(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPolicyToModel(t *testing.T) {
	t.Parallel()

	m := policyToModel(&cloudidentity.Policy{
		Name:        "policies/abc",
		Type:        "ADMIN",
		PolicyQuery: &cloudidentity.PolicyQuery{OrgUnit: "orgUnits/03ph", Query: "q", SortOrder: 2},
		Setting:     &cloudidentity.Setting{Type: "settings/x", Value: googleapi.RawMessage(`{"a":1}`)},
	})
	if m.OrgUnitID.ValueString() != "id:03ph" || !m.Group.IsNull() || m.Value.ValueString() != `{"a":1}` ||
		m.SettingType.ValueString() != "settings/x" {
		t.Errorf("policyToModel() = %+v", m)
	}
}
