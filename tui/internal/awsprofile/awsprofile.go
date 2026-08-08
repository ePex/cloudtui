// Package awsprofile discovers AWS CLI profiles from the shared config and
// credentials files (~/.aws/config, ~/.aws/credentials, or their
// AWS_CONFIG_FILE/AWS_SHARED_CREDENTIALS_FILE overrides). It is read-only:
// it never resolves, validates, or refreshes actual credentials, and makes
// no network calls — classification is based purely on which fields are
// present in the config files.
package awsprofile

import (
	"os"

	"github.com/aws/aws-sdk-go-v2/config"
)

// AuthType classifies how a profile authenticates, based on which fields
// are present — not on whether those credentials actually work.
type AuthType string

const (
	AuthStaticKeys        AuthType = "static-keys"
	AuthSSO               AuthType = "sso"
	AuthAssumeRole        AuthType = "assume-role"
	AuthCredentialProcess AuthType = "credential-process"
	AuthUnknown           AuthType = "unknown"
)

// Profile is one discovered AWS CLI profile.
type Profile struct {
	Name     string
	Region   string
	AuthType AuthType
}

// configFiles resolves which shared config/credentials files to read:
// AWS_CONFIG_FILE/AWS_SHARED_CREDENTIALS_FILE if set, else the SDK's
// standard defaults. config.LoadSharedConfigProfile does not check these
// env vars itself when called directly (only the higher-level
// config.LoadDefaultConfig does), so this is resolved explicitly and
// shared between the section-name scan and the per-profile SDK load —
// both must agree on which files they're reading.
func configFiles() (configFile, credentialsFile string) {
	configFile = os.Getenv("AWS_CONFIG_FILE")
	if configFile == "" {
		configFile = config.DefaultSharedConfigFilename()
	}
	credentialsFile = os.Getenv("AWS_SHARED_CREDENTIALS_FILE")
	if credentialsFile == "" {
		credentialsFile = config.DefaultSharedCredentialsFilename()
	}
	return configFile, credentialsFile
}
