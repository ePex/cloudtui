// Package config loads the tui shell's settings, connections, and AWS
// favorites from ~/.cloudtui/ — config.yaml (appearance/settings),
// connections/jolokia.yaml and connections/proxy.yaml, and
// favorites.yaml — falling back to built-in defaults when they're
// absent. Split into separate files so connections and favorites can
// be copied or shared independently of appearance settings and each
// other; see settingsFile's doc comment.
package config

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed themes/*.yaml
var themesFS embed.FS

// Connection holds all settings for a single named broker connection.
type Connection struct {
	Name    string      `yaml:"name"`
	Backend string      `yaml:"backend"` // "jolokia" | "proxy"
	Queue   QueueConfig `yaml:"queue"`
	Proxy   ProxyConfig `yaml:"proxy"`
}

// Config holds everything about the shell's appearance and connections a user can override.
type Config struct {
	ActiveConnection string       `yaml:"activeConnection"`
	Connections      []Connection `yaml:"connections"`
	// ActiveAWSProfile is the AWS CLI profile currently selected in the
	// Settings -> AWS Profiles picker. Independent of Connections/backends —
	// this slice of AWS support is discovery/selection only, not yet wired
	// to any broker connection. Empty means none selected.
	ActiveAWSProfile string        `yaml:"activeAWSProfile"`
	Datadog          DatadogConfig `yaml:"datadog"`
	Theme            string        `yaml:"theme"` // name of an embedded theme file (e.g. "dark", "cyberpunk")
	Logo             []string      `yaml:"logo"`
	Colors           Palette       `yaml:"colors"`
	AWSFavorites     AWSFavorites  `yaml:"awsFavorites,omitempty"`
}

// settingsFile is config.yaml's on-disk shape: everything except
// Connections and AWSFavorites, which live in their own files under
// the same directory (connections/jolokia.yaml, connections/proxy.yaml,
// favorites.yaml — see favoritesPath/connectionsDir) so they can be
// copied or shared independently of appearance settings and each
// other. Config itself is unchanged and still holds all of it in
// memory; only Load/Save's on-disk representation is split.
type settingsFile struct {
	ActiveConnection string        `yaml:"activeConnection"`
	ActiveAWSProfile string        `yaml:"activeAWSProfile"`
	Datadog          DatadogConfig `yaml:"datadog"`
	Theme            string        `yaml:"theme"`
	Logo             []string      `yaml:"logo"`
	Colors           Palette       `yaml:"colors"`
}

// FavoriteKind identifies which of AWSFavorites' three namespaces a
// favorite belongs to. Parameters, secrets, and log groups are
// independent namespaces — the same name can be favorited in one and not
// another.
type FavoriteKind string

const (
	FavoriteSSMParameter FavoriteKind = "ssmParameter"
	FavoriteSecret       FavoriteKind = "secret"
	FavoriteLogGroup     FavoriteKind = "logGroup"
)

// AWSFavorites holds favorited item names per AWS profile, one map per
// FavoriteKind — a parameter/secret/log-group name is only meaningful
// within the account a profile points at, so favorites don't apply
// globally. Sparse: an unlisted profile or name means "not favorited",
// not an error.
type AWSFavorites struct {
	SSMParameters map[string][]string `yaml:"ssmParameters,omitempty"` // profile -> favorited parameter names
	Secrets       map[string][]string `yaml:"secrets,omitempty"`       // profile -> favorited secret names
	LogGroups     map[string][]string `yaml:"logGroups,omitempty"`     // profile -> favorited log group names
}

// mapFor returns the map for kind, or nil for an unrecognized kind.
func (f AWSFavorites) mapFor(kind FavoriteKind) map[string][]string {
	switch kind {
	case FavoriteSSMParameter:
		return f.SSMParameters
	case FavoriteSecret:
		return f.Secrets
	case FavoriteLogGroup:
		return f.LogGroups
	default:
		return nil
	}
}

// IsFavorite reports whether name is favorited under kind/profile.
func (f AWSFavorites) IsFavorite(kind FavoriteKind, profile, name string) bool {
	for _, n := range f.mapFor(kind)[profile] {
		if n == name {
			return true
		}
	}
	return false
}

// Toggle returns a copy of f with name's favorite status in kind/profile
// flipped (favorited -> unfavorited or vice versa).
func (f AWSFavorites) Toggle(kind FavoriteKind, profile, name string) AWSFavorites {
	out := AWSFavorites{
		SSMParameters: cloneFavoritesMap(f.SSMParameters),
		Secrets:       cloneFavoritesMap(f.Secrets),
		LogGroups:     cloneFavoritesMap(f.LogGroups),
	}

	var target *map[string][]string
	switch kind {
	case FavoriteSSMParameter:
		target = &out.SSMParameters
	case FavoriteSecret:
		target = &out.Secrets
	case FavoriteLogGroup:
		target = &out.LogGroups
	default:
		return out
	}
	if *target == nil {
		*target = make(map[string][]string)
	}

	names := (*target)[profile]
	for i, n := range names {
		if n == name {
			(*target)[profile] = append(names[:i:i], names[i+1:]...)
			if len((*target)[profile]) == 0 {
				delete(*target, profile)
			}
			return out
		}
	}
	(*target)[profile] = append(names, name)
	return out
}

func cloneFavoritesMap(m map[string][]string) map[string][]string {
	if m == nil {
		return nil
	}
	out := make(map[string][]string, len(m))
	for k, v := range m {
		out[k] = append([]string(nil), v...)
	}
	return out
}

// DatadogConfig holds the settings for the Datadog Logs view (see
// spec/39-fe-datadog-logs). AccessToken is a Personal Access Token
// (Personal Settings -> Access Tokens in Datadog, scope
// "logs_read_data") — not the classic API Key + Application Key pair;
// a PAT authenticates alone via "Authorization: Bearer <token>". Can
// also be supplied via DD_ACCESS_TOKEN instead of stored here, same
// pattern as MQPROXY_CLIENT_PASSWORD for Connection passwords.
type DatadogConfig struct {
	Site        string `yaml:"site"` // e.g. "datadoghq.com", "datadoghq.eu"; API host is api.<site>
	AccessToken string `yaml:"accessToken"`
}

// ActiveConn returns the active Connection by name, falling back to the first
// connection if the name is not found, or a zero Connection if the list is empty.
func (c Config) ActiveConn() Connection {
	for _, conn := range c.Connections {
		if conn.Name == c.ActiveConnection {
			return conn
		}
	}
	if len(c.Connections) > 0 {
		return c.Connections[0]
	}
	return Connection{}
}

// SecretAWSProfile returns the AWS profile used to resolve c's password
// secret, or "" if c authenticates with a plain password instead
// (backend-appropriate PasswordSecret is empty) — including a
// hand-edited config with PasswordSecretAWSProfile set but PasswordSecret
// left blank, which doesn't actually use a secret.
func (c Connection) SecretAWSProfile() string {
	if c.Backend == "proxy" {
		if c.Proxy.PasswordSecret == "" {
			return ""
		}
		return c.Proxy.PasswordSecretAWSProfile
	}
	if c.Queue.PasswordSecret == "" {
		return ""
	}
	return c.Queue.PasswordSecretAWSProfile
}

// QueueConfig holds the connection settings for the ActiveMQ Jolokia backend.
// Password is intentionally kept out of version control: leave it empty here
// and set MQPROXY_CLIENT_PASSWORD instead.
type QueueConfig struct {
	BrokerName string `yaml:"brokerName"` // broker name used in JMX MBean ObjectNames
	URL        string `yaml:"url"`        // Jolokia HTTP endpoint
	Username   string `yaml:"username"`
	Password   string `yaml:"password"`
	// PasswordSecret, when non-empty, names an AWS Secrets Manager secret
	// resolved at connect time via PasswordSecretAWSProfile; it takes
	// precedence over Password (see spec/12-named-connections).
	PasswordSecret string `yaml:"passwordSecret,omitempty"`
	// PasswordSecretAWSProfile names the AWS profile used to resolve
	// PasswordSecret — required whenever PasswordSecret is set.
	// Independent of cfg.ActiveAWSProfile, the profile used for SSM
	// Parameters/Secrets Manager/CloudWatch Logs/CodePipeline browsing —
	// switching that one has no effect on an already-configured
	// connection's password.
	PasswordSecretAWSProfile string `yaml:"passwordSecretAWSProfile,omitempty"`
}

// ProxyConfig holds the connection settings for the mq-proxy backend.
type ProxyConfig struct {
	URL      string `yaml:"url"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	// PasswordSecret, when non-empty, names an AWS Secrets Manager secret
	// resolved at connect time via PasswordSecretAWSProfile; it takes
	// precedence over Password (see spec/12-named-connections).
	PasswordSecret string `yaml:"passwordSecret,omitempty"`
	// PasswordSecretAWSProfile names the AWS profile used to resolve
	// PasswordSecret — required whenever PasswordSecret is set.
	// Independent of cfg.ActiveAWSProfile, the profile used for SSM
	// Parameters/Secrets Manager/CloudWatch Logs/CodePipeline browsing —
	// switching that one has no effect on an already-configured
	// connection's password.
	PasswordSecretAWSProfile string `yaml:"passwordSecretAWSProfile,omitempty"`
}

// Palette is the set of named colors used across the shell chrome. Values are
// tview/tcell color names (e.g. "yellow") or hex codes (e.g. "#ffcc00").
type Palette struct {
	Background    string `yaml:"background,omitempty"`
	Border        string `yaml:"border,omitempty"`
	Label         string `yaml:"label,omitempty"`
	Text          string `yaml:"text,omitempty"`
	Value         string `yaml:"value,omitempty"`
	Accent        string `yaml:"accent,omitempty"`
	SelectionBg   string `yaml:"selectionBg,omitempty"`
	SelectionText string `yaml:"selectionText,omitempty"`
	StatusBarBg   string `yaml:"statusBarBg,omitempty"`
	StatusBarText string `yaml:"statusBarText,omitempty"`

	// Views maps a view name to the color used for that view's border/title.
	// Falls back to Border for any name not listed.
	Views map[string]string `yaml:"views,omitempty"`
}

// ViewColor returns the configured color for the named view, falling back to
// Border if the view isn't listed — so a view added later without a palette
// update still gets a sensible border color.
func (p Palette) ViewColor(name string) string {
	if c, ok := p.Views[name]; ok && c != "" {
		return c
	}
	return p.Border
}

// ApplyPaletteOverrides returns a copy of base with any non-empty field in
// overrides applied on top. For Views, individual keys are merged rather than
// the whole map being replaced.
func ApplyPaletteOverrides(base, overrides Palette) Palette {
	result := base
	if overrides.Background != "" {
		result.Background = overrides.Background
	}
	if overrides.Border != "" {
		result.Border = overrides.Border
	}
	if overrides.Label != "" {
		result.Label = overrides.Label
	}
	if overrides.Text != "" {
		result.Text = overrides.Text
	}
	if overrides.Value != "" {
		result.Value = overrides.Value
	}
	if overrides.Accent != "" {
		result.Accent = overrides.Accent
	}
	if overrides.SelectionBg != "" {
		result.SelectionBg = overrides.SelectionBg
	}
	if overrides.SelectionText != "" {
		result.SelectionText = overrides.SelectionText
	}
	if overrides.StatusBarBg != "" {
		result.StatusBarBg = overrides.StatusBarBg
	}
	if overrides.StatusBarText != "" {
		result.StatusBarText = overrides.StatusBarText
	}
	if len(overrides.Views) > 0 {
		merged := make(map[string]string, len(base.Views)+len(overrides.Views))
		for k, v := range base.Views {
			merged[k] = v
		}
		for k, v := range overrides.Views {
			merged[k] = v
		}
		result.Views = merged
	}
	return result
}

// PaletteUserOverrides returns a sparse Palette containing only the fields
// where effective differs from base. Used at save time to strip theme defaults
// so only genuine user customisations are written to config.yaml.
func PaletteUserOverrides(effective, base Palette) Palette {
	var out Palette
	if effective.Background != base.Background {
		out.Background = effective.Background
	}
	if effective.Border != base.Border {
		out.Border = effective.Border
	}
	if effective.Label != base.Label {
		out.Label = effective.Label
	}
	if effective.Text != base.Text {
		out.Text = effective.Text
	}
	if effective.Value != base.Value {
		out.Value = effective.Value
	}
	if effective.Accent != base.Accent {
		out.Accent = effective.Accent
	}
	if effective.SelectionBg != base.SelectionBg {
		out.SelectionBg = effective.SelectionBg
	}
	if effective.SelectionText != base.SelectionText {
		out.SelectionText = effective.SelectionText
	}
	if effective.StatusBarBg != base.StatusBarBg {
		out.StatusBarBg = effective.StatusBarBg
	}
	if effective.StatusBarText != base.StatusBarText {
		out.StatusBarText = effective.StatusBarText
	}
	for k, v := range effective.Views {
		bv := base.Views[k]
		if v != bv {
			if out.Views == nil {
				out.Views = make(map[string]string)
			}
			out.Views[k] = v
		}
	}
	return out
}

// PaletteForTheme loads the built-in palette for name from the embedded
// themes directory. Returns the palette and true on success, or the zero
// Palette and false for an unknown name or malformed file.
func PaletteForTheme(name string) (Palette, bool) {
	data, err := themesFS.ReadFile("themes/" + name + ".yaml")
	if err != nil {
		return Palette{}, false
	}
	var p Palette
	if err := yaml.Unmarshal(data, &p); err != nil {
		return Palette{}, false
	}
	return p, true
}

// AvailableThemes returns the sorted list of built-in theme names derived
// from the embedded themes directory (one name per .yaml file, extension stripped).
func AvailableThemes() []string {
	entries, err := fs.ReadDir(themesFS, "themes")
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".yaml") {
			names = append(names, strings.TrimSuffix(e.Name(), ".yaml"))
		}
	}
	sort.Strings(names)
	return names
}

// Default returns the built-in configuration used when no config file is
// present or a config file only overrides some fields.
func Default() Config {
	p, _ := PaletteForTheme("dark")
	return Config{
		ActiveConnection: "default",
		Connections: []Connection{
			{
				Name:    "default",
				Backend: "jolokia",
				Queue: QueueConfig{
					BrokerName: "localhost",
					URL:        "http://localhost:8161/api/jolokia",
					Username:   "admin",
					Password:   "",
				},
			},
		},
		Theme: "dark",
		Logo: []string{
			"╔═══════════╗",
			"║ CLOUDTUI  ║",
			"╚═══════════╝",
		},
		Colors: p,
	}
}

// favoritesPath returns the favorites.yaml path sibling to settingsPath
// (the config.yaml path Load/Save/LoadDefault/SaveDefault already take/
// resolve) — so callers don't need a second path parameter.
func favoritesPath(settingsPath string) string {
	return filepath.Join(filepath.Dir(settingsPath), "favorites.yaml")
}

// connectionsDir returns the connections/ directory sibling to
// settingsPath, holding jolokia.yaml and proxy.yaml.
func connectionsDir(settingsPath string) string {
	return filepath.Join(filepath.Dir(settingsPath), "connections")
}

// loadConnectionList reads path's connections list (a bare YAML
// sequence, no wrapper key — the filename already scopes it to one
// backend type). A missing file returns (nil, false, nil), not an
// error, so Load can distinguish "not yet migrated to the split
// format" (existed == false for both jolokia.yaml and proxy.yaml) from
// "migrated, but no connections of this type" (existed == true, list
// empty).
func loadConnectionList(path string) (conns []Connection, existed bool, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("reading connections %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, &conns); err != nil {
		return nil, false, fmt.Errorf("parsing connections %s: %w", path, err)
	}
	return conns, true, nil
}

// loadFavorites reads path's favorites (bare AWSFavorites document, no
// wrapper key). A missing file returns (zero AWSFavorites, false,
// nil), not an error.
func loadFavorites(path string) (fav AWSFavorites, existed bool, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return AWSFavorites{}, false, nil
		}
		return AWSFavorites{}, false, fmt.Errorf("reading favorites %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, &fav); err != nil {
		return AWSFavorites{}, false, fmt.Errorf("parsing favorites %s: %w", path, err)
	}
	return fav, true, nil
}

// Load reads and parses the YAML config at path (settings) plus its
// sibling connections/jolokia.yaml, connections/proxy.yaml, and
// favorites.yaml (see favoritesPath/connectionsDir), merging on top of
// Default() so a partial/missing file still gets defaults for unset
// fields. A missing settings file is not an error — Default() is used
// as-is.
//
// The effective Colors palette is derived from the active theme plus any
// explicit per-field overrides present in the file's colors: block.
//
// Legacy fallback: a config.yaml written before this file split (still
// carrying embedded connections:/awsFavorites: keys, or even older
// pre-FE22 top-level backend/queue/proxy fields) loads correctly as
// long as the corresponding split file(s) don't exist yet — split
// files always win if present, so a stale hand-edit left in config.yaml
// after migration can't silently override the real, current files.
// Purely in-memory: nothing is rewritten until the next Save.
//
// If MQPROXY_CLIENT_PASSWORD is set and Queue.Password is empty, the env var
// value is injected so credentials stay out of config.yaml.
func Load(path string) (Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return Config{}, fmt.Errorf("reading config %s: %w", path, err)
	}

	if len(data) > 0 {
		sf := settingsFile{
			ActiveConnection: cfg.ActiveConnection,
			ActiveAWSProfile: cfg.ActiveAWSProfile,
			Datadog:          cfg.Datadog,
			Theme:            cfg.Theme,
			Logo:             cfg.Logo,
			Colors:           cfg.Colors,
		}
		if err := yaml.Unmarshal(data, &sf); err != nil {
			return Config{}, fmt.Errorf("parsing config %s: %w", path, err)
		}
		cfg.ActiveConnection = sf.ActiveConnection
		cfg.ActiveAWSProfile = sf.ActiveAWSProfile
		cfg.Datadog = sf.Datadog
		cfg.Theme = sf.Theme
		cfg.Logo = sf.Logo

		// Second unmarshal into a zero settingsFile captures only the fields
		// explicitly present in the YAML (no Default() noise), so
		// raw.Colors holds the user's actual color overrides rather than a
		// blend of defaults and overrides.
		var raw settingsFile
		if err := yaml.Unmarshal(data, &raw); err != nil {
			return Config{}, fmt.Errorf("parsing config %s: %w", path, err)
		}

		// Compute effective palette: theme base + user color overrides.
		base, ok := PaletteForTheme(cfg.Theme)
		if !ok {
			base, _ = PaletteForTheme("dark")
		}
		cfg.Colors = ApplyPaletteOverrides(base, raw.Colors)
	}

	// Connections: split files win if either exists; otherwise fall back
	// to whatever's embedded in the settings file (current or pre-FE22
	// legacy shape).
	jolokiaPath := filepath.Join(connectionsDir(path), "jolokia.yaml")
	proxyPath := filepath.Join(connectionsDir(path), "proxy.yaml")
	jolokiaConns, jolokiaExisted, err := loadConnectionList(jolokiaPath)
	if err != nil {
		return Config{}, err
	}
	proxyConns, proxyExisted, err := loadConnectionList(proxyPath)
	if err != nil {
		return Config{}, err
	}
	switch {
	case jolokiaExisted || proxyExisted:
		cfg.Connections = append(append([]Connection{}, jolokiaConns...), proxyConns...)
	case len(data) > 0:
		var legacy struct {
			Connections []Connection `yaml:"connections"`
		}
		_ = yaml.Unmarshal(data, &legacy)
		if len(legacy.Connections) > 0 {
			cfg.Connections = legacy.Connections
		} else {
			// Pre-FE22: no connections: key at all, just top-level
			// backend/queue/proxy fields.
			var legacyFields struct {
				Backend string      `yaml:"backend"`
				Queue   QueueConfig `yaml:"queue"`
				Proxy   ProxyConfig `yaml:"proxy"`
			}
			_ = yaml.Unmarshal(data, &legacyFields)
			if legacyFields.Queue.URL != "" || legacyFields.Backend != "" || legacyFields.Proxy.URL != "" {
				if legacyFields.Backend == "" {
					legacyFields.Backend = "jolokia"
				}
				cfg.Connections = []Connection{{
					Name:    "default",
					Backend: legacyFields.Backend,
					Queue:   legacyFields.Queue,
					Proxy:   legacyFields.Proxy,
				}}
				cfg.ActiveConnection = "default"
			}
		}
	}

	// Favorites: split file wins if it exists; otherwise fall back to
	// whatever's embedded in the settings file.
	fav, favExisted, err := loadFavorites(favoritesPath(path))
	if err != nil {
		return Config{}, err
	}
	if favExisted {
		cfg.AWSFavorites = fav
	} else if len(data) > 0 {
		var legacyFav struct {
			AWSFavorites AWSFavorites `yaml:"awsFavorites"`
		}
		_ = yaml.Unmarshal(data, &legacyFav)
		cfg.AWSFavorites = legacyFav.AWSFavorites
	}

	// Env-var password injection: applies to all connections so switching
	// connections in the manager also picks up the env var.
	if p := os.Getenv("MQPROXY_CLIENT_PASSWORD"); p != "" {
		for i := range cfg.Connections {
			switch cfg.Connections[i].Backend {
			case "proxy":
				if cfg.Connections[i].Proxy.Password == "" {
					cfg.Connections[i].Proxy.Password = p
				}
			default:
				if cfg.Connections[i].Queue.Password == "" {
					cfg.Connections[i].Queue.Password = p
				}
			}
		}
	}

	// Env-var access-token injection: only applied when the config file
	// didn't already set one explicitly (same rule as the password
	// injection above).
	if t := os.Getenv("DD_ACCESS_TOKEN"); t != "" && cfg.Datadog.AccessToken == "" {
		cfg.Datadog.AccessToken = t
	}

	return cfg, nil
}

// DefaultPath returns the path to the user's config.yaml
// (~/.cloudtui/config.yaml) — the single location LoadDefault and
// SaveDefault read from and write to, regardless of the process's
// current working directory.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home directory: %w", err)
	}
	return filepath.Join(home, ".cloudtui", "config.yaml"), nil
}

// migrateLegacyConfig copies legacyPath to newPath the first time newPath
// doesn't exist yet, preserving a pre-relocation, cwd-relative config.yaml
// (e.g. tui/config.yaml under the old dev workflow) instead of silently
// discarding it. A no-op once newPath exists, or if legacyPath doesn't.
func migrateLegacyConfig(legacyPath, newPath string) error {
	if _, err := os.Stat(newPath); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	data, err := os.ReadFile(legacyPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	if err := os.MkdirAll(filepath.Dir(newPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(newPath, data, 0o644); err != nil {
		return err
	}
	slog.Info("config: migrated legacy config.yaml", "from", legacyPath, "to", newPath)
	return nil
}

// LoadDefault loads config.yaml from the user's home config directory
// (~/.cloudtui/config.yaml, see DefaultPath), migrating a pre-existing
// cwd-relative config.yaml into place on first run if one is found.
func LoadDefault() (Config, error) {
	path, err := DefaultPath()
	if err != nil {
		return Config{}, err
	}
	if err := migrateLegacyConfig("config.yaml", path); err != nil {
		slog.Warn("config: legacy migration failed", "error", err)
	}
	return Load(path)
}

// saveConnectionList writes conns to path as a bare YAML sequence (no
// wrapper key). Always writes the file, even for an empty/nil conns —
// so once a config has been through one Save, jolokia.yaml/proxy.yaml
// existing (however empty) is what Load uses to tell "migrated, no
// connections of this type" apart from "not yet migrated" (see
// loadConnectionList).
func saveConnectionList(path string, conns []Connection) error {
	data, err := yaml.Marshal(conns)
	if err != nil {
		return fmt.Errorf("encoding connections: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing connections %s: %w", path, err)
	}
	return nil
}

// Save writes cfg to path (settings) and its sibling
// connections/jolokia.yaml, connections/proxy.yaml, and favorites.yaml
// (see favoritesPath/connectionsDir) as YAML. Connections are
// partitioned by Backend; anything not recognized as "proxy" is
// written to jolokia.yaml, matching ActiveConn()/SecretAWSProfile()'s
// existing "proxy is the one special case, everything else behaves
// like jolokia" convention.
//
// Not atomic across the 4 files (this codebase has no atomic-write
// mechanism for any single file either) — a failure partway through
// can leave them inconsistent with each other. Not addressed here;
// see spec-wip/fe-split-config-files's plan.md (now spec/01) for why.
func Save(path string, cfg Config) error {
	sf := settingsFile{
		ActiveConnection: cfg.ActiveConnection,
		ActiveAWSProfile: cfg.ActiveAWSProfile,
		Datadog:          cfg.Datadog,
		Theme:            cfg.Theme,
		Logo:             cfg.Logo,
		Colors:           cfg.Colors,
	}
	data, err := yaml.Marshal(sf)
	if err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing config %s: %w", path, err)
	}

	dir := connectionsDir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating connections directory %s: %w", dir, err)
	}
	var jolokiaConns, proxyConns []Connection
	for _, c := range cfg.Connections {
		if c.Backend == "proxy" {
			proxyConns = append(proxyConns, c)
		} else {
			jolokiaConns = append(jolokiaConns, c)
		}
	}
	if err := saveConnectionList(filepath.Join(dir, "jolokia.yaml"), jolokiaConns); err != nil {
		return err
	}
	if err := saveConnectionList(filepath.Join(dir, "proxy.yaml"), proxyConns); err != nil {
		return err
	}

	favData, err := yaml.Marshal(cfg.AWSFavorites)
	if err != nil {
		return fmt.Errorf("encoding favorites: %w", err)
	}
	if err := os.WriteFile(favoritesPath(path), favData, 0o644); err != nil {
		return fmt.Errorf("writing favorites %s: %w", favoritesPath(path), err)
	}
	return nil
}

// SaveDefault saves cfg to the user's home config directory, mirroring
// LoadDefault's path resolution.
func SaveDefault(cfg Config) error {
	path, err := DefaultPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}
	return Save(path, cfg)
}
