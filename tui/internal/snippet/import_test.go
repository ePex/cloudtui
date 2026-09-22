package snippet

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCleanImportPathUnix(t *testing.T) {
	home := func() (string, error) { return "/Users/me", nil }
	tests := []struct {
		name, in, want, wantErr string
	}{
		{name: "absolute", in: "/tmp/order.json", want: "/tmp/order.json"},
		{name: "surrounding spaces", in: "  /tmp/order.json \n", want: "/tmp/order.json"},
		{name: "double quotes", in: `"/tmp/my order.json"`, want: "/tmp/my order.json"},
		{name: "single quotes", in: `'/tmp/my order.json'`, want: "/tmp/my order.json"},
		{name: "escaped spaces from a terminal drag", in: `/tmp/my\ big\ order.json`, want: "/tmp/my big order.json"},
		{name: "tilde", in: "~", want: "/Users/me"},
		{name: "tilde slash", in: "~/Downloads/order.json", want: "/Users/me/Downloads/order.json"},
		{name: "quoted tilde", in: `"~/Downloads/order.json"`, want: "/Users/me/Downloads/order.json"},
		{name: "relative", in: "order.json", wantErr: "absolute path"},
		{name: "dot relative", in: "./order.json", wantErr: "absolute path"},
		{name: "tilde user isn't expanded", in: "~other/order.json", wantErr: "absolute path"},
		{name: "empty", in: "   ", wantErr: "absolute path"},
		{name: "empty quotes", in: `""`, wantErr: "absolute path"},
		{name: "a Windows path isn't absolute on Unix", in: `C:\data\order.json`, wantErr: "absolute path"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := cleanImportPath(tt.in, "darwin", home)
			checkCleanImportPath(t, got, err, tt.want, tt.wantErr)
		})
	}
}

func TestCleanImportPathWindows(t *testing.T) {
	home := func() (string, error) { return `C:\Users\me`, nil }
	tests := []struct {
		name, in, want, wantErr string
	}{
		{name: "drive path", in: `C:\data\order.json`, want: `C:\data\order.json`},
		{name: "drive path with slashes", in: "c:/data/order.json", want: "c:/data/order.json"},
		{name: "UNC path", in: `\\server\share\order.json`, want: `\\server\share\order.json`},
		{name: "quoted with spaces", in: `"C:\my data\order.json"`, want: `C:\my data\order.json`},
		{name: "backslashes are separators, not escapes", in: `C:\data\ order.json`, want: `C:\data\ order.json`},
		{name: "tilde backslash", in: `~\Downloads\order.json`, want: `C:\Users\me\Downloads\order.json`},
		{name: "tilde slash", in: "~/Downloads/order.json", want: `C:\Users\me/Downloads/order.json`},
		{name: "relative", in: `data\order.json`, wantErr: "absolute path"},
		{name: "drive-relative", in: `C:order.json`, wantErr: "absolute path"},
		{name: "rooted without a drive", in: `\data\order.json`, wantErr: "absolute path"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := cleanImportPath(tt.in, "windows", home)
			checkCleanImportPath(t, got, err, tt.want, tt.wantErr)
		})
	}
}

func checkCleanImportPath(t *testing.T, got string, err error, want, wantErr string) {
	t.Helper()
	if wantErr != "" {
		if err == nil || !strings.Contains(err.Error(), wantErr) {
			t.Errorf("got (%q, %v), want an error containing %q", got, err, wantErr)
		}
		return
	}
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestCleanImportPathHomeError(t *testing.T) {
	_, err := cleanImportPath("~/x", "linux", func() (string, error) { return "", errors.New("no home") })
	if err == nil || !strings.Contains(err.Error(), "no home") {
		t.Errorf("err = %v, want the home lookup error", err)
	}
}

// TestCleanImportPathCleans checks the public function also cleans the
// path by the running OS's rules.
func TestCleanImportPathCleans(t *testing.T) {
	dir := t.TempDir()
	sep := string(filepath.Separator)
	messy := dir + sep + "sub" + sep + ".." + sep + "." + sep + "order.json"
	got, err := CleanImportPath(messy)
	if err != nil {
		t.Fatalf("CleanImportPath: %v", err)
	}
	if want := filepath.Join(dir, "order.json"); got != want {
		t.Errorf("CleanImportPath = %q, want %q", got, want)
	}
}

// writeImportFile writes content to dir/name and returns its path.
func writeImportFile(t *testing.T, dir, name string, content []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestReadImportFile(t *testing.T) {
	dir := t.TempDir()
	tests := []struct {
		name    string
		content []byte
		want    Snippet
	}{
		{
			name:    "plain JSON",
			content: []byte(`{"orderId":42}`),
			want:    Snippet{Body: `{"orderId":42}`},
		},
		{
			name:    "a teammate's snippet file keeps its JMS Type and extra keys",
			content: []byte("---\n# shared\nauthor: someone\njmsType: OrderCreated\n---\n{}"),
			want:    Snippet{JMSType: "OrderCreated", Body: "{}", Extra: "# shared\nauthor: someone\njmsType: OrderCreated\n"},
		},
		{
			name:    "exactly 1 MiB",
			content: bytes.Repeat([]byte("a"), MaxImportSize),
			want:    Snippet{Body: strings.Repeat("a", MaxImportSize)},
		},
	}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeImportFile(t, dir, "f"+string(rune('0'+i)), tt.content)
			got, err := ReadImportFile(path)
			if err != nil {
				t.Fatalf("ReadImportFile: %v", err)
			}
			if got != tt.want {
				t.Errorf("ReadImportFile = %#v, want %#v", got, tt.want)
			}
			if after, _ := os.ReadFile(path); !bytes.Equal(after, tt.content) {
				t.Error("the source file was changed")
			}
		})
	}
}

func TestReadImportFileRefusals(t *testing.T) {
	dir := t.TempDir()
	tests := []struct {
		name    string
		path    func() string
		wantErr string
	}{
		{"missing", func() string { return filepath.Join(dir, "missing.json") }, "missing.json"},
		{"folder", func() string {
			sub := filepath.Join(dir, "folder")
			if err := os.Mkdir(sub, 0o755); err != nil {
				t.Fatal(err)
			}
			return sub
		}, "not a file"},
		{"1 MiB + 1", func() string {
			return writeImportFile(t, dir, "big.json", bytes.Repeat([]byte("a"), MaxImportSize+1))
		}, "larger than 1 MiB"},
		{"NUL byte", func() string { return writeImportFile(t, dir, "nul.bin", []byte("ab\x00cd")) }, "not a text file"},
		{"invalid UTF-8", func() string { return writeImportFile(t, dir, "latin1.txt", []byte{'c', 'a', 'f', 0xe9}) }, "not a text file"},
		{"broken front matter", func() string {
			return writeImportFile(t, dir, "broken.json", []byte("---\njmsType: T\n{}"))
		}, "no closing"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.path()
			_, err := ReadImportFile(path)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("err = %v, want one containing %q", err, tt.wantErr)
			}
			if err != nil && !strings.Contains(err.Error(), filepath.Base(path)) {
				t.Errorf("error %q doesn't name the file", err)
			}
		})
	}
}

func TestReadImportFileFollowsSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need extra privileges on Windows")
	}
	dir := t.TempDir()
	target := writeImportFile(t, dir, "order.json", []byte(`{"id":1}`))
	link := filepath.Join(dir, "link.json")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unsupported here: %v", err)
	}
	got, err := ReadImportFile(link)
	if err != nil || got.Body != `{"id":1}` {
		t.Errorf("ReadImportFile(link) = (%#v, %v), want the target's content", got, err)
	}
}
