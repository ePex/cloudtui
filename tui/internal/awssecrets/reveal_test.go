package awssecrets

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

func TestExtractValuePrefersSecretString(t *testing.T) {
	out := &secretsmanager.GetSecretValueOutput{
		SecretString: aws.String(`{"username":"x"}`),
	}

	value, isBinary, err := extractValue(out)

	if err != nil {
		t.Fatalf("extractValue() error = %v, want nil", err)
	}
	if isBinary {
		t.Error("isBinary = true, want false for a SecretString-only output")
	}
	if value != `{"username":"x"}` {
		t.Errorf("value = %q, want %q", value, `{"username":"x"}`)
	}
}

func TestExtractValueReportsBinaryWithoutValue(t *testing.T) {
	out := &secretsmanager.GetSecretValueOutput{
		SecretBinary: []byte{0x01, 0x02, 0x03},
	}

	value, isBinary, err := extractValue(out)

	if err != nil {
		t.Fatalf("extractValue() error = %v, want nil", err)
	}
	if !isBinary {
		t.Error("isBinary = false, want true for a SecretBinary-only output")
	}
	if value != "" {
		t.Errorf("value = %q, want empty for a binary secret", value)
	}
}

func TestExtractValueErrorsWhenNeitherPopulated(t *testing.T) {
	out := &secretsmanager.GetSecretValueOutput{}

	_, _, err := extractValue(out)

	if err == nil {
		t.Fatal("extractValue() error = nil, want an error when neither SecretString nor SecretBinary is set")
	}
}
