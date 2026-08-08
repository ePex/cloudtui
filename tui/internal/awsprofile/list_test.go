package awsprofile

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// setupFixture points AWS_CONFIG_FILE/AWS_SHARED_CREDENTIALS_FILE at fresh
// temp files with the given contents (empty string = file not created at
// all, to exercise the "file doesn't exist" path distinctly from "file
// exists but is empty").
func setupFixture(t *testing.T, configContent, credentialsContent string) {
	t.Helper()
	dir := t.TempDir()

	configPath := filepath.Join(dir, "config")
	if configContent != "" {
		if err := os.WriteFile(configPath, []byte(configContent), 0o600); err != nil {
			t.Fatalf("write config fixture: %v", err)
		}
	}
	credsPath := filepath.Join(dir, "credentials")
	if credentialsContent != "" {
		if err := os.WriteFile(credsPath, []byte(credentialsContent), 0o600); err != nil {
			t.Fatalf("write credentials fixture: %v", err)
		}
	}

	t.Setenv("AWS_CONFIG_FILE", configPath)
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", credsPath)
}

func findProfile(t *testing.T, profiles []Profile, name string) Profile {
	t.Helper()
	for _, p := range profiles {
		if p.Name == name {
			return p
		}
	}
	t.Fatalf("profile %q not found in %+v", name, profiles)
	return Profile{}
}

func TestListNoFilesAtAll(t *testing.T) {
	setupFixture(t, "", "")

	profiles, err := List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v, want nil when neither file exists", err)
	}
	if len(profiles) != 0 {
		t.Errorf("List() = %+v, want empty", profiles)
	}
}

func TestListStaticKeysProfile(t *testing.T) {
	setupFixture(t, "", `
[static-keys-profile]
aws_access_key_id = AKIAEXAMPLE
aws_secret_access_key = secretexample
`)

	profiles, err := List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	p := findProfile(t, profiles, "static-keys-profile")
	if p.AuthType != AuthStaticKeys {
		t.Errorf("AuthType = %q, want %q", p.AuthType, AuthStaticKeys)
	}
}

func TestListSSOProfile(t *testing.T) {
	setupFixture(t, `
[profile sso-profile]
sso_start_url = https://example.awsapps.com/start
sso_region = us-east-1
sso_account_id = 123456789012
sso_role_name = ReadOnly
region = us-west-2
`, "")

	profiles, err := List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	p := findProfile(t, profiles, "sso-profile")
	if p.AuthType != AuthSSO {
		t.Errorf("AuthType = %q, want %q", p.AuthType, AuthSSO)
	}
	if p.Region != "us-west-2" {
		t.Errorf("Region = %q, want %q", p.Region, "us-west-2")
	}
}

func TestListAssumeRoleProfile(t *testing.T) {
	setupFixture(t, `
[profile base]
region = us-east-1

[profile assume-role-profile]
role_arn = arn:aws:iam::123456789012:role/Example
source_profile = base
region = us-east-1
`, `
[base]
aws_access_key_id = AKIAEXAMPLE
aws_secret_access_key = secretexample
`)

	profiles, err := List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	p := findProfile(t, profiles, "assume-role-profile")
	if p.AuthType != AuthAssumeRole {
		t.Errorf("AuthType = %q, want %q", p.AuthType, AuthAssumeRole)
	}
}

func TestListCredentialProcessProfile(t *testing.T) {
	setupFixture(t, `
[profile cred-process-profile]
credential_process = /bin/echo fake-credentials
region = eu-west-1
`, "")

	profiles, err := List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	p := findProfile(t, profiles, "cred-process-profile")
	if p.AuthType != AuthCredentialProcess {
		t.Errorf("AuthType = %q, want %q", p.AuthType, AuthCredentialProcess)
	}
}

// TestListMixedSSOAndCredentialProcessPrefersCredentialProcess matches a
// real-world pattern (seen live) where an internal tool wraps SSO login
// behind a credential_process script, leaving both sets of fields present.
func TestListMixedSSOAndCredentialProcessPrefersCredentialProcess(t *testing.T) {
	setupFixture(t, `
[profile mixed-profile]
sso_start_url = https://example.awsapps.com/start
sso_region = us-east-1
sso_account_id = 123456789012
sso_role_name = ReadOnly
credential_process = /bin/echo fake-credentials
region = us-east-1
`, "")

	profiles, err := List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	p := findProfile(t, profiles, "mixed-profile")
	if p.AuthType != AuthCredentialProcess {
		t.Errorf("AuthType = %q, want %q (credential_process should take precedence)", p.AuthType, AuthCredentialProcess)
	}
}

func TestListProfileInOnlyOneFile(t *testing.T) {
	setupFixture(t, `
[profile config-only]
region = ap-southeast-1
`, `
[creds-only]
aws_access_key_id = AKIAEXAMPLE
aws_secret_access_key = secretexample
`)

	profiles, err := List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	names := profileNames(profiles)
	for _, want := range []string{"config-only", "creds-only"} {
		found := false
		for _, n := range names {
			if n == want {
				found = true
			}
		}
		if !found {
			t.Errorf("profiles %v missing %q", names, want)
		}
	}
}

func TestListProfileInBothFilesIsNotDuplicated(t *testing.T) {
	setupFixture(t, `
[profile both-files]
region = us-east-1
`, `
[both-files]
aws_access_key_id = AKIAEXAMPLE
aws_secret_access_key = secretexample
`)

	profiles, err := List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	count := 0
	for _, p := range profiles {
		if p.Name == "both-files" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("profile %q appeared %d times, want 1", "both-files", count)
	}
	p := findProfile(t, profiles, "both-files")
	if p.Region != "us-east-1" {
		t.Errorf("Region = %q, want %q (merged from config file)", p.Region, "us-east-1")
	}
	if p.AuthType != AuthStaticKeys {
		t.Errorf("AuthType = %q, want %q (merged from credentials file)", p.AuthType, AuthStaticKeys)
	}
}

func TestListResultsAreSortedByName(t *testing.T) {
	setupFixture(t, `
[profile zebra]
region = us-east-1

[profile apple]
region = us-east-1
`, "")

	profiles, err := List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	names := profileNames(profiles)
	if !sort.StringsAreSorted(names) {
		t.Errorf("profile names not sorted: %v", names)
	}
}

func profileNames(profiles []Profile) []string {
	names := make([]string, len(profiles))
	for i, p := range profiles {
		names[i] = p.Name
	}
	return names
}
