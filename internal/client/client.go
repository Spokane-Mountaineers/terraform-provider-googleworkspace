// Package client wraps the Google Workspace APIs the provider builds on:
// the Admin SDK Directory API and (later) the Cloud Identity API.
package client

import (
	"context"
	"fmt"
	"os"

	"golang.org/x/oauth2"
	admin "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/option"
)

// DefaultCustomerID is the Admin SDK alias for the caller's own account.
const DefaultCustomerID = "my_customer"

// Config holds the resolved provider configuration.
type Config struct {
	// CustomerID is the Workspace customer ID. Empty means DefaultCustomerID.
	CustomerID string
	// AccessToken is an optional OAuth2 access token. Empty falls back to the
	// GOOGLEWORKSPACE_ACCESS_TOKEN env var and then Application Default Credentials.
	AccessToken string
}

// Client is the shared API client handed to resources and data sources.
type Client struct {
	CustomerID string
	Directory  *admin.Service
}

// New resolves credentials and constructs the Admin SDK Directory client.
//
// Credential precedence (first target: keyless admin user token):
//  1. explicit AccessToken
//  2. GOOGLEWORKSPACE_ACCESS_TOKEN
//  3. Application Default Credentials — the operator must have run a scoped
//     `gcloud auth application-default login` with Admin SDK directory scopes.
func New(ctx context.Context, cfg Config) (*Client, error) {
	customerID := firstNonEmpty(cfg.CustomerID, os.Getenv("GOOGLEWORKSPACE_CUSTOMER_ID"), DefaultCustomerID)

	var opts []option.ClientOption
	if token := firstNonEmpty(cfg.AccessToken, os.Getenv("GOOGLEWORKSPACE_ACCESS_TOKEN")); token != "" {
		opts = append(opts, option.WithTokenSource(
			oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token}),
		))
	} else {
		// ADC fallback. User credentials carry the scopes granted at login time;
		// WithScopes only affects service-account/JWT flows.
		opts = append(opts, option.WithScopes(admin.AdminDirectoryDomainReadonlyScope))
	}

	dir, err := admin.NewService(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("creating Admin SDK Directory client: %w", err)
	}

	return &Client{CustomerID: customerID, Directory: dir}, nil
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
