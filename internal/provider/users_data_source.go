package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	admin "google.golang.org/api/admin/directory/v1"

	"github.com/Spokane-Mountaineers/terraform-provider-googleworkspace/internal/client"
)

var (
	_ datasource.DataSource              = (*usersDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*usersDataSource)(nil)
)

type usersDataSource struct {
	client *client.Client
}

// NewUsersDataSource is the data-source factory.
func NewUsersDataSource() datasource.DataSource {
	return &usersDataSource{}
}

type usersDataModel struct {
	Query types.String         `tfsdk:"query"`
	Users []usersDataUserModel `tfsdk:"users"`
}

type usersDataUserModel struct {
	ID                         types.String `tfsdk:"id"`
	PrimaryEmail               types.String `tfsdk:"primary_email"`
	GivenName                  types.String `tfsdk:"given_name"`
	FamilyName                 types.String `tfsdk:"family_name"`
	OrgUnitPath                types.String `tfsdk:"org_unit_path"`
	Suspended                  types.Bool   `tfsdk:"suspended"`
	Archived                   types.Bool   `tfsdk:"archived"`
	IsAdmin                    types.Bool   `tfsdk:"is_admin"`
	RecoveryEmail              types.String `tfsdk:"recovery_email"`
	RecoveryPhone              types.String `tfsdk:"recovery_phone"`
	IncludeInGlobalAddressList types.Bool   `tfsdk:"include_in_global_address_list"`
	CreationTime               types.String `tfsdk:"creation_time"`
	LastLoginTime              types.String `tfsdk:"last_login_time"`
}

func (d *usersDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_users"
}

func (d *usersDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	str := func(desc string) schema.StringAttribute {
		return schema.StringAttribute{Computed: true, MarkdownDescription: desc}
	}
	boolean := func(desc string) schema.BoolAttribute {
		return schema.BoolAttribute{Computed: true, MarkdownDescription: desc}
	}

	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists every user in the Workspace account, optionally filtered by an Admin SDK search query. " +
			"Compare it against the users you manage to find accounts created outside of code.",
		Attributes: map[string]schema.Attribute{
			"query": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "Admin SDK [user search query](https://developers.google.com/admin-sdk/directory/v1/guides/search-users), " +
					"e.g. `orgUnitPath='/Board'` or `isSuspended=false`. Omit to list every user.",
			},
			"users": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Matching users, ordered by primary email.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":                             str("Immutable user ID."),
						"primary_email":                  str("Primary email address."),
						"given_name":                     str("First name."),
						"family_name":                    str("Last name."),
						"org_unit_path":                  str("Full path of the user's organizational unit."),
						"suspended":                      boolean("Whether the account is suspended."),
						"archived":                       boolean("Whether the account is archived."),
						"is_admin":                       boolean("Whether the user is a super admin."),
						"recovery_email":                 str("Recovery email address, or null."),
						"recovery_phone":                 str("Recovery phone number (E.164), or null."),
						"include_in_global_address_list": boolean("Whether the user appears in the global address list."),
						"creation_time":                  str("When the account was created (RFC 3339)."),
						"last_login_time":                str("Last sign-in time (RFC 3339). `1970-01-01T00:00:00.000Z` means never."),
					},
				},
			},
		},
	}
}

func (d *usersDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *usersDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data usersDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	call := d.client.Directory.Users.List().
		Customer(d.client.CustomerID).
		OrderBy("email").
		MaxResults(500)
	if q := data.Query.ValueString(); q != "" {
		call = call.Query(q)
	}

	data.Users = []usersDataUserModel{}
	err := call.Pages(ctx, func(page *admin.Users) error {
		for _, u := range page.Users {
			data.Users = append(data.Users, userToDataModel(u))
		}
		return nil
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to list users", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, data)...)
}

func userToDataModel(u *admin.User) usersDataUserModel {
	m := usersDataUserModel{
		ID:                         types.StringValue(u.Id),
		PrimaryEmail:               types.StringValue(u.PrimaryEmail),
		GivenName:                  types.StringValue(""),
		FamilyName:                 types.StringValue(""),
		OrgUnitPath:                types.StringValue(u.OrgUnitPath),
		Suspended:                  types.BoolValue(u.Suspended),
		Archived:                   types.BoolValue(u.Archived),
		IsAdmin:                    types.BoolValue(u.IsAdmin),
		RecoveryEmail:              stringOrNull(u.RecoveryEmail),
		RecoveryPhone:              stringOrNull(u.RecoveryPhone),
		IncludeInGlobalAddressList: types.BoolValue(u.IncludeInGlobalAddressList),
		CreationTime:               types.StringValue(u.CreationTime),
		LastLoginTime:              types.StringValue(u.LastLoginTime),
	}
	if u.Name != nil {
		m.GivenName = types.StringValue(u.Name.GivenName)
		m.FamilyName = types.StringValue(u.Name.FamilyName)
	}
	return m
}
