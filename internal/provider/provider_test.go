package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/provider"
)

func TestProvider_Metadata(t *testing.T) {
	t.Parallel()

	p := New("test")()
	var resp provider.MetadataResponse
	p.Metadata(context.Background(), provider.MetadataRequest{}, &resp)

	if resp.TypeName != "googleworkspace" {
		t.Errorf("TypeName = %q, want %q", resp.TypeName, "googleworkspace")
	}
	if resp.Version != "test" {
		t.Errorf("Version = %q, want %q", resp.Version, "test")
	}
}

func TestProvider_Schema(t *testing.T) {
	t.Parallel()

	p := New("test")()
	var resp provider.SchemaResponse
	p.Schema(context.Background(), provider.SchemaRequest{}, &resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics: %v", resp.Diagnostics)
	}
	for _, attr := range []string{"customer_id", "access_token"} {
		if _, ok := resp.Schema.Attributes[attr]; !ok {
			t.Errorf("missing provider attribute %q", attr)
		}
	}
}
