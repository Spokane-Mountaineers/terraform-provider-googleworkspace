# The treasurer role mailbox keeps its history and filters; the people holding the
# role read and send from their own accounts. To hand the role over, add the new
# treasurer, then remove the outgoing one once the overlap is done.
resource "googleworkspace_gmail_delegate" "treasurer" {
  for_each = toset([
    "jdoe@example.org",
  ])

  user_email     = "treasurer@example.org"
  delegate_email = each.value
}
