data "googleworkspace_org_units" "all" {}

# Find organizational units created outside of code.
locals {
  managed_ou_ids = [for ou in googleworkspace_org_unit.this : ou.id]
  unmanaged_ous  = [for ou in data.googleworkspace_org_units.all.org_units : ou.org_unit_path if !contains(local.managed_ou_ids, ou.id)]
}
