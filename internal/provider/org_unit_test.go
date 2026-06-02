package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

func TestOrgUnitResource_Schema(t *testing.T) {
	t.Parallel()

	var resp resource.SchemaResponse
	NewOrgUnitResource().Schema(context.Background(), resource.SchemaRequest{}, &resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics: %v", resp.Diagnostics)
	}
	for _, attr := range []string{"id", "name", "parent_org_unit_path", "description", "org_unit_path"} {
		if _, ok := resp.Schema.Attributes[attr]; !ok {
			t.Errorf("resource missing attribute %q", attr)
		}
	}
}

func TestOrgUnitDataSource_Schema(t *testing.T) {
	t.Parallel()

	var resp datasource.SchemaResponse
	NewOrgUnitDataSource().Schema(context.Background(), datasource.SchemaRequest{}, &resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics: %v", resp.Diagnostics)
	}
	if _, ok := resp.Schema.Attributes["org_unit_path"]; !ok {
		t.Error("data source missing required attribute \"org_unit_path\"")
	}
}
