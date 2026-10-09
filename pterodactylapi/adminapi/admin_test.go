package adminapi

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/marte26/pterodactylBackup/internal/fakepanel"
)

func TestGetServersPaginates(t *testing.T) {
	panel := &fakepanel.Panel{
		PageSize: 2,
		Servers: []json.RawMessage{
			fakepanel.Server("a", 1), fakepanel.Server("b", 2), fakepanel.Server("c", 3),
			fakepanel.Server("d", 4), fakepanel.Server("e", 5),
		},
	}
	client := Client{URL: panel.Start(t)}

	servers, err := client.GetServers()
	if err != nil {
		t.Fatal(err)
	}

	var identifiers []string
	for _, server := range servers {
		identifiers = append(identifiers, server.Attributes.Identifier)
	}
	if strings.Join(identifiers, ",") != "a,b,c,d,e" {
		t.Errorf("servers = %v, want a,b,c,d,e", identifiers)
	}
	if servers[4].Attributes.FeatureLimits.Backups != 5 {
		t.Errorf("backup limit = %d, want 5", servers[4].Attributes.FeatureLimits.Backups)
	}
}

func TestGetServersNoServers(t *testing.T) {
	panel := &fakepanel.Panel{}
	client := Client{URL: panel.Start(t)}

	servers, err := client.GetServers()
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 0 {
		t.Errorf("servers = %v, want none", servers)
	}
}

func TestGetServersNullableFields(t *testing.T) {
	tests := []struct {
		name        string
		server      string
		wantExtID   string
		wantThreads string
	}{
		{
			name:   "null",
			server: `{"object":"server","attributes":{"identifier":"a","external_id":null,"limits":{"threads":null}}}`,
		},
		{
			name:        "strings",
			server:      `{"object":"server","attributes":{"identifier":"a","external_id":"ext-1","limits":{"threads":"0-3"}}}`,
			wantExtID:   "ext-1",
			wantThreads: "0-3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			panel := &fakepanel.Panel{Servers: []json.RawMessage{json.RawMessage(tt.server)}}
			client := Client{URL: panel.Start(t)}

			servers, err := client.GetServers()
			if err != nil {
				t.Fatal(err)
			}

			attributes := servers[0].Attributes
			if got := deref(attributes.ExternalID); got != tt.wantExtID {
				t.Errorf("external_id = %q, want %q", got, tt.wantExtID)
			}
			if got := deref(attributes.Limits.Threads); got != tt.wantThreads {
				t.Errorf("threads = %q, want %q", got, tt.wantThreads)
			}
		})
	}
}

func TestGetServersHTTPError(t *testing.T) {
	panel := &fakepanel.Panel{APIKey: "right", Servers: []json.RawMessage{fakepanel.Server("a", 1)}}
	client := Client{URL: panel.Start(t), APIKey: "wrong"}

	servers, err := client.GetServers()
	if err == nil {
		t.Fatalf("expected an error, got servers %v", servers)
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error = %v, want it to mention status 401", err)
	}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}

	return *s
}
