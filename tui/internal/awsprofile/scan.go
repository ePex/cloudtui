package awsprofile

import (
	"bufio"
	"os"
	"strings"
)

// scanProfileNames extracts the set of profile names declared in an AWS
// shared config/credentials file, without interpreting any of their
// fields — that part is deliberately left to config.LoadSharedConfigProfile
// (see awsprofile.go's configFiles doc comment for why). The AWS SDK for Go
// v2 has no public API for this (only internal/ini, which cloudtui cannot
// import), so this is hand-rolled: it's the simple, stable part of the
// format (`[name]` or `[profile name]` section headers), not the part
// worth reimplementing.
//
// Both files use a bare [default] section for the default profile; only
// the config file ever prefixes named profiles with "profile " ([profile
// foo]), while the credentials file uses plain [foo] for every profile.
// Stripping an optional "profile " prefix handles both files with the same
// logic — it's simply absent in credentials-file sections.
// Missing files are not an error: most machines don't have both.
func scanProfileNames(path string) (map[string]bool, error) {
	names := map[string]bool{}

	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return names, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "[") || !strings.HasSuffix(line, "]") {
			continue
		}
		section := strings.TrimSpace(line[1 : len(line)-1])
		name := strings.TrimPrefix(section, "profile ")
		name = strings.TrimSpace(name)
		if name != "" {
			names[name] = true
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return names, nil
}
