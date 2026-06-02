// Package client wraps the Google Workspace APIs the provider builds on:
// the Admin SDK Directory API and (later) the Cloud Identity API.
package client

import (
	"context"
	"fmt"
	"os"

	"golang.org/x/oauth2"
	admin "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/impersonate"
	"google.golang.org/api/option"
)

// DefaultCustomerID is the Admin SDK alias for the caller's own account.
const DefaultCustomerID = "my_customer"

// DefaultScopes are the OAuth scopes requested when minting a domain-wide
// delegation token. They must be authorized for the service account under
// Admin console → Security → API controls → Domain-wide delegation.
var DefaultScopes = []string{
	admin.AdminDirectoryUserReadonlyScope,
	admin.AdminDirectoryGroupReadonlyScope,
	admin.AdminDirectoryGroupMemberReadonlyScope,
	admin.AdminDirectoryOrgunitReadonlyScope,
	admin.AdminDirectoryRolemanagementReadonlyScope,
	admin.AdminDirectoryDomainReadonlyScope,
	admin.AdminDirectoryUserschemaReadonlyScope,
}

// Config holds the resolved provider configuration.
type Config struct {
	CustomerID            string
	ServiceAccount        string
	ImpersonatedUserEmail string
	AccessToken           string
}

// Client is the shared API client handed to resources and data sources.
type Client struct {
	CustomerID string
	Directory  *admin.Service
}

// New resolves credentials and constructs the Admin SDK Directory client.
//
// Authentication is domain-wide delegation only: a service account with DWD
// impersonates an admin user (the subject). The caller's Application Default
// Credentials must hold roles/iam.serviceAccountTokenCreator on the service
// account, so the delegation token is minted keyless (no key on disk).
// GOOGLEWORKSPACE_ACCESS_TOKEN may carry a pre-minted DWD token (e.g. in CI).
func New(ctx context.Context, cfg Config) (*Client, error) {
	customerID := firstNonEmpty(cfg.CustomerID, os.Getenv("GOOGLEWORKSPACE_CUSTOMER_ID"), DefaultCustomerID)

	ts, err := tokenSource(ctx, cfg)
	if err != nil {
		return nil, err
	}

	dir, err := admin.NewService(ctx, option.WithTokenSource(ts))
	if err != nil {
		return nil, fmt.Errorf("creating Admin SDK Directory client: %w", err)
	}

	return &Client{CustomerID: customerID, Directory: dir}, nil
}

func tokenSource(ctx context.Context, cfg Config) (oauth2.TokenSource, error) {
	// Escape hatch: a pre-minted DWD access token.
	if token := firstNonEmpty(cfg.AccessToken, os.Getenv("GOOGLEWORKSPACE_ACCESS_TOKEN")); token != "" {
		return oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token}), nil
	}

	sa := firstNonEmpty(cfg.ServiceAccount, os.Getenv("GOOGLEWORKSPACE_SERVICE_ACCOUNT"))
	subject := firstNonEmpty(cfg.ImpersonatedUserEmail, os.Getenv("GOOGLEWORKSPACE_IMPERSONATED_USER_EMAIL"))
	if sa == "" || subject == "" {
		return nil, fmt.Errorf("domain-wide delegation requires service_account and " +
			"impersonated_user_email (or set GOOGLEWORKSPACE_ACCESS_TOKEN to a pre-minted token); " +
			"see the provider documentation")
	}

	ts, err := impersonate.CredentialsTokenSource(ctx, impersonate.CredentialsConfig{
		TargetPrincipal: sa,
		Scopes:          DefaultScopes,
		Subject:         subject, // setting Subject performs domain-wide delegation
	})
	if err != nil {
		return nil, fmt.Errorf("configuring domain-wide delegation for %s as %s: %w", sa, subject, err)
	}
	return ts, nil
}

// firstNonEmpty returns the first non-empty string, or "" if all are empty.
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
