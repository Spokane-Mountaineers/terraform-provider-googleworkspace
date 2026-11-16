# Imported as "<user_id>/<alias>". The user's email address also works in
# place of its ID and is resolved to the ID on import.
terraform import googleworkspace_user_alias.jane 123456789012345678901/jane@example.org
terraform import googleworkspace_user_alias.old_address jdoe@example.org/jane.doe@example.org
