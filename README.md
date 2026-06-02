# terraform-provider-googleworkspace

An OpenTofu/Terraform provider for managing Google Workspace directory resources,
built on Google's supported APIs — the [Admin SDK Directory API][admin-sdk] and the
[Cloud Identity API][cloud-identity].

It exists because there is no actively-maintained, comprehensive provider for
Workspace: HashiCorp's `googleworkspace` was a tech preview, last released v0.7.0
(June 2022), and was archived in 2025. This is a thin, purpose-built provider on
[`terraform-plugin-framework`][framework], scoped to what the Spokane Mountaineers
org actually manages. Resources are named `googleworkspace_*` (for example
`googleworkspace_user`). The provider source address is
`registry.opentofu.org/spokane-mountaineers/googleworkspace`.

> **Status:** early. Today it ships the provider skeleton and a `googleworkspace_domains`
> data source that proves auth and connectivity end-to-end. Resources (users, groups,
> org units, roles) land iteratively.

## Authentication

This provider authenticates with **domain-wide delegation (DWD) only**. A service
account with DWD impersonates a Workspace admin user; the token is minted **keyless**
from your Application Default Credentials, which must hold
`roles/iam.serviceAccountTokenCreator` on the service account.

```hcl
provider "googleworkspace" {
  service_account         = "tofu-workspace@my-project.iam.gserviceaccount.com"
  impersonated_user_email = "admin@spokanemountaineers.org"
}
```

Equivalent env vars: `GOOGLEWORKSPACE_SERVICE_ACCOUNT`,
`GOOGLEWORKSPACE_IMPERSONATED_USER_EMAIL`, and `GOOGLEWORKSPACE_ACCESS_TOKEN` (a
pre-minted DWD token, for CI). `customer_id` defaults to `my_customer`.

The interactive user-token flow (`gcloud auth application-default login`) is **not
supported** — Workspace API controls block the shared gcloud OAuth client from
sensitive Admin SDK scopes. Full setup is in
[docs/guides/authentication.md](./docs/guides/authentication.md).

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
schema, the `templates/` (rich Overview in `templates/index.md.tmpl` and guides under
`templates/guides/`), and the `examples/` `.tf` files. Edit those sources — not
`docs/` — and run `just docs`. CI fails if the committed `docs/` are stale.

To use a local build, point your CLI config at the filesystem mirror after
`just install`, e.g. in `~/.terraformrc`:

```hcl
provider_installation {
  filesystem_mirror {
    path    = "/Users/<you>/.terraform.d/plugins"
    include = ["registry.opentofu.org/spokane-mountaineers/*"]
  }
  direct {
    exclude = ["registry.opentofu.org/spokane-mountaineers/*"]
  }
}
```

## License

Apache License 2.0 — see [LICENSE](./LICENSE).

[admin-sdk]: https://developers.google.com/admin-sdk/directory
[cloud-identity]: https://cloud.google.com/identity/docs/reference/rest
[framework]: https://github.com/hashicorp/terraform-plugin-framework
[tfplugindocs]: https://github.com/hashicorp/terraform-plugin-docs
