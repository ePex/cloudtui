package snippet

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf8"
)

// MaxImportSize is the largest file ReadImportFile accepts: messages are
// small, and a huge file would make the editor and preview slow.
const MaxImportSize = 1 << 20 // 1 MiB

// errNotAbsolute is the hint shown for any path that isn't absolute.
var errNotAbsolute = errors.New("enter an absolute path")

// CleanImportPath turns a typed or pasted path into a clean absolute path
// for ReadImportFile. It trims surrounding spaces and one pair of
// surrounding quotes, unescapes "\ " to a space on macOS/Linux (as
// terminals paste a dragged-in file; on Windows a backslash is a path
// separator), and expands "~" and a leading "~/" to the home folder.
// Anything that isn't absolute afterwards is rejected, since it's unclear
// what it would be relative to.
func CleanImportPath(input string) (string, error) {
	path, err := cleanImportPath(input, runtime.GOOS, os.UserHomeDir)
	if err != nil {
		return "", err
	}
	return filepath.Clean(path), nil
}

// cleanImportPath is CleanImportPath for a given OS and home lookup, so
// both the Windows and the Unix rules can be tested on any OS. It doesn't
// Clean the result: filepath.Clean follows the running OS's rules.
func cleanImportPath(input, goos string, home func() (string, error)) (string, error) {
	path := strings.TrimSpace(input)
	if len(path) >= 2 {
		if first, last := path[0], path[len(path)-1]; first == last && (first == '"' || first == '\'') {
			path = strings.TrimSpace(path[1 : len(path)-1])
		}
	}
	windows := goos == "windows"
	if !windows {
		path = strings.ReplaceAll(path, `\ `, " ")
	}
	if path == "~" || strings.HasPrefix(path, "~/") || (windows && strings.HasPrefix(path, `~\`)) {
		dir, err := home()
		if err != nil {
			return "", fmt.Errorf("resolving ~: %w", err)
		}
		path = dir + path[1:]
	}
	if path == "" {
		return "", errNotAbsolute
	}
	if !isAbsFor(path, windows) {
		return "", fmt.Errorf("%q is not an absolute path: %w", strings.TrimSpace(input), errNotAbsolute)
	}
	return path, nil
}

// isAbsFor reports whether path is absolute by the rules of Windows
// (a drive letter followed by a separator, or a UNC \\server\share path)
// or of Unix (a leading "/").
func isAbsFor(path string, windows bool) bool {
	if !windows {
		return strings.HasPrefix(path, "/")
	}
	if strings.HasPrefix(path, `\\`) || strings.HasPrefix(path, "//") {
		return true
	}
	return len(path) >= 3 && isDriveLetter(path[0]) && path[1] == ':' && (path[2] == '\\' || path[2] == '/')
}

func isDriveLetter(c byte) bool {
	return ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z')
}

// ReadImportFile reads the file at path (absolute, as returned by
// CleanImportPath) as a snippet to import. It must be a regular file (a
// symlink to one is followed), at most MaxImportSize bytes, text (valid
// UTF-8 without NUL bytes), and — if it starts with "---" — have a valid
// front matter. Errors name the file. The file is only read.
func ReadImportFile(path string) (Snippet, error) {
	info, err := os.Stat(path)
	if err != nil {
		return Snippet{}, fmt.Errorf("%q: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return Snippet{}, fmt.Errorf("%q is not a file", path)
	}
	if info.Size() > MaxImportSize {
		return Snippet{}, fmt.Errorf("%q is larger than 1 MiB", path)
	}

	f, err := os.Open(path)
	if err != nil {
		return Snippet{}, fmt.Errorf("%q: %w", path, err)
	}
	defer f.Close()
	// Read one byte past the limit, in case the file grew since Stat.
	data, err := io.ReadAll(io.LimitReader(f, MaxImportSize+1))
	if err != nil {
		return Snippet{}, fmt.Errorf("reading %q: %w", path, err)
	}
	if len(data) > MaxImportSize {
		return Snippet{}, fmt.Errorf("%q is larger than 1 MiB", path)
	}
	if bytes.IndexByte(data, 0) >= 0 || !utf8.Valid(data) {
		return Snippet{}, fmt.Errorf("%q is not a text file", path)
	}

	sn, err := Parse(data)
	if err != nil {
		return Snippet{}, fmt.Errorf("%q: %w", path, err)
	}
	return sn, nil
}
