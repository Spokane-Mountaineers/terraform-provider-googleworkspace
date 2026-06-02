# terraform-provider-google-workspace

An OpenTofu/Terraform provider for managing Google Workspace directory resources,
built on Google's supported APIs — the [Admin SDK Directory API][admin-sdk] and the
[Cloud Identity API][cloud-identity].

It exists because there is no actively-maintained, comprehensive provider for
Workspace: HashiCorp's `googleworkspace` was a tech preview, last released v0.7.0
(June 2022), and was archived in 2025. This is a thin, purpose-built provider on
[`terraform-plugin-framework`][framework], scoped to what the Spokane Mountaineers
org actually manages.

> **Status:** early. Today it ships the provider skeleton and a `googleworkspace_domains`
> data source that proves auth and connectivity end-to-end. Resources (users, groups,
> org units, roles) land iteratively.

## Naming

The repository is `terraform-provider-google-workspace`, but Terraform resource type
prefixes cannot contain hyphens, so resources are named **`googleworkspace_*`** (for
example `googleworkspace_user`). The provider source address is
`registry.spokanemountaineers.org/spokane-mountaineers/google-workspace`.

## Authentication

The first supported credential model is a **keyless admin user token**. An operator
who is a Workspace super admin authenticates with Application Default Credentials,
scoped to the Admin SDK:

```bash
gcloud auth application-default login --scopes=openid,\
https://www.googleapis.com/auth/admin.directory.user,\
https://www.googleapis.com/auth/admin.directory.group,\
https://www.googleapis.com/auth/admin.directory.group.member,\
https://www.googleapis.com/auth/admin.directory.orgunit,\
https://www.googleapis.com/auth/admin.directory.rolemanagement,\
https://www.googleapis.com/auth/admin.directory.domain.readonly,\
https://www.googleapis.com/auth/admin.directory.userschema
```

Credential precedence:

1. provider `access_token` attribute
2. `GOOGLEWORKSPACE_ACCESS_TOKEN`
3. Application Default Credentials

`customer_id` defaults to `my_customer` and may be set via the provider attribute or
`GOOGLEWORKSPACE_CUSTOMER_ID`.

```hcl
provider "googleworkspace" {
  # customer_id = "my_customer"  # optional
}

data "googleworkspace_domains" "this" {}

output "domains" {
  value = data.googleworkspace_domains.this.domains
}
```

## Development

Requires Go 1.26.3 and [`just`](https://github.com/casey/just).

```bash
just            # list recipes
just fmt        # go fmt ./... + gofmt -s
just fix        # go fix ./...
just lint       # golangci-lint
just test       # unit tests
just ci         # fmt-check + vet + lint + test
just build      # build the provider binary
just install    # build into ~/.terraform.d/plugins filesystem mirror
just docs       # regenerate registry docs (docs/) from schema + templates + examples
```

### Registry documentation

The `docs/` tree is what the OpenTofu/Terraform registry renders. It is **generated**
by [`tfplugindocs`][tfplugindocs] (pinned as a Go tool dependency) from the provider
schema, the `templates/` (rich Overview in `templates/index.md.tmpl`), and the
`examples/` `.tf` files. Edit those sources — not `docs/` — and run `just docs`. CI
fails if the committed `docs/` are stale.

[tfplugindocs]: https://github.com/hashicorp/terraform-plugin-docs

To use a local build, point your CLI config at the filesystem mirror after
`just install`, e.g. in `~/.terraformrc`:

```hcl
provider_installation {
  filesystem_mirror {
    path    = "/Users/<you>/.terraform.d/plugins"
    include = ["registry.spokanemountaineers.org/*/*"]
  }
  direct {
    exclude = ["registry.spokanemountaineers.org/*/*"]
  }
}
```

## License

Apache License 2.0 — see [LICENSE](./LICENSE).

[admin-sdk]: https://developers.google.com/admin-sdk/directory
[cloud-identity]: https://cloud.google.com/identity/docs/reference/rest
[framework]: https://github.com/hashicorp/terraform-plugin-framework
