package awsprofile

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestListFromMergesAndDeduplicates(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config")
	credsPath := filepath.Join(dir, "credentials")

	writeFile(t, configPath, `
[default]
region = us-east-1

[profile foo]
region = eu-west-1

[sso-session my-sso]
sso_start_url = https://example.com
`)
	writeFile(t, credsPath, `
[default]
aws_access_key_id = AAA

[bar]
aws_access_key_id = BBB

[foo]
aws_access_key_id = CCC
`)

	got, err := ListFrom(configPath, credsPath)
	if err != nil {
		t.Fatalf("ListFrom() error = %v", err)
	}

	want := []string{"bar", "default", "foo"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ListFrom() = %v, want %v", got, want)
	}
}

func TestListFromBothFilesMissing(t *testing.T) {
	dir := t.TempDir()

	got, err := ListFrom(filepath.Join(dir, "config"), filepath.Join(dir, "credentials"))
	if err != nil {
		t.Fatalf("ListFrom() error = %v, want nil", err)
	}
	if len(got) != 0 {
		t.Errorf("ListFrom() = %v, want empty", got)
	}
}

func TestListFromConfigOnlyExcludesSsoSessions(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config")

	writeFile(t, configPath, `
[sso-session my-sso]
sso_start_url = https://example.com

[profile only-one]
region = us-east-1
`)

	got, err := ListFrom(configPath, filepath.Join(dir, "missing-credentials"))
	if err != nil {
		t.Fatalf("ListFrom() error = %v", err)
	}

	want := []string{"only-one"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ListFrom() = %v, want %v", got, want)
	}
}
