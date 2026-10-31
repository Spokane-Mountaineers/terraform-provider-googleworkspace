package provider

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	admin "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/option"
)

func TestGroupMemberResource_Schema(t *testing.T) {
	t.Parallel()

	var resp resource.SchemaResponse
	NewGroupMemberResource().Schema(context.Background(), resource.SchemaRequest{}, &resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics: %v", resp.Diagnostics)
	}
	for _, attr := range []string{"id", "group_id", "email", "role", "delivery_settings", "type", "member_id"} {
		if _, ok := resp.Schema.Attributes[attr]; !ok {
			t.Errorf("resource missing attribute %q", attr)
		}
	}
}

func TestMemberKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id, email string
		group     string
		key       string
		ok        bool
	}{
		{"01grp/02mbr", "a@example.org", "01grp", "02mbr", true},
		{"01grp/", "a@example.org", "01grp", "a@example.org", true},
		{"01grp/", "", "", "", false},
		{"noslash", "a@example.org", "", "", false},
		{"/02mbr", "a@example.org", "", "", false},
	}
	for _, tt := range tests {
		g, k, ok := memberKey(tt.id, tt.email)
		if g != tt.group || k != tt.key || ok != tt.ok {
			t.Errorf("memberKey(%q, %q) = (%q, %q, %v), want (%q, %q, %v)", tt.id, tt.email, g, k, ok, tt.group, tt.key, tt.ok)
		}
	}
}

func TestFindMember(t *testing.T) {
	t.Parallel()

	// One member resolves by email; the other only shows up in the member list.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/admin/directory/v1/groups/01grp/members/direct@example.org":
			_, _ = io.WriteString(w, `{"id":"1","email":"direct@example.org"}`)
		case "/admin/directory/v1/groups/01grp/members":
			if r.URL.Query().Get("pageToken") == "" {
				_, _ = io.WriteString(w, `{"members":[{"id":"1","email":"direct@example.org"}],"nextPageToken":"p2"}`)
				return
			}
			_, _ = io.WriteString(w, `{"members":[{"id":"2","email":"Listed@Example.org"}]}`)
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"error":{"code":404,"message":"Resource Not Found: email"}}`)
		}
	}))
	t.Cleanup(srv.Close)

	ctx := context.Background()
	svc, err := admin.NewService(ctx, option.WithEndpoint(srv.URL+"/"), option.WithHTTPClient(srv.Client()))
	if err != nil {
		t.Fatal(err)
	}

	for email, want := range map[string]string{"direct@example.org": "1", "listed@example.org": "2"} {
		m, err := findMember(ctx, svc, "01grp", email)
		if err != nil || m.Id != want {
			t.Errorf("findMember(%q) = %+v, %v; want id %q", email, m, err, want)
		}
	}
	if _, err := findMember(ctx, svc, "01grp", "absent@example.org"); !isNotFound(err) {
		t.Errorf("findMember(absent) error = %v, want a 404", err)
	}
}
