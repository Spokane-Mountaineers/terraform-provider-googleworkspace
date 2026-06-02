data "googleworkspace_domains" "this" {}

# The primary domain on the account.
output "primary_domain" {
  value = one([
    for d in data.googleworkspace_domains.this.domains : d.domain_name if d.is_primary
  ])
}
