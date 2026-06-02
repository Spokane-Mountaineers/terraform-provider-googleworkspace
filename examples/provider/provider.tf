# Domain-wide delegation: the service account impersonates an admin user. The token is
# minted keyless from your Application Default Credentials, which must hold
# roles/iam.serviceAccountTokenCreator on the service account. See the Authentication guide.
provider "googleworkspace" {
  service_account         = "tofu-workspace@my-project.iam.gserviceaccount.com"
  impersonated_user_email = "admin@example.com"
  # customer_id = "my_customer" # optional; defaults to my_customer
}

# List the domains registered on the account.
data "googleworkspace_domains" "this" {}

output "domains" {
  value = data.googleworkspace_domains.this.domains
}
