package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/Spokane-Mountaineers/terraform-provider-googleworkspace/internal/client"
)

var (
	_ datasource.DataSource              = (*groupDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*groupDataSource)(nil)
)

type groupDataSource struct {
	client *client.Client
}

// NewGroupDataSource is the data-source factory.
func NewGroupDataSource() datasource.DataSource {
	return &groupDataSource{}
}

type groupDataModel struct {
	Email       types.String `tfsdk:"email"`
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
}

func (d *groupDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group"
}

func (d *groupDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up a single Google Workspace group by its email address.",
		Attributes: map[string]schema.Attribute{
			"email": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The group's email address (primary or an alias).",
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Immutable group ID.",
			},
			"name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Display name of the group.",
			},
			"description": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Description of the group.",
			},
		},
	}
}

func (d *groupDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *groupDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data groupDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	g, err := d.client.Directory.Groups.Get(data.Email.ValueString()).Context(ctx).Do()
	if err != nil {
		resp.Diagnostics.AddError("Unable to read group", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, groupDataModel{
		Email:       types.StringValue(g.Email),
		ID:          types.StringValue(g.Id),
		Name:        types.StringValue(g.Name),
		Description: types.StringValue(g.Description),
	})...)
}
