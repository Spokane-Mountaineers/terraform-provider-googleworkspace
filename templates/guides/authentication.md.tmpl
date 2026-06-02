---
page_title: "Authentication: Domain-Wide Delegation"
subcategory: ""
description: |-
  Set up keyless domain-wide delegation, the only supported authentication method
  for the Google Workspace provider.
---

# Authentication: Domain-Wide Delegation

Domain-wide delegation (DWD) is the **only supported** authentication method for this
provider. A service account impersonates a Workspace admin user; because the flow is
server-to-server, it never hits the "This app is blocked" consent screen that
Workspace API controls put in front of the interactive `gcloud` user-token flow.

The token is minted **keyless** — no service-account key is downloaded. The operator's
(or CI's) Application Default Credentials sign the delegation assertion via IAM, so
they must hold `roles/iam.serviceAccountTokenCreator` on the service account.

## One-time setup

### 1. Enable the APIs

```shell
gcloud services enable admin.googleapis.com groupssettings.googleapis.com \
  --project=YOUR_PROJECT
```

### 2. Create the service account

```shell
gcloud iam service-accounts create tofu-workspace \
  --project=YOUR_PROJECT \
  --display-name="OpenTofu Google Workspace"
```

### 3. Find the service account's numeric client ID

This is what the Admin console authorizes — not the email.

```shell
gcloud iam service-accounts describe \
  tofu-workspace@YOUR_PROJECT.iam.gserviceaccount.com \
  --format='value(uniqueId)'
```

### 4. Authorize the client in the Admin console

As a super admin: **Admin console → Security → Access and data control → API controls
→ Domain-wide delegation → Add new**. Enter the numeric client ID from step 3 and the
OAuth scopes the provider requests:

```text
https://www.googleapis.com/auth/admin.directory.user.readonly,
https://www.googleapis.com/auth/admin.directory.group.readonly,
https://www.googleapis.com/auth/admin.directory.group.member.readonly,
https://www.googleapis.com/auth/admin.directory.orgunit.readonly,
https://www.googleapis.com/auth/admin.directory.rolemanagement.readonly,
https://www.googleapis.com/auth/admin.directory.domain.readonly,
https://www.googleapis.com/auth/admin.directory.userschema.readonly,
https://www.googleapis.com/auth/apps.groups.settings
```

Drop the `.readonly` suffixes when you start managing (writing) those resources.

### 5. Grant the caller permission to mint tokens (keyless)

```shell
gcloud iam service-accounts add-iam-policy-binding \
  tofu-workspace@YOUR_PROJECT.iam.gserviceaccount.com \
  --member="user:you@example.com" \
  --role="roles/iam.serviceAccountTokenCreator"
```

In CI, grant the same role to the workload identity instead of a user.

## Provider configuration

```terraform
provider "googleworkspace" {
  service_account         = "tofu-workspace@YOUR_PROJECT.iam.gserviceaccount.com"
  impersonated_user_email = "admin@example.com"
}
```

Equivalent environment variables: `GOOGLEWORKSPACE_SERVICE_ACCOUNT` and
`GOOGLEWORKSPACE_IMPERSONATED_USER_EMAIL`. For CI, `GOOGLEWORKSPACE_ACCESS_TOKEN`
accepts a pre-minted DWD access token instead.

The impersonated user must be a real Workspace admin with the privileges for the
resources you manage.

## Troubleshooting

- **`unauthorized_client`** when minting a token — the service account's client ID is
  not authorized in the Admin console for the requested scopes, or the change has not
  propagated yet (usually minutes, up to 24h).
- **`403` on a read or write** — the impersonated user lacks the admin privilege for
  that resource. Use a user with the right admin role.
- **`PermissionDenied` minting the token** — the caller is missing
  `roles/iam.serviceAccountTokenCreator` on the service account.
