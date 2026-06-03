package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	admin "google.golang.org/api/admin/directory/v1"

	"github.com/Spokane-Mountaineers/terraform-provider-googleworkspace/internal/client"
)

var (
	_ resource.Resource                = (*groupMemberResource)(nil)
	_ resource.ResourceWithConfigure   = (*groupMemberResource)(nil)
	_ resource.ResourceWithImportState = (*groupMemberResource)(nil)
)

type groupMemberResource struct {
	client *client.Client
}

// NewGroupMemberResource is the resource factory.
func NewGroupMemberResource() resource.Resource {
	return &groupMemberResource{}
}

type groupMemberModel struct {
	ID               types.String `tfsdk:"id"`
	GroupID          types.String `tfsdk:"group_id"`
	Email            types.String `tfsdk:"email"`
	Role             types.String `tfsdk:"role"`
	Type             types.String `tfsdk:"type"`
	DeliverySettings types.String `tfsdk:"delivery_settings"`
	MemberID         types.String `tfsdk:"member_id"`
}

func (r *groupMemberResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group_member"
}

func (r *groupMemberResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	requiresReplace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		MarkdownDescription: "Membership of a user or group in a Google Workspace group.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				MarkdownDescription: "Composite ID, `<group_id>/<member_id>`. Used for import.",
			},
			"group_id": schema.StringAttribute{
				Required:            true,
				PlanModifiers:       requiresReplace,
				MarkdownDescription: "Immutable ID of the group. Changing this forces a new membership.",
			},
			"email": schema.StringAttribute{
				Required:            true,
				PlanModifiers:       requiresReplace,
				MarkdownDescription: "Email of the member (a user or group). Changing this forces a new membership.",
			},
			"role": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Membership role: `MEMBER`, `MANAGER`, or `OWNER`. Defaults to `MEMBER`.",
			},
			"delivery_settings": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Mail delivery preference: `ALL_MAIL`, `DAILY`, `DIGEST`, `NONE`, or `DISABLED`.",
			},
			"type": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Type of member: `USER`, `GROUP`, `EXTERNAL`, or `CUSTOMER`.",
			},
			"member_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Immutable ID of the member.",
			},
		},
	}
}

func (r *groupMemberResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *groupMemberResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan groupMemberModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	groupID := plan.GroupID.ValueString()
	var m *admin.Member
	// Retry: a member can be added before a just-created group is fully visible.
	err := withRetry(ctx, func() error {
		var e error
		m, e = r.client.Directory.Members.Insert(groupID, &admin.Member{
			Email:            plan.Email.ValueString(),
			Role:             plan.Role.ValueString(),
			DeliverySettings: plan.DeliverySettings.ValueString(),
		}).Context(ctx).Do()
		return e
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to add group member", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, memberToModel(groupID, m))...)
}

func (r *groupMemberResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state groupMemberModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	groupID, memberID, ok := splitMemberID(state.ID.ValueString())
	if !ok {
		resp.Diagnostics.AddError("Invalid group member ID", fmt.Sprintf("expected `<group_id>/<member_id>`, got %q", state.ID.ValueString()))
		return
	}

	m, err := r.client.Directory.Members.Get(groupID, memberID).Context(ctx).Do()
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read group member", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, memberToModel(groupID, m))...)
}

func (r *groupMemberResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state groupMemberModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	groupID := state.GroupID.ValueString()
	m, err := r.client.Directory.Members.Update(groupID, state.MemberID.ValueString(), &admin.Member{
		Role:             plan.Role.ValueString(),
		DeliverySettings: plan.DeliverySettings.ValueString(),
	}).Context(ctx).Do()
	if err != nil {
		resp.Diagnostics.AddError("Unable to update group member", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, memberToModel(groupID, m))...)
}

func (r *groupMemberResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state groupMemberModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.Directory.Members.Delete(state.GroupID.ValueString(), state.MemberID.ValueString()).Context(ctx).Do(); err != nil {
		if isNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Unable to remove group member", err.Error())
	}
}

func (r *groupMemberResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func memberToModel(groupID string, m *admin.Member) groupMemberModel {
	return groupMemberModel{
		ID:               types.StringValue(groupID + "/" + m.Id),
		GroupID:          types.StringValue(groupID),
		Email:            types.StringValue(m.Email),
		Role:             types.StringValue(m.Role),
		Type:             types.StringValue(m.Type),
		DeliverySettings: types.StringValue(m.DeliverySettings),
		MemberID:         types.StringValue(m.Id),
	}
}

func splitMemberID(id string) (groupID, memberID string, ok bool) {
	groupID, memberID, found := strings.Cut(id, "/")
	if !found || groupID == "" || memberID == "" {
		return "", "", false
	}
	return groupID, memberID, true
}
