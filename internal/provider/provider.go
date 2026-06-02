package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/Spokane-Mountaineers/terraform-provider-googleworkspace/internal/client"
)

// Ensure the provider satisfies the framework interface.
var _ provider.Provider = (*googleWorkspaceProvider)(nil)

type googleWorkspaceProvider struct {
	version string
}

type providerModel struct {
	CustomerID  types.String `tfsdk:"customer_id"`
	AccessToken types.String `tfsdk:"access_token"`
}

// New returns a provider factory for the given build version.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &googleWorkspaceProvider{version: version}
	}
}

func (p *googleWorkspaceProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	// Resource/data-source prefix. Hyphen-free because Terraform type names must
	// be valid identifiers, so resources are googleworkspace_*.
	resp.TypeName = "googleworkspace"
	resp.Version = p.version
}

func (p *googleWorkspaceProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manage Google Workspace directory resources via the Admin SDK Directory and Cloud Identity APIs.",
		Attributes: map[string]schema.Attribute{
			"customer_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Workspace customer ID. Defaults to `my_customer`. Also settable via `GOOGLEWORKSPACE_CUSTOMER_ID`.",
			},
			"access_token": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "OAuth2 access token with Admin SDK directory scopes. If unset, falls back to `GOOGLEWORKSPACE_ACCESS_TOKEN` and then Application Default Credentials.",
			},
		},
	}
}

func (p *googleWorkspaceProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var cfg providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	c, err := client.New(ctx, client.Config{
		CustomerID:  cfg.CustomerID.ValueString(),
		AccessToken: cfg.AccessToken.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to configure Google Workspace client", err.Error())
		return
	}

	resp.DataSourceData = c
	resp.ResourceData = c
}

func (p *googleWorkspaceProvider) Resources(_ context.Context) []func() resource.Resource {
	return nil
}

func (p *googleWorkspaceProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewDomainsDataSource,
	}
}
