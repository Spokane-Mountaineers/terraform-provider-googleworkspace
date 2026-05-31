package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/Spokane-Mountaineers/terraform-provider-google-workspace/internal/client"
)

var (
	_ datasource.DataSource              = (*domainsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*domainsDataSource)(nil)
)

// domainsDataSource lists the domains on the Workspace account. It is the
// smallest read that proves auth and API connectivity end-to-end.
type domainsDataSource struct {
	client *client.Client
}

// NewDomainsDataSource is the data-source factory.
func NewDomainsDataSource() datasource.DataSource {
	return &domainsDataSource{}
}

type domainsModel struct {
	CustomerID types.String  `tfsdk:"customer_id"`
	Domains    []domainModel `tfsdk:"domains"`
}

type domainModel struct {
	DomainName types.String `tfsdk:"domain_name"`
	IsPrimary  types.Bool   `tfsdk:"is_primary"`
	Verified   types.Bool   `tfsdk:"verified"`
}

func (d *domainsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domains"
}

func (d *domainsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists the domains registered on the Google Workspace account.",
		Attributes: map[string]schema.Attribute{
			"customer_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The customer ID the domains were read for.",
			},
			"domains": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "The domains on the account.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"domain_name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The fully qualified domain name.",
						},
						"is_primary": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Whether this is the primary domain.",
						},
						"verified": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Whether the domain is verified.",
						},
					},
				},
			},
		},
	}
}

func (d *domainsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected provider data type",
			fmt.Sprintf("expected *client.Client, got %T", req.ProviderData),
		)
		return
	}
	d.client = c
}

func (d *domainsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	res, err := d.client.Directory.Domains.List(d.client.CustomerID).Context(ctx).Do()
	if err != nil {
		resp.Diagnostics.AddError("Unable to list domains", err.Error())
		return
	}

	state := domainsModel{CustomerID: types.StringValue(d.client.CustomerID)}
	for _, dm := range res.Domains {
		state.Domains = append(state.Domains, domainModel{
			DomainName: types.StringValue(dm.DomainName),
			IsPrimary:  types.BoolValue(dm.IsPrimary),
			Verified:   types.BoolValue(dm.Verified),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
