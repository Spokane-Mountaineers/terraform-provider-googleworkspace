package provider

import (
	"context"
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
	_ resource.Resource                = (*userAliasResource)(nil)
	_ resource.ResourceWithConfigure   = (*userAliasResource)(nil)
	_ resource.ResourceWithImportState = (*userAliasResource)(nil)
)

type userAliasResource struct {
	client *client.Client
}

// NewUserAliasResource is the resource factory.
func NewUserAliasResource() resource.Resource {
	return &userAliasResource{}
}

type userAliasModel struct {
	ID     types.String `tfsdk:"id"`
	UserID types.String `tfsdk:"user_id"`
	Alias  types.String `tfsdk:"alias"`
}

func (r *userAliasResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_alias"
}

func (r *userAliasResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	requiresReplace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		MarkdownDescription: "An additional email address (alias) for a Google Workspace user. Mail sent to the " +
			"alias is delivered to the user's mailbox, and the user can sign in with it.\n\n" +
			"Aliases cannot be changed in place, so changing either attribute removes the alias and adds a new one. " +
			"Removing an alias stops mail delivery to that address.\n\n" +
			"-> **Renaming a user adds an alias.** When a user's primary email changes, Workspace keeps the old " +
			"address as an alias. That alias is not created by this resource, so import it to manage it, or to " +
			"remove it and free the address.\n\n" +
			"~> **The address must be free.** An alias cannot be an address that a user or group in the " +
			"organization still uses, as its primary address or as an alias " +
			"([Add or delete an alternate email address](https://support.google.com/a/answer/33327), " +
			"[Resolve duplicate account conflicts](https://knowledge.workspace.google.com/p/duplicate-account-errors)). To move an " +
			"address from one user or group to this user, remove it there in an earlier apply than the one that " +
			"adds this alias: Terraform does not order deleting one resource before creating an unrelated one.\n\n" +
			"Google allows up to 30 aliases per user and notes that alias changes can take up to 24 hours to reach " +
			"mail delivery ([Add or delete an alternate email address](https://support.google.com/a/answer/33327)). " +
			"The addresses Google adds on its own, such as those in a domain alias, are not editable and are not " +
			"managed by this resource.\n\n" +
			"~> **Requirements:** the service account's domain-wide delegation must include the " +
			"`https://www.googleapis.com/auth/admin.directory.user` scope, which the provider requests by default.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				MarkdownDescription: "Composite ID, `<user_id>/<alias>`. Used for import.",
			},
			"user_id": schema.StringAttribute{
				Required:            true,
				PlanModifiers:       requiresReplace,
				MarkdownDescription: "Immutable ID of the user, e.g. `googleworkspace_user.example.id`. Changing this forces a new alias.",
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

func (r *userAliasResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *userAliasResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan userAliasModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	userID, alias := plan.UserID.ValueString(), plan.Alias.ValueString()
	// Retry: an alias can be added before a just-created user is fully visible.
	err := withRetry(ctx, func() error {
		_, e := r.client.Directory.Users.Aliases.Insert(userID, &admin.Alias{Alias: alias}).Context(ctx).Do()
		return e
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to add user alias", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, userAliasToModel(userID, alias))...)
}

func (r *userAliasResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state userAliasModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	userID := state.UserID.ValueString()
	list, err := r.client.Directory.Users.Aliases.List(userID).Context(ctx).Do()
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read user aliases", err.Error())
		return
	}
	aliases, err := decodeAliases(list)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read user aliases", err.Error())
		return
	}
	if !hasAlias(aliases, state.Alias.ValueString()) {
		resp.State.RemoveResource(ctx)
		return
	}

	// Keep the configured spelling: Workspace may report the address in lower case.
	resp.Diagnostics.Append(resp.State.Set(ctx, userAliasToModel(userID, state.Alias.ValueString()))...)
}

// Update is never called: every configurable attribute forces replacement.
func (r *userAliasResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Unexpected update", "googleworkspace_user_alias has no updatable attributes")
}

func (r *userAliasResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state userAliasModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.Directory.Users.Aliases.Delete(state.UserID.ValueString(), state.Alias.ValueString()).Context(ctx).Do(); err != nil {
		if isNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Unable to remove user alias", err.Error())
	}
}

// ImportState accepts `<user_id>/<alias>` or `<user_email>/<alias>`. A user email
// is resolved to the user's immutable ID so the import matches configuration that
// uses `googleworkspace_user.<name>.id`.
func (r *userAliasResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	user, alias, ok := splitAliasID(req.ID)
	if !ok {
		resp.Diagnostics.AddError("Invalid user alias ID",
			fmt.Sprintf("expected `<user_id>/<alias>` or `<user_email>/<alias>`, got %q", req.ID))
		return
	}

	userID := user
	if strings.Contains(user, "@") {
		u, err := r.client.Directory.Users.Get(user).Fields("id").Context(ctx).Do()
		if err != nil {
			resp.Diagnostics.AddError("Unable to look up user", fmt.Sprintf("resolving %q to a user ID: %s", user, err))
			return
		}
		userID = u.Id
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, userAliasToModel(userID, alias))...)
}

func userAliasToModel(userID, alias string) userAliasModel {
	return userAliasModel{
		ID:     types.StringValue(userID + "/" + alias),
		UserID: types.StringValue(userID),
		Alias:  types.StringValue(alias),
	}
}
