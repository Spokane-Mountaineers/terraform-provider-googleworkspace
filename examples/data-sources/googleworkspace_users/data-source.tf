data "googleworkspace_users" "all" {}

# Find accounts that exist in Workspace but are not managed in code.
locals {
  managed_ids = [for u in googleworkspace_user.this : u.id]
  unmanaged   = [for u in data.googleworkspace_users.all.users : u.primary_email if !contains(local.managed_ids, u.id)]
}

data "googleworkspace_users" "board" {
  query = "orgUnitPath='/Board' isSuspended=false"
}
