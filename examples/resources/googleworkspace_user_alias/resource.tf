resource "googleworkspace_user" "jdoe" {
  primary_email = "jdoe@example.org"
  given_name    = "Jane"
  family_name   = "Doe"
}

# Mail to jane@ reaches jdoe@'s mailbox. The address may not still be used by
# a user or group, as its primary address or as an alias.
resource "googleworkspace_user_alias" "jane" {
  user_id = googleworkspace_user.jdoe.id
  alias   = "jane@example.org"
}

# Renaming a user keeps the old address as an alias that Workspace adds on
# its own. Import it to keep it in code, or to remove it later:
#   import {
#     to = googleworkspace_user_alias.old_address
#     id = "jdoe@example.org/jane.doe@example.org"
#   }
resource "googleworkspace_user_alias" "old_address" {
  user_id = googleworkspace_user.jdoe.id
  alias   = "jane.doe@example.org"
}

# Find aliases that exist in Workspace but are not managed in code.
data "googleworkspace_users" "all" {}

locals {
  managed_aliases = [
    googleworkspace_user_alias.jane.alias,
    googleworkspace_user_alias.old_address.alias,
  ]
  unmanaged_aliases = flatten([
    for u in data.googleworkspace_users.all.users :
    [for a in u.aliases : a if !contains(local.managed_aliases, a)]
  ])
}
