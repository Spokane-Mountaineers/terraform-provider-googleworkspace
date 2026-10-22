data "googleworkspace_policies" "all" {}
data "googleworkspace_org_units" "all" {}

# Settings applied directly to each organizational unit (not inherited). An empty
# list means moving users into or out of that unit changes no exposed setting.
locals {
  local_settings = {
    for ou in data.googleworkspace_org_units.all.org_units : ou.org_unit_path => [
      for p in data.googleworkspace_policies.all.policies : p.setting_type
      if p.org_unit_id == ou.id && p.type == "ADMIN"
    ]
  }
}

data "googleworkspace_policies" "gmail" {
  filter = "setting.type.matches('gmail.*')"
}
