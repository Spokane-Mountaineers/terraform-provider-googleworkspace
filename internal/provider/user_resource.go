package provider

import (
	"context"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	admin "google.golang.org/api/admin/directory/v1"

	"github.com/Spokane-Mountaineers/terraform-provider-googleworkspace/internal/client"
)

var (
	_ resource.Resource                = (*userResource)(nil)
	_ resource.ResourceWithConfigure   = (*userResource)(nil)
	_ resource.ResourceWithImportState = (*userResource)(nil)
)

// e164 matches an E.164 phone number, the only format the Admin SDK accepts for
// recoveryPhone.
var e164 = regexp.MustCompile(`^\+[1-9]\d{1,14}$`)

type userResource struct {
	client *client.Client
}

// NewUserResource is the resource factory.
func NewUserResource() resource.Resource {
	return &userResource{}
}

type userModel struct {
	ID                         types.String `tfsdk:"id"`
	PrimaryEmail               types.String `tfsdk:"primary_email"`
	GivenName                  types.String `tfsdk:"given_name"`
	FamilyName                 types.String `tfsdk:"family_name"`
	OrgUnitPath                types.String `tfsdk:"org_unit_path"`
	Suspended                  types.Bool   `tfsdk:"suspended"`
	Archived                   types.Bool   `tfsdk:"archived"`
	RecoveryEmail              types.String `tfsdk:"recovery_email"`
	RecoveryPhone              types.String `tfsdk:"recovery_phone"`
	IncludeInGlobalAddressList types.Bool   `tfsdk:"include_in_global_address_list"`
	ChangePasswordAtNextLogin  types.Bool   `tfsdk:"change_password_at_next_login"`
	PasswordWO                 types.String `tfsdk:"password_wo"`
	PasswordWOVersion          types.String `tfsdk:"password_wo_version"`
}

func (r *userResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (r *userResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A Google Workspace user account.\n\n" +
			"Existing accounts are adopted with an `import` block or `tofu import`, by user ID or primary email. " +
			"Creating an account requires `password_wo`; importing one does not.\n\n" +
			"~> Destroying this resource **deletes the account**. Google keeps a deleted user restorable for 20 days. " +
			"To offboard someone, set `suspended = true` instead, and consider `lifecycle { prevent_destroy = true }`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				MarkdownDescription: "Immutable user ID. Used for import (the primary email also works).",
			},
			"primary_email": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "The user's primary email address. Changing it renames the account in place; " +
					"Workspace keeps the old address as an alias.",
			},
			"given_name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The user's first name.",
			},
			"family_name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The user's last name.",
			},
			"org_unit_path": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("/"),
				MarkdownDescription: "Full path of the organizational unit the user belongs to. Defaults to `/`.",
			},
			"suspended": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "Whether the account is suspended. Suspension is the reversible way to offboard a user.",
			},
			"archived": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "Whether the account is archived. Requires an Archived User license.",
			},
			"recovery_email": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Recovery email address, used for self-service password reset. Omit to clear it.",
			},
			"recovery_phone": schema.StringAttribute{
				Optional: true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(e164, "must be an E.164 phone number, e.g. +15095550100"),
				},
				MarkdownDescription: "Recovery phone number in E.164 format, e.g. `+15095550100`. Omit to clear it.",
			},
			"include_in_global_address_list": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
				MarkdownDescription: "Whether the user appears in the domain's global address list. Defaults to `true`.",
			},
			"change_password_at_next_login": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
				MarkdownDescription: "Whether the user must change the password at next sign-in. Sent only together " +
					"with a password (on create, or when `password_wo_version` changes). It is not read back, because " +
					"Workspace clears the flag once the user signs in.",
			},
			"password_wo": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				WriteOnly: true,
				MarkdownDescription: "Password for the account. **Write-only**: never stored in state. Required on " +
					"create; ignored afterwards unless `password_wo_version` changes. Pair it with an ephemeral " +
					"`random_password` so the value never touches disk.",
			},
			"password_wo_version": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "Arbitrary version marker for `password_wo`. Changing it sends the current " +
					"`password_wo` to Workspace, which is how a password is rotated.",
			},
		},
	}
}

func (r *userResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type", fmt.Sprintf("expected *client.Client, got %T", req.ProviderData))
		return
	}
	r.client = c
}

func (r *userResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan, config userModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	// Write-only values exist only in config, never in the plan.
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if config.PasswordWO.IsNull() {
		resp.Diagnostics.AddAttributeError(path.Root("password_wo"), "Missing password",
			"Workspace requires a password to create a user. Set password_wo (an ephemeral random_password works), "+
				"or import the existing account instead.")
		return
	}

	u := modelToUser(plan)
	u.Password = config.PasswordWO.ValueString()
	u.ChangePasswordAtNextLogin = plan.ChangePasswordAtNextLogin.ValueBool()

	created, err := r.client.Directory.Users.Insert(u).Context(ctx).Do()
	if err != nil {
		resp.Diagnostics.AddError("Unable to create user", err.Error())
		return
	}

	// A fresh user can 404 on read until it propagates.
	var got *admin.User
	err = withRetry(ctx, func() error {
		var e error
		got, e = r.client.Directory.Users.Get(created.Id).Context(ctx).Do()
		return e
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to read created user", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, userToModel(got, plan))...)
}

func (r *userResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state userModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	u, err := r.client.Directory.Users.Get(state.ID.ValueString()).Context(ctx).Do()
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read user", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, userToModel(u, state))...)
}

func (r *userResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state, config userModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	u := modelToUser(plan)
	if !plan.PasswordWOVersion.Equal(state.PasswordWOVersion) {
		if config.PasswordWO.IsNull() {
			resp.Diagnostics.AddAttributeError(path.Root("password_wo"), "Missing password",
				"password_wo_version changed but password_wo is not set.")
			return
		}
		u.Password = config.PasswordWO.ValueString()
		u.ChangePasswordAtNextLogin = plan.ChangePasswordAtNextLogin.ValueBool()
		u.ForceSendFields = append(u.ForceSendFields, "ChangePasswordAtNextLogin")
	}

	updated, err := r.client.Directory.Users.Update(state.ID.ValueString(), u).Context(ctx).Do()
	if err != nil {
		resp.Diagnostics.AddError("Unable to update user", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, userToModel(updated, plan))...)
}

func (r *userResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state userModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.Directory.Users.Delete(state.ID.ValueString()).Context(ctx).Do(); err != nil {
		if isNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Unable to delete user", err.Error())
	}
}

// ImportState accepts a user ID or a primary email; Read resolves either, and the
// state is then keyed by the immutable ID.
func (r *userResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	u, err := r.client.Directory.Users.Get(req.ID).Context(ctx).Do()
	if err != nil {
		resp.Diagnostics.AddError("Unable to import user", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), u.Id)...)
}

// modelToUser builds the API payload for the managed fields. Booleans and the
// recovery fields are force-sent so that false and "" clear values in Workspace
// instead of being dropped as zero values.
func modelToUser(m userModel) *admin.User {
	return &admin.User{
		PrimaryEmail: m.PrimaryEmail.ValueString(),
		Name: &admin.UserName{
			GivenName:  m.GivenName.ValueString(),
			FamilyName: m.FamilyName.ValueString(),
		},
		OrgUnitPath:                m.OrgUnitPath.ValueString(),
		Suspended:                  m.Suspended.ValueBool(),
		Archived:                   m.Archived.ValueBool(),
		RecoveryEmail:              m.RecoveryEmail.ValueString(),
		RecoveryPhone:              m.RecoveryPhone.ValueString(),
		IncludeInGlobalAddressList: m.IncludeInGlobalAddressList.ValueBool(),
		ForceSendFields: []string{
			"Suspended", "Archived", "IncludeInGlobalAddressList", "RecoveryEmail", "RecoveryPhone",
		},
	}
}

// userToModel maps an API user onto the model. Fields Workspace never returns
// (the write-only password, its version, and the one-shot change-password flag)
// are carried over from prior.
func userToModel(u *admin.User, prior userModel) userModel {
	m := userModel{
		ID:                         types.StringValue(u.Id),
		PrimaryEmail:               types.StringValue(u.PrimaryEmail),
		OrgUnitPath:                types.StringValue(u.OrgUnitPath),
		Suspended:                  types.BoolValue(u.Suspended),
		Archived:                   types.BoolValue(u.Archived),
		RecoveryEmail:              stringOrNull(u.RecoveryEmail),
		RecoveryPhone:              stringOrNull(u.RecoveryPhone),
		IncludeInGlobalAddressList: types.BoolValue(u.IncludeInGlobalAddressList),
		ChangePasswordAtNextLogin:  prior.ChangePasswordAtNextLogin,
		PasswordWO:                 types.StringNull(),
		PasswordWOVersion:          prior.PasswordWOVersion,
	}
	if u.Name != nil {
		m.GivenName = types.StringValue(u.Name.GivenName)
		m.FamilyName = types.StringValue(u.Name.FamilyName)
	} else {
		m.GivenName = types.StringValue("")
		m.FamilyName = types.StringValue("")
	}
	// After import, prior holds only the ID; settle the one-shot flag on its default.
	if m.ChangePasswordAtNextLogin.IsNull() || m.ChangePasswordAtNextLogin.IsUnknown() {
		m.ChangePasswordAtNextLogin = types.BoolValue(false)
	}
	return m
}

// stringOrNull maps the API's empty string to null, so an omitted optional
// attribute matches an unset value in Workspace.
func stringOrNull(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}
