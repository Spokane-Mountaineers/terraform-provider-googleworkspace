resource "googleworkspace_org_unit" "engineering" {
  name                 = "Engineering"
  parent_org_unit_path = "/"
  description          = "Engineering organizational unit"
}

# A nested org unit.
resource "googleworkspace_org_unit" "frontend" {
  name                 = "Frontend"
  parent_org_unit_path = googleworkspace_org_unit.engineering.org_unit_path
}
