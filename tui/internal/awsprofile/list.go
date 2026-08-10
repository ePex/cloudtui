package awsprofile

import (
	"context"
	"log/slog"
	"sort"

	"github.com/aws/aws-sdk-go-v2/config"
)

// List returns every profile discoverable in the shared AWS config and
// credentials files. Read-only: no credentials are resolved, refreshed, or
// validated, and no network calls are made. A profile that fails to load
// (e.g. a malformed entry) is skipped and logged rather than failing the
// whole call — this is a best-effort discovery aid, not a strict
// validator. Results are sorted by name for stable, predictable display.
func List(ctx context.Context) ([]Profile, error) {
	configFile, credentialsFile := configFiles()

	configNames, err := scanProfileNames(configFile)
	if err != nil {
		return nil, err
	}
	credNames, err := scanProfileNames(credentialsFile)
	if err != nil {
		return nil, err
	}

	all := map[string]bool{}
	for n := range configNames {
		all[n] = true
	}
	for n := range credNames {
		all[n] = true
	}

	names := make([]string, 0, len(all))
	for n := range all {
		names = append(names, n)
	}
	sort.Strings(names)

	profiles := make([]Profile, 0, len(names))
	for _, name := range names {
		sc, err := loadProfile(ctx, configFile, credentialsFile, name)
		if err != nil {
			slog.Warn("awsprofile: skipping unparseable profile", "profile", name, "error", err)
			continue
		}
		profiles = append(profiles, Profile{
			Name:     name,
			Region:   sc.Region,
			AuthType: classify(sc),
		})
	}
	return profiles, nil
}

// AuthTypeFor classifies a single named profile's authentication method,
// without scanning or loading every other profile the way List does —
// used where only one profile's auth type is needed (e.g. deciding
// whether an AWS error is SSO-reauth-eligible for the active profile).
func AuthTypeFor(ctx context.Context, name string) (AuthType, error) {
	configFile, credentialsFile := configFiles()
	sc, err := loadProfile(ctx, configFile, credentialsFile, name)
	if err != nil {
		return AuthUnknown, err
	}
	return classify(sc), nil
}

// loadProfile loads and parses a single profile from the given config/
// credentials files, shared by List (which loads every profile) and
// AuthTypeFor (which loads just one).
func loadProfile(ctx context.Context, configFile, credentialsFile, name string) (config.SharedConfig, error) {
	return config.LoadSharedConfigProfile(ctx, name,
		func(o *config.LoadSharedConfigOptions) {
			o.ConfigFiles = []string{configFile}
			o.CredentialsFiles = []string{credentialsFile}
		},
	)
}

// classify picks the auth method a profile actually uses. Fields for more
// than one method can be present at once (e.g. a profile with both
// sso_start_url and credential_process, seen in the wild where an internal
// tool wraps SSO login behind a credential_process script) — the order
// here follows the AWS SDK's own credential-provider precedence rather
// than inventing a different one.
func classify(sc config.SharedConfig) AuthType {
	switch {
	case sc.CredentialProcess != "":
		return AuthCredentialProcess
	case sc.RoleARN != "":
		return AuthAssumeRole
	case sc.SSOSessionName != "" || sc.SSOStartURL != "":
		return AuthSSO
	case sc.Credentials.AccessKeyID != "":
		return AuthStaticKeys
	default:
		return AuthUnknown
	}
}
