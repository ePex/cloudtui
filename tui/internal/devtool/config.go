package devtool

import (
	"fmt"

	"github.com/ePex/cloudtui/tui/internal/config"
)

// AddProxyConnection returns a copy of cfg with a new proxy-backend
// connection appended. It does not persist anything — callers decide
// whether/how to save, keeping this pure and easy to test. Errors if name
// is already used, matching the uniqueness rule the connection editor
// enforces interactively.
func AddProxyConnection(cfg config.Config, name, url, username, password string) (config.Config, error) {
	for _, c := range cfg.Connections {
		if c.Name == name {
			return cfg, fmt.Errorf("connection %q already exists", name)
		}
	}
	cfg.Connections = append(cfg.Connections, config.Connection{
		Name:    name,
		Backend: "proxy",
		Proxy:   config.ProxyConfig{URL: url, Username: username, Password: password},
	})
	return cfg, nil
}
