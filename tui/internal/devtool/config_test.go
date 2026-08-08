package devtool

import (
	"testing"

	"github.com/ePex/cloudtui/tui/internal/config"
)

func TestAddProxyConnectionAppends(t *testing.T) {
	cfg := config.Config{Connections: []config.Connection{
		{Name: "default", Alias: "def", Backend: "jolokia"},
	}}

	got, err := AddProxyConnection(cfg, "smoke", "smk", "http://localhost:8080", "cloudtui", "changeme")
	if err != nil {
		t.Fatalf("AddProxyConnection() error = %v", err)
	}
	if len(got.Connections) != 2 {
		t.Fatalf("Connections count = %d, want 2", len(got.Connections))
	}
	added := got.Connections[1]
	if added.Name != "smoke" || added.Alias != "smk" || added.Backend != "proxy" {
		t.Errorf("added connection = %+v, want name=smoke alias=smk backend=proxy", added)
	}
	if added.Proxy.URL != "http://localhost:8080" || added.Proxy.Username != "cloudtui" || added.Proxy.Password != "changeme" {
		t.Errorf("added connection proxy config = %+v", added.Proxy)
	}
	// Original untouched (pure function).
	if len(cfg.Connections) != 1 {
		t.Errorf("original cfg.Connections mutated: %v", cfg.Connections)
	}
}

func TestAddProxyConnectionRejectsDuplicateName(t *testing.T) {
	cfg := config.Config{Connections: []config.Connection{
		{Name: "default", Alias: "def", Backend: "jolokia"},
	}}
	if _, err := AddProxyConnection(cfg, "default", "x", "http://localhost:8080", "u", "p"); err == nil {
		t.Fatal("AddProxyConnection() error = nil, want an error for a duplicate name")
	}
}
