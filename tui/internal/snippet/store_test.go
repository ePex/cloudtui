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
	want := []Entry{{Name: "shared", IsDir: true, IsLink: true}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("List = %#v, want %#v (symlinked folder listed as a link, dangling link skipped)", got, want)
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
	want := Format(Snippet{JMSType: sn.JMSType, Body: FormatBody(sn.Body), Extra: sn.Extra})
	if string(data) != string(want) {
		t.Errorf("file content = %q, want %q", data, want)
	}
}

func TestSaveFormatsBodiesAndKeepsFrontMatter(t *testing.T) {
	root := t.TempDir()
	s := NewStore(root)
	tests := []struct {
		name      string
		body      string
		formatted string
	}{
		{
			name:      "json",
			body:      `{"orderId":42}`,
			formatted: "{\n  \"orderId\": 42\n}",
		},
		{
			name:      "xml",
			body:      `<order><id>42</id></order>`,
			formatted: "<order>\n  <id>42</id>\n</order>",
		},
		{
			name:      "plain text",
			body:      "hello world",
			formatted: "hello world",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name := tt.name + ".txt"
			sn := Snippet{
				JMSType: "OrderCreated",
				Body:    tt.body,
				Extra:   "# Team-owned metadata.\nauthor: payments\n",
			}
			if err := s.Save(name, sn, false); err != nil {
				t.Fatalf("Save: %v", err)
			}
			data, err := os.ReadFile(filepath.Join(root, name))
			if err != nil {
				t.Fatal(err)
			}
			want := string(Format(Snippet{JMSType: sn.JMSType, Body: tt.formatted, Extra: sn.Extra}))
			if string(data) != want {
				t.Errorf("file content = %q, want %q", data, want)
			}
			parsed, err := Parse(data)
			if err != nil {
				t.Fatalf("Parse saved snippet: %v", err)
			}
			if parsed.Body != tt.formatted || parsed.JMSType != sn.JMSType || !strings.Contains(parsed.Extra, "author: payments") || !strings.Contains(parsed.Extra, "# Team-owned metadata.") {
				t.Errorf("saved snippet lost formatted body or front matter: %#v", parsed)
			}
		})
	}
}

func TestSaveFormatsBodyWhenOverwriting(t *testing.T) {
	root := t.TempDir()
	s := NewStore(root)
	writeFile(t, root, "event.json", "old")
	if err := s.Save("event.json", Snippet{Body: `{"id":1}`}, true); err != nil {
		t.Fatalf("Save overwrite: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "event.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), "{\n  \"id\": 1\n}"; got != want {
		t.Errorf("overwritten body = %q, want %q", got, want)
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

// symlinkOrSkip creates link → target, skipping the test where symlinks
// aren't available (e.g. unprivileged Windows).
func symlinkOrSkip(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unsupported here: %v", err)
	}
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func TestMkDir(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "orders/created.json", "x")
	s := NewStore(root)

	if err := s.MkDir("eu/archive"); err != nil {
		t.Fatalf("MkDir nested: %v", err)
	}
	if info, err := os.Stat(filepath.Join(root, "eu", "archive")); err != nil || !info.IsDir() {
		t.Errorf("eu/archive not created as a folder: %v", err)
	}

	for _, name := range []string{"orders", "orders/created.json", "eu/archive"} {
		if err := s.MkDir(name); !errors.Is(err, ErrExists) {
			t.Errorf("MkDir(%q) err = %v, want ErrExists", name, err)
		}
	}
	for _, name := range []string{"", "../x", ".hidden", "a/"} {
		if err := s.MkDir(name); err == nil {
			t.Errorf("MkDir(%q): expected a validation error", name)
		}
	}
}

func TestMove(t *testing.T) {
	tests := []struct {
		name     string
		from, to string
		wantErr  string // "" = success; else a substring, or "exists" for ErrExists
		gone     string // must no longer exist after success
		present  string // must exist after success
	}{
		{name: "rename in place", from: "orders/created.json", to: "orders/renamed.json",
			gone: "orders/created.json", present: "orders/renamed.json"},
		{name: "move into a new subfolder", from: "orders/created.json", to: "eu/archive/created.json",
			gone: "orders/created.json", present: "eu/archive/created.json"},
		{name: "move a folder", from: "orders", to: "archive/orders",
			gone: "orders", present: "archive/orders/created.json"},
		{name: "target exists", from: "orders/created.json", to: "ping.txt", wantErr: "exists"},
		{name: "folder into itself", from: "orders", to: "orders/inner", wantErr: "into itself"},
		{name: "folder onto itself", from: "orders", to: "orders", wantErr: "into itself"},
		{name: "missing source", from: "nope.txt", to: "x.txt", wantErr: "nope.txt"},
		{name: "target outside the root", from: "ping.txt", to: "../escape.txt", wantErr: "inside the snippets folder"},
		{name: "source outside the root", from: "../x", to: "y", wantErr: "outside"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, root, "orders/created.json", "x")
			writeFile(t, root, "ping.txt", "p")

			err := NewStore(root).Move(tt.from, tt.to)
			switch {
			case tt.wantErr == "exists":
				if !errors.Is(err, ErrExists) {
					t.Fatalf("err = %v, want ErrExists", err)
				}
			case tt.wantErr != "":
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want one containing %q", err, tt.wantErr)
				}
			default:
				if err != nil {
					t.Fatalf("Move: %v", err)
				}
				if exists(filepath.Join(root, filepath.FromSlash(tt.gone))) {
					t.Errorf("%s still exists", tt.gone)
				}
				if !exists(filepath.Join(root, filepath.FromSlash(tt.present))) {
					t.Errorf("%s doesn't exist", tt.present)
				}
				return
			}
			// A failed move changes nothing.
			if !exists(filepath.Join(root, "orders", "created.json")) || !exists(filepath.Join(root, "ping.txt")) {
				t.Error("a failed Move changed the library")
			}
		})
	}
}

func TestDelete(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "orders/created.json", "x")
	s := NewStore(root)

	if err := s.Delete("orders"); err == nil || !strings.Contains(err.Error(), "folder") {
		t.Errorf("Delete(folder) err = %v, want a refusal", err)
	}
	if err := s.Delete("orders/created.json"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if exists(filepath.Join(root, "orders", "created.json")) {
		t.Error("snippet still exists")
	}
	if err := s.Delete("orders/created.json"); err == nil {
		t.Error("Delete of a missing snippet: expected an error")
	}
	if err := s.Delete("../x"); err == nil {
		t.Error("Delete outside the root: expected an error")
	}
}

func TestDeleteFolder(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "orders/eu/created.json", "x")
	writeFile(t, root, "ping.txt", "p")
	s := NewStore(root)

	if err := s.DeleteFolder(""); err == nil {
		t.Error("DeleteFolder(root): expected a refusal")
	}
	if err := s.DeleteFolder("ping.txt"); err == nil {
		t.Error("DeleteFolder(file): expected a refusal")
	}
	if err := s.DeleteFolder("orders"); err != nil {
		t.Fatalf("DeleteFolder: %v", err)
	}
	if exists(filepath.Join(root, "orders")) {
		t.Error("folder still exists")
	}
	if !exists(filepath.Join(root, "ping.txt")) {
		t.Error("DeleteFolder removed more than the folder")
	}
}

// TestDeleteFolderSymlinkRemovesOnlyTheLink guards a shared checkout
// linked into the library: deleting the link must never delete its
// target's contents.
func TestDeleteFolderSymlinkRemovesOnlyTheLink(t *testing.T) {
	root := t.TempDir()
	shared := t.TempDir()
	writeFile(t, shared, "team/order.json", "x")
	symlinkOrSkip(t, shared, filepath.Join(root, "shared"))

	if err := NewStore(root).DeleteFolder("shared"); err != nil {
		t.Fatalf("DeleteFolder(link): %v", err)
	}
	if exists(filepath.Join(root, "shared")) {
		t.Error("link still exists")
	}
	if !exists(filepath.Join(shared, "team", "order.json")) {
		t.Fatal("the linked folder's contents were deleted")
	}
}

func TestCount(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "orders/created.json", "x")
	writeFile(t, root, "orders/eu/a.json", "x")
	writeFile(t, root, "orders/eu/b.json", "x")
	writeFile(t, root, "orders/.hidden", "x")
	writeFile(t, root, "orders/.git/config", "x")
	if err := os.MkdirAll(filepath.Join(root, "orders", "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	s := NewStore(root)

	snippets, folders, err := s.Count("orders")
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if snippets != 3 || folders != 2 {
		t.Errorf("Count(orders) = %d snippets, %d folders; want 3, 2", snippets, folders)
	}
	if _, _, err := s.Count("nope"); err == nil {
		t.Error("Count of a missing folder: expected an error")
	}
}

func TestCountDoesNotFollowLinkedFolders(t *testing.T) {
	root := t.TempDir()
	shared := t.TempDir()
	writeFile(t, shared, "a.json", "x")
	writeFile(t, shared, "b.json", "x")
	writeFile(t, root, "orders/created.json", "x")
	symlinkOrSkip(t, shared, filepath.Join(root, "orders", "shared"))

	snippets, folders, err := NewStore(root).Count("orders")
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if snippets != 1 || folders != 1 {
		t.Errorf("Count = %d snippets, %d folders; want 1, 1 (the link counts, its contents don't)", snippets, folders)
	}
}

func TestStat(t *testing.T) {
	root := t.TempDir()
	shared := t.TempDir()
	writeFile(t, root, "orders/created.json", "x")
	symlinkOrSkip(t, shared, filepath.Join(root, "shared"))
	s := NewStore(root)

	tests := []struct {
		name string
		want Entry
	}{
		{"orders", Entry{Name: "orders", IsDir: true}},
		{"orders/created.json", Entry{Name: "created.json"}},
		{"shared", Entry{Name: "shared", IsDir: true, IsLink: true}},
	}
	for _, tt := range tests {
		got, err := s.Stat(tt.name)
		if err != nil {
			t.Errorf("Stat(%q): %v", tt.name, err)
			continue
		}
		if got != tt.want {
			t.Errorf("Stat(%q) = %#v, want %#v", tt.name, got, tt.want)
		}
	}
	if _, err := s.Stat("missing"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Stat(missing) err = %v, want ErrNotExist", err)
	}
}

func TestNewOperationsOnUnavailableStore(t *testing.T) {
	s := NewStore("")
	checks := map[string]error{
		"MkDir":        s.MkDir("x"),
		"Move":         s.Move("a", "b"),
		"Delete":       s.Delete("x"),
		"DeleteFolder": s.DeleteFolder("x"),
	}
	_, _, checks["Count"] = s.Count("")
	_, checks["Stat"] = s.Stat("x")
	for op, err := range checks {
		if !errors.Is(err, errUnavailable) {
			t.Errorf("%s err = %v, want errUnavailable", op, err)
		}
	}
}

// TestExampleSnippets loads every file in the repo's examples/snippets/
// folder through a Store, the way the app would after a user copies the
// folder into ~/.cloudtui/snippets/, so a broken example fails CI. It
// also checks a few examples' contents and that saving each one without
// edits wouldn't change it.
func TestExampleSnippets(t *testing.T) {
	root := filepath.Join("..", "..", "..", "examples", "snippets")
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("examples folder missing: %v", err)
	}
	s := NewStore(root)

	loaded := map[string]Snippet{}
	var walk func(dir string)
	walk = func(dir string) {
		entries, err := s.List(dir)
		if err != nil {
			t.Fatalf("List(%q): %v", dir, err)
		}
		for _, e := range entries {
			rel := filepath.Join(dir, e.Name)
			if e.IsDir {
				walk(rel)
				continue
			}
			sn, err := s.Load(rel)
			if err != nil {
				t.Errorf("example %s doesn't parse: %v", filepath.ToSlash(rel), err)
				continue
			}
			if again, err := Parse(Format(sn)); err != nil || again != sn {
				t.Errorf("example %s changes when saved unedited: %#v, %v", filepath.ToSlash(rel), again, err)
			}
			loaded[filepath.ToSlash(rel)] = sn
		}
	}
	walk("")

	checks := []struct {
		path, jmsType string
		extra         bool
	}{
		{"orders/order-created.json", "OrderCreated", false},
		{"orders/order-cancelled.json", "OrderCancelled", false},
		{"payments/payment-received.xml", "PaymentReceived", false},
		{"inventory/stock-updated.json", "StockUpdated", true},
		{"ping.txt", "", false},
	}
	for _, c := range checks {
		sn, ok := loaded[c.path]
		if !ok {
			t.Errorf("example %s missing", c.path)
			continue
		}
		if sn.JMSType != c.jmsType || (sn.Extra != "") != c.extra || sn.Body == "" {
			t.Errorf("example %s = (JMSType %q, has Extra %v, body %d bytes), want (%q, %v, non-empty)",
				c.path, sn.JMSType, sn.Extra != "", len(sn.Body), c.jmsType, c.extra)
		}
	}
	if len(loaded) != len(checks) {
		t.Errorf("examples folder holds %d snippets, the test knows %d — add new ones to the checks", len(loaded), len(checks))
	}
}
