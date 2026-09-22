package snippet

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// writeFile creates root/rel (with parent folders) holding content.
func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDefaultRoot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	got, err := DefaultRoot()
	if err != nil {
		t.Fatalf("DefaultRoot: %v", err)
	}
	if want := filepath.Join(home, ".cloudtui", "snippets"); got != want {
		t.Errorf("DefaultRoot = %q, want %q", got, want)
	}
}

func TestListOrderingAndFiltering(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "zeta.txt", "")
	writeFile(t, root, "Alpha.json", "")
	writeFile(t, root, "beta", "")
	writeFile(t, root, ".hidden", "")
	writeFile(t, root, "orders/created.json", "")
	writeFile(t, root, "Archive/old.txt", "")
	writeFile(t, root, ".git/config", "")

	got, err := NewStore(root).List("")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	want := []Entry{
		{Name: "Archive", IsDir: true},
		{Name: "orders", IsDir: true},
		{Name: "Alpha.json"},
		{Name: "beta"},
		{Name: "zeta.txt"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("List(\"\") = %#v, want %#v", got, want)
	}
}

func TestListSubfolder(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "orders/created.json", "")
	writeFile(t, root, "orders/eu/updated.json", "")

	got, err := NewStore(root).List("orders")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	want := []Entry{{Name: "eu", IsDir: true}, {Name: "created.json"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("List(\"orders\") = %#v, want %#v", got, want)
	}
}

func TestListFollowsSymlinks(t *testing.T) {
	root := t.TempDir()
	shared := t.TempDir()
	writeFile(t, shared, "team.json", "")
	if err := os.Symlink(shared, filepath.Join(root, "shared")); err != nil {
		t.Skipf("symlinks unsupported here: %v", err)
	}
	if err := os.Symlink(filepath.Join(root, "missing"), filepath.Join(root, "dangling")); err != nil {
		t.Fatal(err)
	}

	got, err := NewStore(root).List("")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	want := []Entry{{Name: "shared", IsDir: true}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("List = %#v, want %#v (symlinked folder listed, dangling link skipped)", got, want)
	}
}

func TestListMissingRootIsEmpty(t *testing.T) {
	root := filepath.Join(t.TempDir(), "does-not-exist")
	got, err := NewStore(root).List("")
	if err != nil {
		t.Fatalf("List: unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("List = %#v, want empty", got)
	}
}

func TestListMissingSubfolderIsError(t *testing.T) {
	if _, err := NewStore(t.TempDir()).List("nope"); err == nil {
		t.Error("List(\"nope\"): expected error, got nil")
	}
}

func TestLoad(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "orders/created.json", "---\njmsType: OrderCreated\n---\n{}")
	writeFile(t, root, "broken", "---\njmsType: T\n")

	s := NewStore(root)
	got, err := s.Load("orders/created.json")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if want := (Snippet{JMSType: "OrderCreated", Body: "{}"}); got != want {
		t.Errorf("Load = %#v, want %#v", got, want)
	}

	if _, err := s.Load("broken"); err == nil || !strings.Contains(err.Error(), "broken") {
		t.Errorf("Load(broken) error = %v, want a parse error naming the snippet", err)
	}
	if _, err := s.Load("missing"); err == nil {
		t.Error("Load(missing): expected error, got nil")
	}
}

func TestLoadAndListRefuseNonLocalPaths(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(filepath.Dir(root), "outside.txt")
	s := NewStore(root)
	for _, rel := range []string{"../outside.txt", outside, "a/../../outside.txt"} {
		if _, err := s.Load(rel); err == nil || !strings.Contains(err.Error(), "outside") {
			t.Errorf("Load(%q) error = %v, want an outside-the-folder error", rel, err)
		}
		if _, err := s.List(rel); err == nil || !strings.Contains(err.Error(), "outside") {
			t.Errorf("List(%q) error = %v, want an outside-the-folder error", rel, err)
		}
	}
	if _, err := s.Load(""); err == nil {
		t.Error("Load(\"\"): expected error, got nil")
	}
}

func TestSaveCreatesFoldersAndWritesFormat(t *testing.T) {
	root := filepath.Join(t.TempDir(), "snippets") // root itself doesn't exist yet
	s := NewStore(root)
	sn := Snippet{JMSType: "OrderCreated", Body: `{"orderId": 42}`}
	if err := s.Save("orders/eu/created.json", sn, false); err != nil {
		t.Fatalf("Save: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, "orders", "eu", "created.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(Format(sn)) {
		t.Errorf("file content = %q, want %q", data, Format(sn))
	}
}

func TestSaveExistsAndOverwrite(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "ping.txt", "original")
	s := NewStore(root)

	err := s.Save("ping.txt", Snippet{Body: "new"}, false)
	if !errors.Is(err, ErrExists) {
		t.Fatalf("Save without overwrite: err = %v, want ErrExists", err)
	}
	if data, _ := os.ReadFile(filepath.Join(root, "ping.txt")); string(data) != "original" {
		t.Errorf("file changed to %q despite ErrExists", data)
	}

	if err := s.Save("ping.txt", Snippet{Body: "new"}, true); err != nil {
		t.Fatalf("Save with overwrite: %v", err)
	}
	if data, _ := os.ReadFile(filepath.Join(root, "ping.txt")); string(data) != "new" {
		t.Errorf("file content = %q, want %q (fully replaced, not appended)", data, "new")
	}
}

func TestSaveRejectsInvalidName(t *testing.T) {
	root := t.TempDir()
	if err := NewStore(root).Save("../escape.txt", Snippet{Body: "x"}, false); err == nil {
		t.Fatal("Save(../escape.txt): expected error, got nil")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(root), "escape.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("file was written outside the root: %v", err)
	}
}

func TestUnavailableStore(t *testing.T) {
	s := NewStore("")
	if _, err := s.List(""); !errors.Is(err, errUnavailable) {
		t.Errorf("List err = %v, want errUnavailable", err)
	}
	if _, err := s.Load("x"); !errors.Is(err, errUnavailable) {
		t.Errorf("Load err = %v, want errUnavailable", err)
	}
	if err := s.Save("x", Snippet{}, false); !errors.Is(err, errUnavailable) {
		t.Errorf("Save err = %v, want errUnavailable", err)
	}
}

func TestValidateName(t *testing.T) {
	valid := []struct {
		in   string
		want string
	}{
		{"created.json", "created.json"},
		{"  padded.txt  ", "padded.txt"},
		{"no-extension", "no-extension"},
		{"orders/created.json", filepath.Join("orders", "created.json")},
		{"orders/eu/created.json", filepath.Join("orders", "eu", "created.json")},
		{"orders//created.json", filepath.Join("orders", "created.json")},
		{"./created.json", "created.json"},
		{"file.with.dots", "file.with.dots"},
	}
	for _, tt := range valid {
		got, err := ValidateName(tt.in)
		if err != nil {
			t.Errorf("ValidateName(%q): unexpected error: %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ValidateName(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}

	invalid := []string{
		"",
		"   ",
		"orders/",
		"/abs.txt",
		"../escape",
		"orders/../../escape",
		"..",
		".",
		".hidden",
		"orders/.hidden",
		".git/config",
	}
	for _, in := range invalid {
		if got, err := ValidateName(in); err == nil {
			t.Errorf("ValidateName(%q) = %q, want error", in, got)
		}
	}
}
