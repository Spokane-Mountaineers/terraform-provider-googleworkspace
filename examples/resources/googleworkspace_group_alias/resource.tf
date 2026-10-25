resource "googleworkspace_group" "hiking" {
  email = "hiking@example.org"
  name  = "Hiking"
}

# Mail to trips@ and outings@ reaches the hiking@ list. Neither address may
# still be used by a user or group, as its primary address or as an alias.
resource "googleworkspace_group_alias" "trips" {
  group_id = googleworkspace_group.hiking.id
  alias    = "trips@example.org"
}

resource "googleworkspace_group_alias" "outings" {
  group_id = googleworkspace_group.hiking.id
  alias    = "outings@example.org"
}

# To fold a retired group's address into this list, remove the old group in
# one apply and add the alias in a later one. In a single apply Terraform may
# add the alias before deleting the group, and Workspace rejects the alias
# while the address is still in use.
