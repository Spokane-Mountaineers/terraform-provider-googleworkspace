package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	admin "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/googleapi"

	"github.com/Spokane-Mountaineers/terraform-provider-googleworkspace/internal/client"
)

var (
	_ resource.Resource                = (*orgUnitResource)(nil)
	_ resource.ResourceWithConfigure   = (*orgUnitResource)(nil)
	_ resource.ResourceWithImportState = (*orgUnitResource)(nil)
)

type orgUnitResource struct {
	client *client.Client
}

// NewOrgUnitResource is the resource factory.
func NewOrgUnitResource() resource.Resource {
	return &orgUnitResource{}
}

type orgUnitModel struct {
	ID                types.String `tfsdk:"id"`
	Name              types.String `tfsdk:"name"`
	ParentOrgUnitPath types.String `tfsdk:"parent_org_unit_path"`
	Description       types.String `tfsdk:"description"`
	OrgUnitPath       types.String `tfsdk:"org_unit_path"`
}

func (r *orgUnitResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_org_unit"
}

func (r *orgUnitResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "A Google Workspace organizational unit.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				MarkdownDescription: "Immutable organizational unit ID (the `id:...` value). Used for import.",
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Name of the organizational unit (the final path component).",
			},
			"parent_org_unit_path": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Full path of the parent organizational unit, e.g. `/` for the root or `/Engineering`.",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Description of the organizational unit.",
			},
			"org_unit_path": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Full path of this organizational unit.",
			},
		},
	}
}

func (r *orgUnitResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *orgUnitResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan orgUnitModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ou, err := r.client.Directory.Orgunits.Insert(r.client.CustomerID, &admin.OrgUnit{
		Name:              plan.Name.ValueString(),
		ParentOrgUnitPath: plan.ParentOrgUnitPath.ValueString(),
		Description:       plan.Description.ValueString(),
	}).Context(ctx).Do()
	if err != nil {
		resp.Diagnostics.AddError("Unable to create organizational unit", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, orgUnitToModel(ou))...)
}

func (r *orgUnitResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state orgUnitModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ou, err := r.client.Directory.Orgunits.Get(r.client.CustomerID, state.ID.ValueString()).Context(ctx).Do()
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read organizational unit", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, orgUnitToModel(ou))...)
}

func (r *orgUnitResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state orgUnitModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ou, err := r.client.Directory.Orgunits.Update(r.client.CustomerID, state.ID.ValueString(), &admin.OrgUnit{
		Name:              plan.Name.ValueString(),
		ParentOrgUnitPath: plan.ParentOrgUnitPath.ValueString(),
		Description:       plan.Description.ValueString(),
	}).Context(ctx).Do()
	if err != nil {
		resp.Diagnostics.AddError("Unable to update organizational unit", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, orgUnitToModel(ou))...)
}

func (r *orgUnitResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state orgUnitModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.Directory.Orgunits.Delete(r.client.CustomerID, state.ID.ValueString()).Context(ctx).Do(); err != nil {
		if isNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Unable to delete organizational unit", err.Error())
	}
}

func (r *orgUnitResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func orgUnitToModel(ou *admin.OrgUnit) orgUnitModel {
	return orgUnitModel{
		ID:                types.StringValue(ou.OrgUnitId),
		Name:              types.StringValue(ou.Name),
		ParentOrgUnitPath: types.StringValue(ou.ParentOrgUnitPath),
		Description:       types.StringValue(ou.Description),
		OrgUnitPath:       types.StringValue(ou.OrgUnitPath),
	}
}

// isNotFound reports whether err is a Google API 404.
func isNotFound(err error) bool {
	var ae *googleapi.Error
	return errors.As(err, &ae) && ae.Code == http.StatusNotFound
}
