package awssecrets

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
)

func TestBuildSecretsPopulatesFields(t *testing.T) {
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	raw := []types.SecretListEntry{
		{
			Name:            aws.String("/app/db"),
			ARN:             aws.String("arn:aws:secretsmanager:eu-central-1:123456789012:secret:/app/db-abc123"),
			LastChangedDate: &ts,
			RotationEnabled: aws.Bool(true),
		},
	}

	got := buildSecrets(raw)

	if len(got) != 1 {
		t.Fatalf("buildSecrets() len = %d, want 1", len(got))
	}
	s := got[0]
	if s.Name != "/app/db" {
		t.Errorf("Name = %q, want %q", s.Name, "/app/db")
	}
	if s.ARN != "arn:aws:secretsmanager:eu-central-1:123456789012:secret:/app/db-abc123" {
		t.Errorf("ARN = %q, want the given ARN", s.ARN)
	}
	if !s.LastChanged.Equal(ts) {
		t.Errorf("LastChanged = %v, want %v", s.LastChanged, ts)
	}
	if !s.RotationEnabled {
		t.Error("RotationEnabled = false, want true")
	}
}

func TestBuildSecretsHandlesNilLastChangedDate(t *testing.T) {
	raw := []types.SecretListEntry{
		{Name: aws.String("/app/x"), LastChangedDate: nil},
	}

	got := buildSecrets(raw)

	if !got[0].LastChanged.IsZero() {
		t.Errorf("LastChanged = %v, want zero value when AWS returns nil", got[0].LastChanged)
	}
}

func TestBuildSecretsHandlesNilRotationEnabled(t *testing.T) {
	raw := []types.SecretListEntry{
		{Name: aws.String("/app/x"), RotationEnabled: nil},
	}

	got := buildSecrets(raw)

	if got[0].RotationEnabled {
		t.Error("RotationEnabled = true, want false when AWS returns nil")
	}
}

func TestBuildSecretsSortsByName(t *testing.T) {
	raw := []types.SecretListEntry{
		{Name: aws.String("/z")},
		{Name: aws.String("/a")},
		{Name: aws.String("/m")},
	}

	got := buildSecrets(raw)

	want := []string{"/a", "/m", "/z"}
	for i, w := range want {
		if got[i].Name != w {
			t.Errorf("got[%d].Name = %q, want %q", i, got[i].Name, w)
		}
	}
}

func TestBuildSecretsEmptyInput(t *testing.T) {
	got := buildSecrets(nil)
	if len(got) != 0 {
		t.Errorf("buildSecrets(nil) = %+v, want empty", got)
	}
}
