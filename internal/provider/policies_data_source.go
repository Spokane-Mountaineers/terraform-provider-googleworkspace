package provider

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/api/cloudidentity/v1"

	"github.com/Spokane-Mountaineers/terraform-provider-googleworkspace/internal/client"
)

var (
	_ datasource.DataSource              = (*policiesDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*policiesDataSource)(nil)
)

type policiesDataSource struct {
	client *client.Client
}

// NewPoliciesDataSource is the data-source factory.
func NewPoliciesDataSource() datasource.DataSource {
	return &policiesDataSource{}
}

type policiesDataModel struct {
	Filter   types.String            `tfsdk:"filter"`
	Policies []policiesDataItemModel `tfsdk:"policies"`
}

type policiesDataItemModel struct {
	Name        types.String  `tfsdk:"name"`
	Type        types.String  `tfsdk:"type"`
	SettingType types.String  `tfsdk:"setting_type"`
	Value       types.String  `tfsdk:"value"`
	OrgUnitID   types.String  `tfsdk:"org_unit_id"`
	Group       types.String  `tfsdk:"group"`
	Query       types.String  `tfsdk:"query"`
	SortOrder   types.Float64 `tfsdk:"sort_order"`
}

func (d *policiesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_policies"
}

func (d *policiesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	str := func(desc string) schema.StringAttribute {
		return schema.StringAttribute{Computed: true, MarkdownDescription: desc}
	}

	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists the Admin console settings (policies) that apply in the account, through the " +
			"[Cloud Identity Policy API](https://cloud.google.com/identity/docs/concepts/overview-policies). " +
			"Each policy is one setting applied to an organizational unit or group, so this shows what moving a " +
			"user between units would change.\n\n" +
			"The Policy API is read-only for nearly all settings (Google marks only Data Protection rules and " +
			"detectors as writable), so this is for inventory and drift detection rather than management. Not " +
			"every Admin console setting is exposed; Gmail routing rules, for example, are not.\n\n" +
			"~> **Requirements:** the provider must use `service_account`, its domain-wide delegation must include " +
			"`https://www.googleapis.com/auth/cloud-identity.policies`, and the Cloud Identity API must be enabled " +
			"in its project. The token for this scope is separate, so other resources do not need it.",
		Attributes: map[string]schema.Attribute{
			"filter": schema.StringAttribute{
				Optional: true,
				MarkdownDescription: "Policy API [filter](https://cloud.google.com/identity/docs/reference/rest/v1/policies/list), " +
					"e.g. `setting.type.matches('gmail.*')`. Omit to list every policy.",
			},
			"policies": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Matching policies, ordered by setting type, then organizational unit.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name":         str("Resource name, `policies/...`."),
						"type":         str("`ADMIN` for settings an admin configured, `SYSTEM` for Google defaults."),
						"setting_type": str("Setting identifier, e.g. `settings/security.two_step_verification_enforcement`."),
						"value":        str("The setting's value as a JSON object."),
						"org_unit_id": str("Organizational unit the policy is applied to, as `id:...` (matching " +
							"`googleworkspace_org_unit.id`), or null when it applies through a group."),
						"group": str("Group the policy is applied to (`groups/...`), or null."),
						"query": str("The policy's full CEL targeting expression."),
						"sort_order": schema.Float64Attribute{
							Computed:            true,
							MarkdownDescription: "Precedence relative to other policies with the same setting type.",
						},
					},
				},
			},
		},
	}
}

func (d *policiesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *policiesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data policiesDataModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	svc, err := d.client.Policies()
	if err != nil {
		resp.Diagnostics.AddError("Unable to configure Cloud Identity client", err.Error())
		return
	}

	call := svc.Policies.List().PageSize(100)
	if f := data.Filter.ValueString(); f != "" {
		call = call.Filter(f)
	}

	data.Policies = []policiesDataItemModel{}
	err = call.Pages(ctx, func(page *cloudidentity.ListPoliciesResponse) error {
		for _, p := range page.Policies {
			data.Policies = append(data.Policies, policyToModel(p))
		}
		return nil
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to list policies", err.Error())
		return
	}
	sort.SliceStable(data.Policies, func(i, j int) bool {
		a, b := data.Policies[i], data.Policies[j]
		if a.SettingType.ValueString() != b.SettingType.ValueString() {
			return a.SettingType.ValueString() < b.SettingType.ValueString()
		}
		return a.OrgUnitID.ValueString() < b.OrgUnitID.ValueString()
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, data)...)
}

func policyToModel(p *cloudidentity.Policy) policiesDataItemModel {
	m := policiesDataItemModel{
		Name:      types.StringValue(p.Name),
		Type:      types.StringValue(p.Type),
		Value:     types.StringValue("{}"),
		OrgUnitID: types.StringNull(),
		Group:     types.StringNull(),
		Query:     types.StringNull(),
		SortOrder: types.Float64Value(0),
	}
	if p.Setting != nil {
		m.SettingType = types.StringValue(p.Setting.Type)
		if len(p.Setting.Value) > 0 {
			m.Value = types.StringValue(string(p.Setting.Value))
		}
	} else {
		m.SettingType = types.StringValue("")
	}
	if q := p.PolicyQuery; q != nil {
		m.OrgUnitID = stringOrNull(normalizeOrgUnitID(q.OrgUnit))
		m.Group = stringOrNull(q.Group)
		m.Query = stringOrNull(q.Query)
		m.SortOrder = types.Float64Value(q.SortOrder)
	}
	return m
}

// normalizeOrgUnitID maps the API's `orgUnits/X` or bare `X` to `id:X`, the form
// the Directory API and googleworkspace_org_unit use.
func normalizeOrgUnitID(s string) string {
	s = strings.TrimPrefix(s, "orgUnits/")
	if s == "" || strings.HasPrefix(s, "id:") {
		return s
	}
	return "id:" + s
}
