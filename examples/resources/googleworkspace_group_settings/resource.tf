# An access-only group: its membership grants access to the treasurer@ mailbox
# through Gmail delegation, so nobody should mail it or find it. The group has no
# owners, and ALL_OWNERS_CAN_POST lets only owners post. NONE_CAN_POST would need
# archive_only, which marks the group inactive.
resource "googleworkspace_group" "treasurer_delegates" {
  email       = "treasurer-delegates@example.org"
  name        = "Treasurer delegates"
  description = "Access list for treasurer@. Not a mailing list."
}

resource "googleworkspace_group_settings" "treasurer_delegates" {
  email = googleworkspace_group.treasurer_delegates.email

  who_can_post_message           = "ALL_OWNERS_CAN_POST"
  who_can_join                   = "INVITED_CAN_JOIN"
  who_can_discover_group         = "ALL_MEMBERS_CAN_DISCOVER"
  allow_external_members         = false
  include_in_global_address_list = false
}

# A mailing list that forwards to people outside the organization, replacing a
# Gmail routing rule.
resource "googleworkspace_group_settings" "hiking" {
  email = "hiking@example.org"

  who_can_post_message     = "ANYONE_CAN_POST"
  message_moderation_level = "MODERATE_NON_MEMBERS"
  allow_external_members   = true
}

# A shared support inbox: members take, assign, and resolve conversations.
# Collaborative inbox needs conversation history, so is_archived is set too.
resource "googleworkspace_group_settings" "help" {
  email = "help@example.org"

  enable_collaborative_inbox = true
  is_archived                = true
  who_can_post_message       = "ANYONE_CAN_POST"
  who_can_view_group         = "ALL_MEMBERS_CAN_VIEW"
}
