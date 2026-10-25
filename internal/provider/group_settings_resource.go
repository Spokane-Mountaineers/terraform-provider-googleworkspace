package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/api/groupssettings/v1"

	"github.com/Spokane-Mountaineers/terraform-provider-googleworkspace/internal/client"
)

var (
	_ resource.Resource                = (*groupSettingsResource)(nil)
	_ resource.ResourceWithConfigure   = (*groupSettingsResource)(nil)
	_ resource.ResourceWithImportState = (*groupSettingsResource)(nil)
	_ resource.ResourceWithModifyPlan  = (*groupSettingsResource)(nil)
)

type groupSettingsResource struct {
	client *client.Client
}

// NewGroupSettingsResource is the resource factory.
func NewGroupSettingsResource() resource.Resource {
	return &groupSettingsResource{}
}

// groupSetting maps one Terraform attribute to a Groups Settings API field. The API
// carries booleans as "true"/"false" strings; values is nil for those.
type groupSetting struct {
	name        string
	description string
	values      []string
	get         func(*groupssettings.Groups) string
	set         func(*groupssettings.Groups, string)
}

func (s groupSetting) isBool() bool { return s.values == nil }

// groupSettings are the managed settings. Deprecated API fields are left out; their
// replacements (who_can_moderate_*, who_can_discover_group) are here.
var groupSettings = []groupSetting{
	{
		name: "who_can_post_message",
		description: "Who can send mail to the group. `NONE_CAN_POST` is only accepted while `archive_only` is " +
			"true, and setting `archive_only` sets it.",
		values: []string{"NONE_CAN_POST", "ALL_MANAGERS_CAN_POST", "ALL_MEMBERS_CAN_POST", "ALL_OWNERS_CAN_POST", "ALL_IN_DOMAIN_CAN_POST", "ANYONE_CAN_POST"},
		get:    func(g *groupssettings.Groups) string { return g.WhoCanPostMessage },
		set:    func(g *groupssettings.Groups, v string) { g.WhoCanPostMessage = v },
	},
	{
		name:        "who_can_join",
		description: "Who can join the group. `ANYONE_CAN_JOIN` includes people outside the organization.",
		values:      []string{"ANYONE_CAN_JOIN", "ALL_IN_DOMAIN_CAN_JOIN", "INVITED_CAN_JOIN", "CAN_REQUEST_TO_JOIN"},
		get:         func(g *groupssettings.Groups) string { return g.WhoCanJoin },
		set:         func(g *groupssettings.Groups, v string) { g.WhoCanJoin = v },
	},
	{
		name:        "who_can_view_membership",
		description: "Who can see the member list.",
		values:      []string{"ALL_IN_DOMAIN_CAN_VIEW", "ALL_MEMBERS_CAN_VIEW", "ALL_MANAGERS_CAN_VIEW", "ALL_OWNERS_CAN_VIEW"},
		get:         func(g *groupssettings.Groups) string { return g.WhoCanViewMembership },
		set:         func(g *groupssettings.Groups, v string) { g.WhoCanViewMembership = v },
	},
	{
		name:        "who_can_view_group",
		description: "Who can read the group's conversations.",
		values:      []string{"ANYONE_CAN_VIEW", "ALL_IN_DOMAIN_CAN_VIEW", "ALL_MEMBERS_CAN_VIEW", "ALL_MANAGERS_CAN_VIEW", "ALL_OWNERS_CAN_VIEW"},
		get:         func(g *groupssettings.Groups) string { return g.WhoCanViewGroup },
		set:         func(g *groupssettings.Groups, v string) { g.WhoCanViewGroup = v },
	},
	{
		name:        "who_can_discover_group",
		description: "Who can find the group in the groups directory.",
		values:      []string{"ANYONE_CAN_DISCOVER", "ALL_IN_DOMAIN_CAN_DISCOVER", "ALL_MEMBERS_CAN_DISCOVER"},
		get:         func(g *groupssettings.Groups) string { return g.WhoCanDiscoverGroup },
		set:         func(g *groupssettings.Groups, v string) { g.WhoCanDiscoverGroup = v },
	},
	{
		name:        "who_can_contact_owner",
		description: "Who can email the group's owners.",
		values:      []string{"ALL_IN_DOMAIN_CAN_CONTACT", "ALL_MANAGERS_CAN_CONTACT", "ALL_MEMBERS_CAN_CONTACT", "ANYONE_CAN_CONTACT"},
		get:         func(g *groupssettings.Groups) string { return g.WhoCanContactOwner },
		set:         func(g *groupssettings.Groups, v string) { g.WhoCanContactOwner = v },
	},
	{
		name:        "who_can_leave_group",
		description: "Who can leave the group on their own.",
		values:      []string{"ALL_MANAGERS_CAN_LEAVE", "ALL_MEMBERS_CAN_LEAVE", "NONE_CAN_LEAVE"},
		get:         func(g *groupssettings.Groups) string { return g.WhoCanLeaveGroup },
		set:         func(g *groupssettings.Groups, v string) { g.WhoCanLeaveGroup = v },
	},
	{
		name:        "who_can_moderate_members",
		description: "Who can add, remove, approve, and ban members.",
		values:      []string{"ALL_MEMBERS", "OWNERS_AND_MANAGERS", "OWNERS_ONLY", "NONE"},
		get:         func(g *groupssettings.Groups) string { return g.WhoCanModerateMembers },
		set:         func(g *groupssettings.Groups, v string) { g.WhoCanModerateMembers = v },
	},
	{
		name:        "who_can_moderate_content",
		description: "Who can approve, delete, and move messages.",
		values:      []string{"ALL_MEMBERS", "OWNERS_AND_MANAGERS", "OWNERS_ONLY", "NONE"},
		get:         func(g *groupssettings.Groups) string { return g.WhoCanModerateContent },
		set:         func(g *groupssettings.Groups, v string) { g.WhoCanModerateContent = v },
	},
	{
		name:        "message_moderation_level",
		description: "Which messages wait for a moderator before delivery.",
		values:      []string{"MODERATE_ALL_MESSAGES", "MODERATE_NON_MEMBERS", "MODERATE_NEW_MEMBERS", "MODERATE_NONE"},
		get:         func(g *groupssettings.Groups) string { return g.MessageModerationLevel },
		set:         func(g *groupssettings.Groups, v string) { g.MessageModerationLevel = v },
	},
	{
		name:        "spam_moderation_level",
		description: "What happens to messages flagged as spam.",
		values:      []string{"ALLOW", "MODERATE", "SILENTLY_MODERATE", "REJECT"},
		get:         func(g *groupssettings.Groups) string { return g.SpamModerationLevel },
		set:         func(g *groupssettings.Groups, v string) { g.SpamModerationLevel = v },
	},
	{
		name: "allow_external_members",
		description: "Whether people outside the organization can be members. Workspace must also allow external " +
			"members for the organization (Admin console > Apps > Google Groups for Business).",
		get: func(g *groupssettings.Groups) string { return g.AllowExternalMembers },
		set: func(g *groupssettings.Groups, v string) { g.AllowExternalMembers = v },
	},
	{
		name:        "allow_web_posting",
		description: "Whether members can post from the Google Groups web interface, not only by email.",
		get:         func(g *groupssettings.Groups) string { return g.AllowWebPosting },
		set:         func(g *groupssettings.Groups, v string) { g.AllowWebPosting = v },
	},
	{
		name:        "include_in_global_address_list",
		description: "Whether the group appears in the Global Address List and in address autocomplete.",
		get:         func(g *groupssettings.Groups) string { return g.IncludeInGlobalAddressList },
		set:         func(g *groupssettings.Groups, v string) { g.IncludeInGlobalAddressList = v },
	},
	{
		name:        "members_can_post_as_the_group",
		description: "Whether members can send mail with the group's address as the sender.",
		get:         func(g *groupssettings.Groups) string { return g.MembersCanPostAsTheGroup },
		set:         func(g *groupssettings.Groups, v string) { g.MembersCanPostAsTheGroup = v },
	},
	{
		name:        "is_archived",
		description: "Whether messages sent to the group are kept in its web archive.",
		get:         func(g *groupssettings.Groups) string { return g.IsArchived },
		set:         func(g *groupssettings.Groups, v string) { g.IsArchived = v },
	},
	{
		name: "enable_collaborative_inbox",
		description: "Whether the group is a collaborative inbox, where members can take, assign, and resolve " +
			"conversations. Google requires conversation history (`is_archived`) for collaborative inbox features " +
			"([Make a group a Collaborative Inbox](https://support.google.com/a/users/answer/10375787)). The Groups " +
			"Settings API reference does not say whether it rejects `true` while `is_archived` is false, so the plan " +
			"warns about that combination instead of failing.",
		get: func(g *groupssettings.Groups) string { return g.EnableCollaborativeInbox },
		set: func(g *groupssettings.Groups, v string) { g.EnableCollaborativeInbox = v },
	},
	{
		name: "archive_only",
		description: "Makes the group read-only: new messages are rejected and `who_can_post_message` becomes " +
			"`NONE_CAN_POST`. Turning it off resets `who_can_post_message` to `ALL_MANAGERS_CAN_POST`.",
		get: func(g *groupssettings.Groups) string { return g.ArchiveOnly },
		set: func(g *groupssettings.Groups, v string) { g.ArchiveOnly = v },
	},
}

func (r *groupSettingsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group_settings"
}

func (r *groupSettingsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	attrs := map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:            true,
			PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			MarkdownDescription: "The group's email address. Used for import.",
		},
		"email": schema.StringAttribute{
			Required:            true,
			PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
			MarkdownDescription: "Primary email of the group, e.g. `googleworkspace_group.example.email`.",
		},
	}
	for _, s := range groupSettings {
		if s.isBool() {
			attrs[s.name] = schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
				MarkdownDescription: s.description,
			}
			continue
		}
		attrs[s.name] = schema.StringAttribute{
			Optional:            true,
			Computed:            true,
			PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			Validators:          []validator.String{stringvalidator.OneOf(s.values...)},
			MarkdownDescription: s.description + " One of " + backtickList(s.values) + ".",
		}
	}

	resp.Schema = schema.Schema{
		MarkdownDescription: "Settings of a Google Workspace group, such as who can post, who can join, and whether " +
			"people outside the organization can be members. Uses the Groups Settings API.\n\n" +
			"Only the settings set in configuration are managed; the rest are read so they show in state, and left " +
			"as they are. Settings cannot be deleted, so destroying this resource only removes it from state, and " +
			"the group keeps its current settings.\n\n" +
			"~> **Requirements:** the provider must use `service_account` (a pre-minted `access_token` cannot be " +
			"used), the service account's domain-wide delegation must include the " +
			"`https://www.googleapis.com/auth/apps.groups.settings` scope, and the Groups Settings API must be " +
			"enabled in its project.",
		Attributes: attrs,
	}
}

func (r *groupSettingsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// ModifyPlan warns when collaborative inbox is planned without conversation
// history, and marks who_can_post_message unknown when archive_only changes and
// the configuration leaves it unset, since Workspace changes it as a side effect.
func (r *groupSettingsResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}
	resp.Diagnostics.Append(warnCollaborativeInboxWithoutHistory(ctx, req.Plan)...)
	if req.State.Raw.IsNull() || resp.Diagnostics.HasError() {
		return
	}
	var planned, prior types.Bool
	var configured types.String
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("archive_only"), &planned)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("archive_only"), &prior)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("who_can_post_message"), &configured)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if configured.IsNull() && !planned.IsUnknown() && !planned.Equal(prior) {
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("who_can_post_message"), types.StringUnknown())...)
	}
}

// warnCollaborativeInboxWithoutHistory warns when the plan turns on collaborative
// inbox while is_archived (conversation history) is known to be false, whether
// configured or kept from state. Google's help center says the features need
// conversation history; whether the API rejects the combination is unverified.
func warnCollaborativeInboxWithoutHistory(ctx context.Context, plan planReader) diag.Diagnostics {
	var diags diag.Diagnostics
	var inbox, archived types.Bool
	diags.Append(plan.GetAttribute(ctx, path.Root("enable_collaborative_inbox"), &inbox)...)
	diags.Append(plan.GetAttribute(ctx, path.Root("is_archived"), &archived)...)
	if diags.HasError() {
		return diags
	}
	if inbox.IsUnknown() || archived.IsUnknown() || !inbox.ValueBool() || archived.IsNull() || archived.ValueBool() {
		return diags
	}
	diags.AddAttributeWarning(path.Root("enable_collaborative_inbox"),
		"Collaborative inbox without conversation history",
		"enable_collaborative_inbox is true but is_archived is false. Google requires conversation history "+
			"(is_archived) for Collaborative Inbox features (https://support.google.com/a/users/answer/10375787); "+
			"set is_archived = true.")
	return diags
}

func (r *groupSettingsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var email types.String
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("email"), &email)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(r.apply(ctx, email.ValueString(), req.Plan, &resp.State)...)
}

func (r *groupSettingsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var email types.String
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("email"), &email)...)
	if resp.Diagnostics.HasError() {
		return
	}

	svc, err := r.client.GroupSettings()
	if err != nil {
		resp.Diagnostics.AddError("Unable to configure Groups Settings client", err.Error())
		return
	}
	g, err := svc.Groups.Get(email.ValueString()).Context(ctx).Do()
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read group settings", err.Error())
		return
	}

	resp.Diagnostics.Append(setGroupSettingsState(ctx, email.ValueString(), g, &resp.State)...)
}

func (r *groupSettingsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var email types.String
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root("email"), &email)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(r.apply(ctx, email.ValueString(), req.Plan, &resp.State)...)
}

// Delete leaves the group's settings as they are: they cannot be removed, and
// resetting them to defaults could reopen a group that was locked down.
func (r *groupSettingsResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}

func (r *groupSettingsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if !strings.Contains(req.ID, "@") {
		resp.Diagnostics.AddError("Invalid group settings ID", fmt.Sprintf("expected the group's email address, got %q", req.ID))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("email"), req.ID)...)
}

// apply patches the settings known in plan, then records what Workspace reports.
func (r *groupSettingsResource) apply(ctx context.Context, email string, plan planReader, state stateWriter) diag.Diagnostics {
	var diags diag.Diagnostics

	patch, d := groupSettingsFromPlan(ctx, plan)
	diags.Append(d...)
	if diags.HasError() {
		return diags
	}

	svc, err := r.client.GroupSettings()
	if err != nil {
		diags.AddError("Unable to configure Groups Settings client", err.Error())
		return diags
	}

	var g *groupssettings.Groups
	// Retry: a group created in the same apply may not be visible to this API yet.
	err = withRetry(ctx, func() error {
		var e error
		g, e = svc.Groups.Patch(email, patch).Context(ctx).Do()
		return e
	})
	if err != nil {
		diags.AddError("Unable to update group settings", err.Error())
		return diags
	}

	diags.Append(setGroupSettingsState(ctx, email, g, state)...)
	return diags
}

// planReader and stateWriter are the parts of tfsdk.Plan and tfsdk.State used here.
type planReader interface {
	GetAttribute(context.Context, path.Path, any) diag.Diagnostics
}

type stateWriter interface {
	SetAttribute(context.Context, path.Path, any) diag.Diagnostics
}

// groupSettingsFromPlan builds a patch from the settings with known values. Unknown
// ones (unset in configuration, on create) are left out so Workspace keeps them.
func groupSettingsFromPlan(ctx context.Context, plan planReader) (*groupssettings.Groups, diag.Diagnostics) {
	var diags diag.Diagnostics
	patch := &groupssettings.Groups{}
	for _, s := range groupSettings {
		if s.isBool() {
			var b types.Bool
			diags.Append(plan.GetAttribute(ctx, path.Root(s.name), &b)...)
			if !b.IsNull() && !b.IsUnknown() {
				s.set(patch, strconv.FormatBool(b.ValueBool()))
			}
			continue
		}
		var str types.String
		diags.Append(plan.GetAttribute(ctx, path.Root(s.name), &str)...)
		if !str.IsNull() && !str.IsUnknown() {
			s.set(patch, str.ValueString())
		}
	}
	return patch, diags
}

func setGroupSettingsState(ctx context.Context, email string, g *groupssettings.Groups, state stateWriter) diag.Diagnostics {
	var diags diag.Diagnostics
	diags.Append(state.SetAttribute(ctx, path.Root("id"), email)...)
	diags.Append(state.SetAttribute(ctx, path.Root("email"), email)...)
	for _, s := range groupSettings {
		raw := s.get(g)
		if s.isBool() {
			diags.Append(state.SetAttribute(ctx, path.Root(s.name), apiBool(raw))...)
			continue
		}
		if raw == "" {
			diags.Append(state.SetAttribute(ctx, path.Root(s.name), types.StringNull())...)
			continue
		}
		diags.Append(state.SetAttribute(ctx, path.Root(s.name), raw)...)
	}
	return diags
}

// apiBool converts the API's "true"/"false" strings; anything else is null.
func apiBool(raw string) types.Bool {
	b, err := strconv.ParseBool(raw)
	if err != nil {
		return types.BoolNull()
	}
	return types.BoolValue(b)
}

func backtickList(values []string) string {
	quoted := make([]string, len(values))
	for i, v := range values {
		quoted[i] = "`" + v + "`"
	}
	return strings.Join(quoted, ", ")
}
