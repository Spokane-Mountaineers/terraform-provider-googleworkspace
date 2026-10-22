package provider

import (
	"context"
	"fmt"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/Spokane-Mountaineers/terraform-provider-googleworkspace/internal/client"
)

var (
	_ datasource.DataSource              = (*orgUnitsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*orgUnitsDataSource)(nil)
)

type orgUnitsDataSource struct {
	client *client.Client
}

// NewOrgUnitsDataSource is the data-source factory.
func NewOrgUnitsDataSource() datasource.DataSource {
	return &orgUnitsDataSource{}
}

type orgUnitsDataModel struct {
	OrgUnits []orgUnitsDataItemModel `tfsdk:"org_units"`
}

type orgUnitsDataItemModel struct {
	ID                types.String `tfsdk:"id"`
	Name              types.String `tfsdk:"name"`
	OrgUnitPath       types.String `tfsdk:"org_unit_path"`
	ParentOrgUnitPath types.String `tfsdk:"parent_org_unit_path"`
	Description       types.String `tfsdk:"description"`
}

func (d *orgUnitsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_org_units"
}

func (d *orgUnitsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	str := func(desc string) schema.StringAttribute {
		return schema.StringAttribute{Computed: true, MarkdownDescription: desc}
	}

	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists every organizational unit in the Workspace account below the root, including " +
			"empty ones. Compare it against managed `googleworkspace_org_unit` resources to find units created " +
			"in the Admin console.",
		Attributes: map[string]schema.Attribute{
			"org_units": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "All organizational units, ordered by path.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":                   str("Immutable organizational unit ID (the `id:...` value). Used for import."),
						"name":                 str("Name of the organizational unit."),
						"org_unit_path":        str("Full path, e.g. `/SMI Board`."),
						"parent_org_unit_path": str("Full path of the parent organizational unit."),
						"description":          str("Description of the organizational unit."),
					},
				},
			},
		},
	}
}

func (d *orgUnitsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *orgUnitsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	// type=all returns every descendant of the root, not just direct children.
	list, err := d.client.Directory.Orgunits.List(d.client.CustomerID).Type("all").Context(ctx).Do()
	if err != nil {
		resp.Diagnostics.AddError("Unable to list organizational units", err.Error())
		return
	}

	data := orgUnitsDataModel{OrgUnits: []orgUnitsDataItemModel{}}
	for _, ou := range list.OrganizationUnits {
		data.OrgUnits = append(data.OrgUnits, orgUnitsDataItemModel{
			ID:                types.StringValue(ou.OrgUnitId),
			Name:              types.StringValue(ou.Name),
			OrgUnitPath:       types.StringValue(ou.OrgUnitPath),
			ParentOrgUnitPath: types.StringValue(ou.ParentOrgUnitPath),
			Description:       types.StringValue(ou.Description),
		})
	}
	sort.Slice(data.OrgUnits, func(i, j int) bool {
		return data.OrgUnits[i].OrgUnitPath.ValueString() < data.OrgUnits[j].OrgUnitPath.ValueString()
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, data)...)
}
