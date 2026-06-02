package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/Spokane-Mountaineers/terraform-provider-googleworkspace/internal/client"
)

var (
	_ datasource.DataSource              = (*orgUnitDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*orgUnitDataSource)(nil)
)

type orgUnitDataSource struct {
	client *client.Client
}

// NewOrgUnitDataSource is the data-source factory.
func NewOrgUnitDataSource() datasource.DataSource {
	return &orgUnitDataSource{}
}

type orgUnitDataModel struct {
	OrgUnitPath       types.String `tfsdk:"org_unit_path"`
	ID                types.String `tfsdk:"id"`
	Name              types.String `tfsdk:"name"`
	Description       types.String `tfsdk:"description"`
	ParentOrgUnitPath types.String `tfsdk:"parent_org_unit_path"`
}

func (d *orgUnitDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_org_unit"
}

func (d *orgUnitDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up a single Google Workspace organizational unit by its full path.",
		Attributes: map[string]schema.Attribute{
			"org_unit_path": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Full path of the organizational unit, e.g. `/Engineering/Frontend`.",
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Immutable organizational unit ID.",
			},
			"name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Name of the organizational unit.",
			},
			"description": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Description of the organizational unit.",
			},
			"parent_org_unit_path": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Full path of the parent organizational unit.",
			},
		},
	}
}

func (d *orgUnitDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type", fmt.Sprintf("expected *client.Client, got %T", req.ProviderData))
		return
	}
	d.client = c
}

func (d *orgUnitDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data orgUnitDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The Admin SDK path parameter wants the path without the leading slash.
	ou, err := d.client.Directory.Orgunits.Get(
		d.client.CustomerID,
		strings.TrimPrefix(data.OrgUnitPath.ValueString(), "/"),
	).Context(ctx).Do()
	if err != nil {
		resp.Diagnostics.AddError("Unable to read organizational unit", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, orgUnitDataModel{
		OrgUnitPath:       types.StringValue(ou.OrgUnitPath),
		ID:                types.StringValue(ou.OrgUnitId),
		Name:              types.StringValue(ou.Name),
		Description:       types.StringValue(ou.Description),
		ParentOrgUnitPath: types.StringValue(ou.ParentOrgUnitPath),
	})...)
}
