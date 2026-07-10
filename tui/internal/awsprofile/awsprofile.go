// Package awsprofile discovers the AWS profile names available on the
// local machine by reading the user's shared config and credentials
// files — no AWS API calls, no aws-sdk-go-v2 dependency.
package awsprofile

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// List returns the AWS profile names found in the user's shared config
// and credentials files (~/.aws/config, ~/.aws/credentials, or their
// AWS_CONFIG_FILE/AWS_SHARED_CREDENTIALS_FILE overrides), merged and
// de-duplicated. Missing files contribute nothing — not an error.
func List() ([]string, error) {
	return ListFrom(configFilePath(), credentialsFilePath())
}

// ListFrom is List's injectable core, for testing against fixture files
// instead of the real ~/.aws.
func ListFrom(configPath, credentialsPath string) ([]string, error) {
	names := map[string]struct{}{}

	if err := scanConfigProfiles(configPath, names); err != nil {
		return nil, err
	}
	if err := scanCredentialsProfiles(credentialsPath, names); err != nil {
		return nil, err
	}

	out := make([]string, 0, len(names))
	for n := range names {
		out = append(out, n)
	}
	sort.Strings(out)
	return out, nil
}

// scanConfigProfiles adds profile names from a shared config file (where
// section headers are "[default]" or "[profile <name>]"; anything else,
// e.g. "[sso-session ...]", isn't a profile and is skipped).
func scanConfigProfiles(path string, names map[string]struct{}) error {
	return scanSections(path, func(header string) {
		if header == "default" {
			names["default"] = struct{}{}
			return
		}
		if rest, ok := strings.CutPrefix(header, "profile "); ok {
			if name := strings.TrimSpace(rest); name != "" {
				names[name] = struct{}{}
			}
		}
	})
}

// scanCredentialsProfiles adds profile names from a shared credentials
// file, where every section header (including "[default]") is a profile
// name directly.
func scanCredentialsProfiles(path string, names map[string]struct{}) error {
	return scanSections(path, func(header string) {
		if header != "" {
			names[header] = struct{}{}
		}
	})
}

// scanSections reads path line by line, calling onSection with the
// trimmed contents of each "[...]" section header. A missing file
// contributes nothing and isn't an error.
func scanSections(path string, onSection func(header string)) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading %s: %w", path, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "[") || !strings.HasSuffix(line, "]") {
			continue
		}
		header := strings.TrimSpace(line[1 : len(line)-1])
		onSection(header)
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}
	return nil
}

func configFilePath() string {
	if p := os.Getenv("AWS_CONFIG_FILE"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".aws", "config")
}

func credentialsFilePath() string {
	if p := os.Getenv("AWS_SHARED_CREDENTIALS_FILE"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".aws", "credentials")
}
