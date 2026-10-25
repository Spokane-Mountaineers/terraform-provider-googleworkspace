package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	admin "google.golang.org/api/admin/directory/v1"

	"github.com/Spokane-Mountaineers/terraform-provider-googleworkspace/internal/client"
)

var (
	_ resource.Resource                = (*groupAliasResource)(nil)
	_ resource.ResourceWithConfigure   = (*groupAliasResource)(nil)
	_ resource.ResourceWithImportState = (*groupAliasResource)(nil)
)

type groupAliasResource struct {
	client *client.Client
}

// NewGroupAliasResource is the resource factory.
func NewGroupAliasResource() resource.Resource {
	return &groupAliasResource{}
}

type groupAliasModel struct {
	ID      types.String `tfsdk:"id"`
	GroupID types.String `tfsdk:"group_id"`
	Alias   types.String `tfsdk:"alias"`
}

func (r *groupAliasResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group_alias"
}

func (r *groupAliasResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	requiresReplace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		MarkdownDescription: "An additional email address (alias) for a Google Workspace group. Mail sent to the " +
			"alias is delivered to the group as if it were sent to the group's primary address, which suits role " +
			"addresses such as `trips@` that should reach an existing list.\n\n" +
			"Aliases cannot be changed in place, so changing either attribute removes the alias and adds a new one. " +
			"Removing an alias stops mail delivery to that address.\n\n" +
			"~> **The address must be free.** An alias cannot be an address that a user or group in the " +
			"organization still uses, as its primary address or as an alias " +
			"([Add or delete an alternate email address](https://support.google.com/a/answer/33327), " +
			"[Resolve duplicate account conflicts](https://knowledge.workspace.google.com/p/duplicate-account-errors)). To move an " +
			"address from a user or group to this group, delete or rename that user or group and remove the old " +
			"alias first (renaming a user keeps the old address as an alias), in an earlier apply than the one that " +
			"adds this alias: Terraform does not order deleting one resource before creating an unrelated one.\n\n" +
			"Google allows up to 30 aliases per group, reserves some local parts (such as `abuse`, `admin`, " +
			"`postmaster`, and `webmaster`), and notes that alias changes can take up to 24 hours to reach mail " +
			"delivery ([Give a group an additional alias address](https://support.google.com/a/answer/2727156)).\n\n" +
			"~> **Requirements:** the service account's domain-wide delegation must include the " +
			"`https://www.googleapis.com/auth/admin.directory.group` scope, which the provider requests by default.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				MarkdownDescription: "Composite ID, `<group_id>/<alias>`. Used for import.",
			},
			"group_id": schema.StringAttribute{
				Required:            true,
				PlanModifiers:       requiresReplace,
				MarkdownDescription: "Immutable ID of the group, e.g. `googleworkspace_group.example.id`. Changing this forces a new alias.",
			},
			"alias": schema.StringAttribute{
				Required:      true,
				PlanModifiers: requiresReplace,
				MarkdownDescription: "The alias email address, in one of the account's domains. It must not be in use " +
					"by any user or group. Changing this forces a new alias.",
			},
		},
	}
}

func (r *groupAliasResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *groupAliasResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan groupAliasModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	groupID, alias := plan.GroupID.ValueString(), plan.Alias.ValueString()
	// Retry: an alias can be added before a just-created group is fully visible.
	err := withRetry(ctx, func() error {
		_, e := r.client.Directory.Groups.Aliases.Insert(groupID, &admin.Alias{Alias: alias}).Context(ctx).Do()
		return e
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to add group alias", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, aliasToModel(groupID, alias))...)
}

func (r *groupAliasResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state groupAliasModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	groupID := state.GroupID.ValueString()
	list, err := r.client.Directory.Groups.Aliases.List(groupID).Context(ctx).Do()
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read group aliases", err.Error())
		return
	}
	aliases, err := decodeAliases(list)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read group aliases", err.Error())
		return
	}
	if !hasAlias(aliases, state.Alias.ValueString()) {
		resp.State.RemoveResource(ctx)
		return
	}

	// Keep the configured spelling: Workspace may report the address in lower case.
	resp.Diagnostics.Append(resp.State.Set(ctx, aliasToModel(groupID, state.Alias.ValueString()))...)
}

// Update is never called: every configurable attribute forces replacement.
func (r *groupAliasResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Unexpected update", "googleworkspace_group_alias has no updatable attributes")
}

func (r *groupAliasResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state groupAliasModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.Directory.Groups.Aliases.Delete(state.GroupID.ValueString(), state.Alias.ValueString()).Context(ctx).Do(); err != nil {
		if isNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Unable to remove group alias", err.Error())
	}
}

// ImportState accepts `<group_id>/<alias>` or `<group_email>/<alias>`. A group email
// is resolved to the group's immutable ID so the import matches configuration that
// uses `googleworkspace_group.<name>.id`.
func (r *groupAliasResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	group, alias, ok := splitAliasID(req.ID)
	if !ok {
		resp.Diagnostics.AddError("Invalid group alias ID",
			fmt.Sprintf("expected `<group_id>/<alias>` or `<group_email>/<alias>`, got %q", req.ID))
		return
	}

	groupID := group
	if strings.Contains(group, "@") {
		g, err := r.client.Directory.Groups.Get(group).Fields("id").Context(ctx).Do()
		if err != nil {
			resp.Diagnostics.AddError("Unable to look up group", fmt.Sprintf("resolving %q to a group ID: %s", group, err))
			return
		}
		groupID = g.Id
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, aliasToModel(groupID, alias))...)
}

func aliasToModel(groupID, alias string) groupAliasModel {
	return groupAliasModel{
		ID:      types.StringValue(groupID + "/" + alias),
		GroupID: types.StringValue(groupID),
		Alias:   types.StringValue(alias),
	}
}

// decodeAliases converts a list response, whose items the client library leaves
// untyped, into aliases.
func decodeAliases(list *admin.Aliases) ([]admin.Alias, error) {
	if list == nil || len(list.Aliases) == 0 {
		return nil, nil
	}
	raw, err := json.Marshal(list.Aliases)
	if err != nil {
		return nil, fmt.Errorf("encoding alias list: %w", err)
	}
	var aliases []admin.Alias
	if err := json.Unmarshal(raw, &aliases); err != nil {
		return nil, fmt.Errorf("decoding alias list: %w", err)
	}
	return aliases, nil
}

// hasAlias reports whether alias is among aliases. Email addresses are compared
// without regard to case.
func hasAlias(aliases []admin.Alias, alias string) bool {
	for _, a := range aliases {
		if strings.EqualFold(a.Alias, alias) {
			return true
		}
	}
	return false
}

func splitAliasID(id string) (group, alias string, ok bool) {
	group, alias, found := strings.Cut(id, "/")
	if !found || group == "" || !strings.Contains(alias, "@") {
		return "", "", false
	}
	return group, alias, true
}
