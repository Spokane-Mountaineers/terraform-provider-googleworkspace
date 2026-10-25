# Imported as "<group_id>/<alias>". The group's email address also works in
# place of its ID and is resolved to the ID on import.
terraform import googleworkspace_group_alias.trips 01234567890abcde/trips@example.org
terraform import googleworkspace_group_alias.outings hiking@example.org/outings@example.org
