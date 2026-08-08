package awsssm

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

func TestBuildParametersPopulatesStringValue(t *testing.T) {
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	raw := []types.Parameter{
		{Name: aws.String("/app/name"), Type: types.ParameterTypeString, Value: aws.String("hello"), LastModifiedDate: &ts},
	}

	got := buildParameters(raw)

	if len(got) != 1 {
		t.Fatalf("buildParameters() len = %d, want 1", len(got))
	}
	p := got[0]
	if p.Name != "/app/name" || p.Type != TypeString || p.Value != "hello" || !p.LastModified.Equal(ts) {
		t.Errorf("got %+v", p)
	}
}

func TestBuildParametersPopulatesStringListValue(t *testing.T) {
	raw := []types.Parameter{
		{Name: aws.String("/app/list"), Type: types.ParameterTypeStringList, Value: aws.String("a,b,c")},
	}

	got := buildParameters(raw)

	if got[0].Value != "a,b,c" {
		t.Errorf("StringList value = %q, want %q", got[0].Value, "a,b,c")
	}
	if got[0].Type != TypeStringList {
		t.Errorf("Type = %q, want %q", got[0].Type, TypeStringList)
	}
}

func TestBuildParametersDiscardsSecureStringCiphertext(t *testing.T) {
	raw := []types.Parameter{
		{Name: aws.String("/app/secret"), Type: types.ParameterTypeSecureString, Value: aws.String("AQICAHh...ciphertext...")},
	}

	got := buildParameters(raw)

	if got[0].Type != TypeSecureString {
		t.Fatalf("Type = %q, want %q", got[0].Type, TypeSecureString)
	}
	if got[0].Value != "" {
		t.Errorf("SecureString Value = %q, want empty (ciphertext must never be surfaced)", got[0].Value)
	}
}

func TestBuildParametersHandlesNilLastModifiedDate(t *testing.T) {
	raw := []types.Parameter{
		{Name: aws.String("/app/x"), Type: types.ParameterTypeString, Value: aws.String("v"), LastModifiedDate: nil},
	}

	got := buildParameters(raw)

	if !got[0].LastModified.IsZero() {
		t.Errorf("LastModified = %v, want zero value when AWS returns nil", got[0].LastModified)
	}
}

func TestBuildParametersSortsByName(t *testing.T) {
	raw := []types.Parameter{
		{Name: aws.String("/z"), Type: types.ParameterTypeString, Value: aws.String("z")},
		{Name: aws.String("/a"), Type: types.ParameterTypeString, Value: aws.String("a")},
		{Name: aws.String("/m"), Type: types.ParameterTypeString, Value: aws.String("m")},
	}

	got := buildParameters(raw)

	want := []string{"/a", "/m", "/z"}
	for i, w := range want {
		if got[i].Name != w {
			t.Errorf("got[%d].Name = %q, want %q", i, got[i].Name, w)
		}
	}
}

func TestBuildParametersEmptyInput(t *testing.T) {
	got := buildParameters(nil)
	if len(got) != 0 {
		t.Errorf("buildParameters(nil) = %+v, want empty", got)
	}
}
