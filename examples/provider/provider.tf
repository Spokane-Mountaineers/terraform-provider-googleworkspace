# Authenticate with a keyless admin user token via Application Default Credentials.
# Run a scoped `gcloud auth application-default login` first (see Authentication below).
provider "googleworkspace" {
  # customer_id = "my_customer" # optional; defaults to my_customer
}

# List the domains registered on the account.
data "googleworkspace_domains" "this" {}

output "domains" {
  value = data.googleworkspace_domains.this.domains
}
