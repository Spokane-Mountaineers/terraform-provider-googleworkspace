data "googleworkspace_role_assignments" "all" {}
data "googleworkspace_users" "all" {}

# Super admins by email.
locals {
  emails_by_id = { for u in data.googleworkspace_users.all.users : u.id => u.primary_email }
  super_admins = [
    for a in data.googleworkspace_role_assignments.all.role_assignments :
    lookup(local.emails_by_id, a.assigned_to, a.assigned_to) if a.is_super_admin
  ]
}
