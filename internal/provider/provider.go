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
	CustomerID            types.String `tfsdk:"customer_id"`
	ServiceAccount        types.String `tfsdk:"service_account"`
	ImpersonatedUserEmail types.String `tfsdk:"impersonated_user_email"`
	AccessToken           types.String `tfsdk:"access_token"`
}

// New returns a provider factory for the given build version.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &googleWorkspaceProvider{version: version}
	}
}

func (p *googleWorkspaceProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "googleworkspace"
	resp.Version = p.version
}

func (p *googleWorkspaceProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manage Google Workspace directory resources via the Admin SDK Directory and Cloud Identity APIs. " +
			"Authentication is **domain-wide delegation only** — see the provider guide.",
		Attributes: map[string]schema.Attribute{
			"service_account": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "Email of the service account with domain-wide delegation. The caller's " +
					"Application Default Credentials must hold `roles/iam.serviceAccountTokenCreator` on it so the " +
					"delegation token is minted keyless. Also settable via `GOOGLEWORKSPACE_SERVICE_ACCOUNT`.",
			},
			"impersonated_user_email": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "The Workspace admin user the service account impersonates (the delegation " +
					"subject). Also settable via `GOOGLEWORKSPACE_IMPERSONATED_USER_EMAIL`.",
			},
			"access_token": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				MarkdownDescription: "Escape hatch: a pre-minted domain-wide delegation access token (for example in " +
					"CI). Takes precedence over `service_account`/`impersonated_user_email`. Also settable via " +
					"`GOOGLEWORKSPACE_ACCESS_TOKEN`.",
			},
			"customer_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Workspace customer ID. Defaults to `my_customer`. Also settable via `GOOGLEWORKSPACE_CUSTOMER_ID`.",
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
		CustomerID:            cfg.CustomerID.ValueString(),
		ServiceAccount:        cfg.ServiceAccount.ValueString(),
		ImpersonatedUserEmail: cfg.ImpersonatedUserEmail.ValueString(),
		AccessToken:           cfg.AccessToken.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to configure Google Workspace client", err.Error())
		return
	}

	resp.DataSourceData = c
	resp.ResourceData = c
}

func (p *googleWorkspaceProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewOrgUnitResource,
	}
}

func (p *googleWorkspaceProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewDomainsDataSource,
		NewOrgUnitDataSource,
	}
}
