# The initial password is generated ephemerally and sent write-only, so it never
# lands in state or plan files. The user sets their own at first sign-in.
ephemeral "random_password" "jdoe" {
  length = 24
}

resource "googleworkspace_user" "jdoe" {
  primary_email = "jdoe@example.com"
  given_name    = "Jane"
  family_name   = "Doe"
  org_unit_path = "/Board"

  recovery_email = "jane@example.net"
  recovery_phone = "+15095550100"

  password_wo                   = ephemeral.random_password.jdoe.result
  password_wo_version           = "1"
  change_password_at_next_login = true

  # Destroying a user deletes the account. Offboard with `suspended = true`.
  lifecycle {
    prevent_destroy = true
  }
}
