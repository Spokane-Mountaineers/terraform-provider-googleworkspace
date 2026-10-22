package provider

import (
	"context"
	"fmt"
	"sort"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	admin "google.golang.org/api/admin/directory/v1"

	"github.com/Spokane-Mountaineers/terraform-provider-googleworkspace/internal/client"
)

var (
	_ datasource.DataSource              = (*roleAssignmentsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*roleAssignmentsDataSource)(nil)
)

type roleAssignmentsDataSource struct {
	client *client.Client
}

// NewRoleAssignmentsDataSource is the data-source factory.
func NewRoleAssignmentsDataSource() datasource.DataSource {
	return &roleAssignmentsDataSource{}
}

type roleAssignmentsDataModel struct {
	RoleAssignments []roleAssignmentsDataItemModel `tfsdk:"role_assignments"`
}

type roleAssignmentsDataItemModel struct {
	ID           types.String `tfsdk:"id"`
	RoleID       types.String `tfsdk:"role_id"`
	RoleName     types.String `tfsdk:"role_name"`
	IsSuperAdmin types.Bool   `tfsdk:"is_super_admin"`
	AssignedTo   types.String `tfsdk:"assigned_to"`
	AssigneeType types.String `tfsdk:"assignee_type"`
	ScopeType    types.String `tfsdk:"scope_type"`
	OrgUnitID    types.String `tfsdk:"org_unit_id"`
}

func (d *roleAssignmentsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role_assignments"
}

func (d *roleAssignmentsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	str := func(desc string) schema.StringAttribute {
		return schema.StringAttribute{Computed: true, MarkdownDescription: desc}
	}

	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists every admin role assignment in the account: who holds which admin role, and " +
			"whether it covers the whole account or a single organizational unit.",
		Attributes: map[string]schema.Attribute{
			"role_assignments": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "All role assignments, ordered by role name.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":        str("Role assignment ID."),
						"role_id":   str("ID of the assigned role."),
						"role_name": str("Name of the assigned role, e.g. `_SEED_ADMIN_ROLE` for super admin."),
						"is_super_admin": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Whether the role is the super admin role.",
						},
						"assigned_to": str("ID of the user, group, or service account holding the role. Join with " +
							"`googleworkspace_users` on `id` to get an email."),
						"assignee_type": str("`USER` or `GROUP`."),
						"scope_type":    str("`CUSTOMER` (whole account) or `ORG_UNIT`."),
						"org_unit_id":   str("For `ORG_UNIT` scope, the unit as `id:...`; otherwise null."),
					},
				},
			},
		},
	}
}

func (d *roleAssignmentsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *roleAssignmentsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	roles := map[int64]*admin.Role{}
	err := d.client.Directory.Roles.List(d.client.CustomerID).Pages(ctx, func(page *admin.Roles) error {
		for _, r := range page.Items {
			roles[r.RoleId] = r
		}
		return nil
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to list admin roles", err.Error())
		return
	}

	data := roleAssignmentsDataModel{RoleAssignments: []roleAssignmentsDataItemModel{}}
	err = d.client.Directory.RoleAssignments.List(d.client.CustomerID).Pages(ctx, func(page *admin.RoleAssignments) error {
		for _, a := range page.Items {
			m := roleAssignmentsDataItemModel{
				ID:           types.StringValue(strconv.FormatInt(a.RoleAssignmentId, 10)),
				RoleID:       types.StringValue(strconv.FormatInt(a.RoleId, 10)),
				RoleName:     types.StringValue(""),
				IsSuperAdmin: types.BoolValue(false),
				AssignedTo:   types.StringValue(a.AssignedTo),
				AssigneeType: types.StringValue(a.AssigneeType),
				ScopeType:    types.StringValue(a.ScopeType),
				OrgUnitID:    stringOrNull(normalizeOrgUnitID(a.OrgUnitId)),
			}
			if r, ok := roles[a.RoleId]; ok {
				m.RoleName = types.StringValue(r.RoleName)
				m.IsSuperAdmin = types.BoolValue(r.IsSuperAdminRole)
			}
			data.RoleAssignments = append(data.RoleAssignments, m)
		}
		return nil
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to list role assignments", err.Error())
		return
	}
	sort.SliceStable(data.RoleAssignments, func(i, j int) bool {
		return data.RoleAssignments[i].RoleName.ValueString() < data.RoleAssignments[j].RoleName.ValueString()
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, data)...)
}
