# The treasurer role mailbox keeps its history and filters; the people holding the
# role open it from their own accounts. Delegating to a group makes handing the
# role over a membership change: add the incoming treasurer, then remove the
# outgoing one once the overlap is done.
resource "googleworkspace_group" "treasurer_delegates" {
  email       = "treasurer-delegates@example.org"
  name        = "Treasurer delegates"
  description = "Access list for treasurer@. Not a mailing list."
}

resource "googleworkspace_group_member" "treasurer" {
  group_id          = googleworkspace_group.treasurer_delegates.id
  email             = "jdoe@example.org"
  delivery_settings = "NONE"
}

resource "googleworkspace_gmail_delegate" "treasurer" {
  user_email     = "treasurer@example.org"
  delegate_email = googleworkspace_group.treasurer_delegates.email
}
