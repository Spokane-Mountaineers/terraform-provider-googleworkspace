resource "googleworkspace_group" "engineers" {
  email = "engineers@example.com"
  name  = "Engineers"
}

resource "googleworkspace_group_member" "alice" {
  group_id = googleworkspace_group.engineers.id
  email    = "alice@example.com"
  role     = "MEMBER"
}
