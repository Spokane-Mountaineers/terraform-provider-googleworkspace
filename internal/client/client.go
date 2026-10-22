// Package client wraps the Google Workspace APIs the provider builds on:
// the Admin SDK Directory API, and per-mailbox Gmail settings.
package client

import (
	"context"
	"fmt"
	"os"
	"sync"

	"golang.org/x/oauth2"
	admin "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/cloudidentity/v1"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/impersonate"
	"google.golang.org/api/option"
)

// DefaultCustomerID is the Admin SDK alias for the caller's own account.
const DefaultCustomerID = "my_customer"

// DefaultScopes are the OAuth scopes requested when minting a domain-wide
// delegation token. They are read-write so the provider can manage directory
// resources, and must be authorized for the service account under
// Admin console → Security → API controls → Domain-wide delegation.
var DefaultScopes = []string{
	admin.AdminDirectoryUserScope,
	admin.AdminDirectoryGroupScope,
	admin.AdminDirectoryGroupMemberScope,
	admin.AdminDirectoryOrgunitScope,
	admin.AdminDirectoryRolemanagementScope,
	admin.AdminDirectoryDomainScope,
	admin.AdminDirectoryUserschemaScope,
}

// GmailSharingScope is requested only for per-mailbox Gmail tokens (delegation
// settings), never with DefaultScopes, so stacks that do not manage Gmail settings
// need not authorize it.
const GmailSharingScope = gmail.GmailSettingsSharingScope

// PoliciesScope is requested only for Cloud Identity Policy API reads, with its own
// token, so stacks that do not read policies need not authorize it.
const PoliciesScope = cloudidentity.CloudIdentityPoliciesScope

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

	// For clients that need their own delegation token (a different subject or
	// scope); unset when using a pre-minted access token.
	ctx            context.Context
	serviceAccount string
	adminSubject   string
	mu             sync.Mutex
	gmail          map[string]*gmail.Service
	policies       *cloudidentity.Service
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

	// The token source refreshes lazily, long after the caller's context (the
	// provider's Configure request) has been canceled, so detach from it.
	ts, err := tokenSource(context.WithoutCancel(ctx), cfg)
	if err != nil {
		return nil, err
	}

	dir, err := admin.NewService(ctx, option.WithTokenSource(ts))
	if err != nil {
		return nil, fmt.Errorf("creating Admin SDK Directory client: %w", err)
	}

	return &Client{
		CustomerID:     customerID,
		Directory:      dir,
		ctx:            context.WithoutCancel(ctx),
		serviceAccount: firstNonEmpty(cfg.ServiceAccount, os.Getenv("GOOGLEWORKSPACE_SERVICE_ACCOUNT")),
		adminSubject:   firstNonEmpty(cfg.ImpersonatedUserEmail, os.Getenv("GOOGLEWORKSPACE_IMPERSONATED_USER_EMAIL")),
		gmail:          map[string]*gmail.Service{},
	}, nil
}

// GmailFor returns a Gmail client acting as mailbox, which Gmail settings APIs
// require: they only operate on the authenticated user's own mailbox. It mints a
// domain-wide delegation token with mailbox as the subject and caches the client.
func (c *Client) GmailFor(mailbox string) (*gmail.Service, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if svc, ok := c.gmail[mailbox]; ok {
		return svc, nil
	}
	ts, err := c.delegatedTokenSource(mailbox, GmailSharingScope)
	if err != nil {
		return nil, err
	}
	svc, err := gmail.NewService(c.ctx, option.WithTokenSource(ts))
	if err != nil {
		return nil, fmt.Errorf("creating Gmail client for %s: %w", mailbox, err)
	}
	c.gmail[mailbox] = svc
	return svc, nil
}

// Policies returns a Cloud Identity client for the Policy API, acting as the admin
// subject with only PoliciesScope.
func (c *Client) Policies() (*cloudidentity.Service, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.policies != nil {
		return c.policies, nil
	}
	ts, err := c.delegatedTokenSource(c.adminSubject, PoliciesScope)
	if err != nil {
		return nil, err
	}
	svc, err := cloudidentity.NewService(c.ctx, option.WithTokenSource(ts))
	if err != nil {
		return nil, fmt.Errorf("creating Cloud Identity client: %w", err)
	}
	c.policies = svc
	return svc, nil
}

// delegatedTokenSource mints domain-wide delegation tokens for subject with only
// the given scope. It needs service_account; a pre-minted token has a fixed
// subject and scope set.
func (c *Client) delegatedTokenSource(subject, scope string) (oauth2.TokenSource, error) {
	if c.serviceAccount == "" || subject == "" {
		return nil, fmt.Errorf("requesting %s as %q requires service_account and a subject (domain-wide "+
			"delegation); a pre-minted access token cannot do this", scope, subject)
	}
	ts, err := impersonate.CredentialsTokenSource(c.ctx, impersonate.CredentialsConfig{
		TargetPrincipal: c.serviceAccount,
		Scopes:          []string{scope},
		Subject:         subject,
	})
	if err != nil {
		return nil, fmt.Errorf("configuring domain-wide delegation for %s as %s: %w", c.serviceAccount, subject, err)
	}
	return ts, nil
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
