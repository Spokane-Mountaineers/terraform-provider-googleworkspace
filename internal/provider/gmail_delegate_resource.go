package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/api/gmail/v1"

	"github.com/Spokane-Mountaineers/terraform-provider-googleworkspace/internal/client"
)

var (
	_ resource.Resource                = (*gmailDelegateResource)(nil)
	_ resource.ResourceWithConfigure   = (*gmailDelegateResource)(nil)
	_ resource.ResourceWithImportState = (*gmailDelegateResource)(nil)
)

type gmailDelegateResource struct {
	client *client.Client
}

// NewGmailDelegateResource is the resource factory.
func NewGmailDelegateResource() resource.Resource {
	return &gmailDelegateResource{}
}

// How often and how long Create polls for a new owner's mailbox.
const (
	mailboxPollInterval = 5 * time.Second
	mailboxWait         = 5 * time.Minute
)

type gmailDelegateModel struct {
	ID                 types.String `tfsdk:"id"`
	UserEmail          types.String `tfsdk:"user_email"`
	DelegateEmail      types.String `tfsdk:"delegate_email"`
	VerificationStatus types.String `tfsdk:"verification_status"`
}

func (r *gmailDelegateResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_gmail_delegate"
}

func (r *gmailDelegateResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	requiresReplace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		MarkdownDescription: "Gives one user access to another user's Gmail mailbox (mail delegation). The delegate " +
			"can read, send, and delete mail in the mailbox from their own account, without its password.\n\n" +
			"This suits role mailboxes such as `treasurer@`: the mailbox keeps its history and filters while the " +
			"people holding the role change. The delegate can be a group, so handing a role over becomes a membership " +
			"change: add the new holder to the group, then remove the old one.\n\n" +
			"Delegates are added as `accepted` immediately, with no invitation email, though Google notes they can " +
			"take up to a minute to become usable. If the mailbox owner was just created, creating the delegate " +
			"waits up to 5 minutes for the owner's mailbox to be set up first. Both users must be in this Workspace account, and Google " +
			"generally allows up to 25 delegates per mailbox.\n\n" +
			"~> **Requirements:** the provider must use `service_account` (the call impersonates the mailbox owner, " +
			"so a pre-minted `access_token` cannot be used), the service account's domain-wide delegation must " +
			"include the `https://www.googleapis.com/auth/gmail.settings.basic` (read) and " +
			"`https://www.googleapis.com/auth/gmail.settings.sharing` (create/delete) scopes, the Gmail API must be " +
			"enabled in its project, and **Mail delegation** must be enabled in the Admin console for the " +
			"mailbox's organizational unit.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				MarkdownDescription: "Composite ID, `<user_email>/<delegate_email>`. Used for import.",
			},
			"user_email": schema.StringAttribute{
				Required:            true,
				PlanModifiers:       requiresReplace,
				MarkdownDescription: "Primary email of the mailbox being shared, e.g. `treasurer@example.org`.",
			},
			"delegate_email": schema.StringAttribute{
				Required:      true,
				PlanModifiers: requiresReplace,
				MarkdownDescription: "Primary email (not an alias) of the user or group that gets access. A group grants " +
					"access to all of its members who are users in this Workspace account (not external members), and " +
					"requires group delegation to be allowed in the Admin console. Google's API reference only mentions " +
					"users, but group delegates were verified to work.",
			},
			"verification_status": schema.StringAttribute{
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				MarkdownDescription: "Delegation status reported by Gmail, normally `accepted`.",
			},
		},
	}
}

func (r *gmailDelegateResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *gmailDelegateResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan gmailDelegateModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	user := plan.UserEmail.ValueString()
	svc, err := r.client.GmailFor(user)
	if err != nil {
		resp.Diagnostics.AddError("Unable to configure Gmail client", err.Error())
		return
	}

	// A mailbox owner created in the same apply has no mailbox for a few seconds
	// to minutes, and Gmail rejects delegates until it exists.
	err = waitForMailbox(ctx, mailboxPollInterval, mailboxWait, func(ctx context.Context) (bool, error) {
		u, e := r.client.Directory.Users.Get(user).Fields("isMailboxSetup").Context(ctx).Do()
		if isTransient(e) {
			return false, nil
		}
		if e != nil {
			return false, e
		}
		return u.IsMailboxSetup, nil
	})
	if err != nil {
		resp.Diagnostics.AddError("Mailbox not ready for a Gmail delegate", fmt.Sprintf("%s: %s", user, err))
		return
	}

	var d *gmail.Delegate
	// Retry: a delegate created in the same apply may not be visible to Gmail yet.
	err = withRetry(ctx, func() error {
		var e error
		d, e = svc.Users.Settings.Delegates.Create("me", &gmail.Delegate{
			DelegateEmail: plan.DelegateEmail.ValueString(),
		}).Context(ctx).Do()
		return e
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to add Gmail delegate", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, delegateToModel(user, d))...)
}

func (r *gmailDelegateResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state gmailDelegateModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	user := state.UserEmail.ValueString()
	svc, err := r.client.GmailFor(user)
	if err != nil {
		resp.Diagnostics.AddError("Unable to configure Gmail client", err.Error())
		return
	}

	d, err := svc.Users.Settings.Delegates.Get("me", state.DelegateEmail.ValueString()).Context(ctx).Do()
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Gmail delegate", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, delegateToModel(user, d))...)
}

// Update is never called: every configurable attribute forces replacement.
func (r *gmailDelegateResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Unexpected update", "googleworkspace_gmail_delegate has no updatable attributes")
}

func (r *gmailDelegateResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state gmailDelegateModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	svc, err := r.client.GmailFor(state.UserEmail.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to configure Gmail client", err.Error())
		return
	}

	if err := svc.Users.Settings.Delegates.Delete("me", state.DelegateEmail.ValueString()).Context(ctx).Do(); err != nil {
		if isNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Unable to remove Gmail delegate", err.Error())
	}
}

// ImportState accepts `<user_email>/<delegate_email>`.
func (r *gmailDelegateResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	user, delegate, ok := splitDelegateID(req.ID)
	if !ok {
		resp.Diagnostics.AddError("Invalid Gmail delegate ID", fmt.Sprintf("expected `<user_email>/<delegate_email>`, got %q", req.ID))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("user_email"), user)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("delegate_email"), delegate)...)
}

func delegateToModel(user string, d *gmail.Delegate) gmailDelegateModel {
	return gmailDelegateModel{
		ID:                 types.StringValue(user + "/" + d.DelegateEmail),
		UserEmail:          types.StringValue(user),
		DelegateEmail:      types.StringValue(d.DelegateEmail),
		VerificationStatus: types.StringValue(d.VerificationStatus),
	}
}

func splitDelegateID(id string) (user, delegate string, ok bool) {
	user, delegate, found := strings.Cut(id, "/")
	if !found || !strings.Contains(user, "@") || !strings.Contains(delegate, "@") {
		return "", "", false
	}
	return user, delegate, true
}

// waitForMailbox polls ready every interval until it reports true, for at most
// wait. Gmail answers a delegate request for a mailbox that doesn't exist yet with
// 400 failedPrecondition, the same error it gives when delegation is off for the
// owner's organizational unit, so the error itself can't be retried safely.
func waitForMailbox(ctx context.Context, interval, wait time.Duration, ready func(context.Context) (bool, error)) error {
	deadline := time.Now().Add(wait)
	for {
		ok, err := ready(ctx)
		if err != nil || ok {
			return err
		}
		if !time.Now().Before(deadline) {
			return fmt.Errorf("mailbox still not set up after %s", wait)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}
