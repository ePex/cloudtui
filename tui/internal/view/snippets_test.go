package view

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/dialog"
	"github.com/ePex/cloudtui/tui/internal/snippet"
)

// snippetsFixture is a SnippetsView over a temp library, with its dialogs.
type snippetsFixture struct {
	v       *SnippetsView
	host    *fakeViewHost
	root    string
	confirm *dialog.ConfirmDialog
	editor  *dialog.SnippetEditor
	prompt  *dialog.TextPrompt
}

// newSnippetsFixture builds the view over a library holding
// orders/created.json (JMS Type OrderCreated), orders/eu/updated.json,
// an empty folder, and ping.txt (no JMS Type).
func newSnippetsFixture(t *testing.T) *snippetsFixture {
	t.Helper()
	root := t.TempDir()
	write := func(rel, content string) {
		t.Helper()
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("orders/created.json", "---\njmsType: OrderCreated\n---\n{\"id\":1}")
	write("orders/eu/updated.json", "u")
	write("ping.txt", "ping")
	if err := os.MkdirAll(filepath.Join(root, "empty"), 0o755); err != nil {
		t.Fatal(err)
	}

	host := newFakeViewHost()
	store := snippet.NewStore(root)
	confirm := dialog.NewConfirmDialog(host)
	editor := dialog.NewSnippetEditor(host, store, confirm)
	prompt := dialog.NewTextPrompt(host)
	v := NewSnippetsView(host, store, confirm, editor, prompt)
	return &snippetsFixture{v: v, host: host, root: root, confirm: confirm, editor: editor, prompt: prompt}
}

// focusTree focuses p the way tview.Application does, recursing into
// whatever p delegates focus to.
func focusTree(p tview.Primitive) {
	var focus func(tview.Primitive)
	focus = func(p tview.Primitive) { p.Focus(focus) }
	focus(p)
}

// send delivers ev to p's input handler (which runs input captures
// first, as tview.Application does).
func send(p tview.Primitive, ev *tcell.EventKey) {
	var focus func(tview.Primitive)
	focus = func(p tview.Primitive) { p.Focus(focus) }
	p.InputHandler()(ev, focus)
}

func key(k tcell.Key) *tcell.EventKey { return tcell.NewEventKey(k, 0, tcell.ModNone) }
func runeKey(r rune) *tcell.EventKey  { return tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone) }

// press sends a rune key to the library list, as when the view has focus.
func (f *snippetsFixture) press(r rune) {
	focusTree(f.v.browser.List())
	send(f.v.browser.List(), runeKey(r))
}

// cursorTo puts the list cursor on the entry called name.
func (f *snippetsFixture) cursorTo(t *testing.T, name string) {
	t.Helper()
	if !f.v.browser.Select(name) {
		t.Fatalf("no entry %q in the current folder", name)
	}
}

// typeInto focuses the overlay prim, clears its focused field (Ctrl-U),
// types text, and presses Enter.
func typeInto(prim tview.Primitive, text string) {
	focusTree(prim)
	send(prim, key(tcell.KeyCtrlU))
	for _, r := range text {
		send(prim, runeKey(r))
	}
	send(prim, key(tcell.KeyEnter))
}

// answer picks No (0) or Yes (1) in the confirmation dialog.
func (f *snippetsFixture) answer(yes bool) {
	prim := f.confirm.Primitive()
	focusTree(prim)
	if yes {
		send(prim, key(tcell.KeyDown))
	}
	send(prim, key(tcell.KeyEnter))
}

func (f *snippetsFixture) exists(rel string) bool {
	_, err := os.Lstat(filepath.Join(f.root, filepath.FromSlash(rel)))
	return err == nil
}

func (f *snippetsFixture) previewText() string { return f.v.preview.GetText(true) }

func (f *snippetsFixture) selectedName() string {
	_, e, _ := f.v.browser.Selected()
	return e.Name
}

func TestSnippetsViewPreview(t *testing.T) {
	f := newSnippetsFixture(t)
	path := filepath.Join(f.root, "orders", "created.json")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	f.cursorTo(t, "ping.txt")
	if got := f.previewText(); !strings.Contains(got, "JMS Type: (none)") || !strings.Contains(got, "ping") {
		t.Errorf("preview of ping.txt = %q", got)
	}
	f.cursorTo(t, "orders")
	if got := f.previewText(); !strings.Contains(got, "2 snippets, 1 subfolder") {
		t.Errorf("preview of orders/ = %q, want the nested counts", got)
	}
	f.cursorTo(t, "empty")
	if got := f.previewText(); !strings.Contains(got, "Empty folder") {
		t.Errorf("preview of empty/ = %q", got)
	}

	f.v.browser.SetDir("orders")
	f.cursorTo(t, "created.json")
	if got := f.previewText(); !strings.Contains(got, "JMS Type: OrderCreated") || !strings.Contains(got, "{\n  \"id\": 1\n}") {
		t.Errorf("preview of created.json = %q", got)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != string(original) {
		t.Errorf("preview changed the file: content = %q, err = %v", got, err)
	}
}

func TestSnippetsViewPreviewFormatsXMLWithoutWriting(t *testing.T) {
	f := newSnippetsFixture(t)
	path := filepath.Join(f.root, "payment.xml")
	const body = `<payment><id>42</id></payment>`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	f.v.Activate()
	f.cursorTo(t, "payment.xml")
	if got := f.previewText(); !strings.Contains(got, "<payment>\n  <id>42</id>\n</payment>") {
		t.Errorf("XML preview = %q, want formatted XML", got)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != body {
		t.Errorf("preview changed the XML file: content = %q, err = %v", got, err)
	}
}

func TestSnippetsViewPreviewParseErrorAndLink(t *testing.T) {
	f := newSnippetsFixture(t)
	if err := os.WriteFile(filepath.Join(f.root, "broken.txt"), []byte("---\njmsType: T\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	f.v.Activate()
	f.cursorTo(t, "broken.txt")
	if got := f.previewText(); !strings.Contains(got, "no closing") {
		t.Errorf("preview of broken.txt = %q, want the parse error", got)
	}

	if err := os.Symlink(t.TempDir(), filepath.Join(f.root, "shared")); err != nil {
		t.Skipf("symlinks unsupported here: %v", err)
	}
	f.v.Activate()
	f.cursorTo(t, "shared")
	if got := f.previewText(); !strings.Contains(got, "Linked folder") {
		t.Errorf("preview of shared/ = %q", got)
	}
}

func TestSnippetsViewEmptyLibraryHint(t *testing.T) {
	host := newFakeViewHost()
	store := snippet.NewStore(filepath.Join(t.TempDir(), "snippets"))
	confirm := dialog.NewConfirmDialog(host)
	v := NewSnippetsView(host, store, confirm, dialog.NewSnippetEditor(host, store, confirm), dialog.NewTextPrompt(host))

	text := renderedScreenText(t, v.Primitive(), 120, 10)
	if !strings.Contains(text, "No snippets yet") || !strings.Contains(text, "press n to create one") {
		t.Errorf("empty library shows %q, want the hint mentioning n", text)
	}
}

func TestSnippetsViewEntryKeysDoNothingOnUpRow(t *testing.T) {
	f := newSnippetsFixture(t)
	f.v.browser.SetDir("orders")
	f.v.browser.List().SetCurrentItem(0) // ".."
	for _, r := range "eRd" {
		f.press(r)
	}
	if f.editor.Visible() || f.prompt.Visible() || f.confirm.Visible() {
		t.Errorf("a dialog opened on \"..\": editor %v, prompt %v, confirm %v", f.editor.Visible(), f.prompt.Visible(), f.confirm.Visible())
	}
}

func TestSnippetsViewEditOpensOnlyForSnippets(t *testing.T) {
	f := newSnippetsFixture(t)
	f.cursorTo(t, "orders")
	f.press('e')
	if f.editor.Visible() || f.host.status != "" {
		t.Errorf("e on a folder: editor visible %v, status %q; want nothing to happen", f.editor.Visible(), f.host.status)
	}

	f.cursorTo(t, "ping.txt")
	f.press('e')
	if !f.editor.Visible() {
		t.Error("e on a snippet didn't open the editor")
	}
}

func TestSnippetsViewEnterOnSnippetEdits(t *testing.T) {
	f := newSnippetsFixture(t)
	f.cursorTo(t, "ping.txt")
	send(f.v.browser.List(), key(tcell.KeyEnter))
	if !f.editor.Visible() {
		t.Error("Enter on a snippet didn't open the editor")
	}
}

func TestSnippetsViewEditParseErrorReported(t *testing.T) {
	f := newSnippetsFixture(t)
	if err := os.WriteFile(filepath.Join(f.root, "broken.txt"), []byte("---\njmsType: T\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	f.v.Activate()
	f.cursorTo(t, "broken.txt")
	f.press('e')
	if f.editor.Visible() || !strings.Contains(f.host.status, "no closing") {
		t.Errorf("editor visible = %v, status = %q; want the parse error instead", f.editor.Visible(), f.host.status)
	}
}

func TestSnippetsViewEditFormatsAndPersistsBody(t *testing.T) {
	f := newSnippetsFixture(t)
	f.v.browser.SetDir("orders")
	f.cursorTo(t, "created.json")
	f.press('e')
	if !f.editor.Visible() {
		t.Fatal("editor didn't open")
	}
	data, err := os.ReadFile(filepath.Join(f.root, "orders", "created.json"))
	if err != nil {
		t.Fatal(err)
	}
	if want := "---\njmsType: OrderCreated\n---\n{\n  \"id\": 1\n}"; string(data) != want {
		t.Errorf("edited snippet on disk = %q (%d bytes), want %q (%d bytes)", data, len(data), want, len(want))
	}
}

func TestSnippetsViewEditFormatWriteFailureKeepsEditorClosed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("read-only file permissions are not enforced for this test process")
	}
	f := newSnippetsFixture(t)
	path := filepath.Join(f.root, "orders", "created.json")
	if err := os.Chmod(path, 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })
	probe, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err == nil {
		_ = probe.Close()
		t.Skip("read-only file permissions are not enforced for this test process")
	}
	f.v.browser.SetDir("orders")
	f.cursorTo(t, "created.json")
	f.press('e')
	if f.editor.Visible() || !strings.Contains(f.host.status, "saving snippet") {
		t.Errorf("editor visible = %v, status = %q; want write error and closed editor", f.editor.Visible(), f.host.status)
	}
}

func TestSnippetsViewNewSnippetLandsOnIt(t *testing.T) {
	f := newSnippetsFixture(t)
	f.v.browser.SetDir("orders")
	f.press('n')
	if !f.editor.Visible() {
		t.Fatal("n didn't open the editor")
	}

	typeInto(f.editor.Primitive(), "fresh.txt")

	if !f.exists("orders/fresh.txt") {
		t.Fatal("orders/fresh.txt not created")
	}
	if f.v.browser.Dir() != "orders" || f.selectedName() != "fresh.txt" {
		t.Errorf("view at %q on %q, want orders on fresh.txt", f.v.browser.Dir(), f.selectedName())
	}
	if f.host.focused != f.v.browser.List() {
		t.Error("focus not returned to the list")
	}
}

func TestSnippetsViewNewFolderLandsOnIt(t *testing.T) {
	f := newSnippetsFixture(t)
	f.press('N')
	if !f.prompt.Visible() {
		t.Fatal("N didn't open the prompt")
	}

	typeInto(f.prompt.Primitive(), "eu/archive")

	if info, err := os.Stat(filepath.Join(f.root, "eu", "archive")); err != nil || !info.IsDir() {
		t.Fatalf("eu/archive not created: %v", err)
	}
	if f.prompt.Visible() || f.v.browser.Dir() != "eu" || f.selectedName() != "archive" {
		t.Errorf("prompt visible %v, view at %q on %q; want closed, eu, archive", f.prompt.Visible(), f.v.browser.Dir(), f.selectedName())
	}
}

func TestSnippetsViewNewFolderExistingKeepsPromptOpen(t *testing.T) {
	f := newSnippetsFixture(t)
	f.press('N')
	typeInto(f.prompt.Primitive(), "orders")
	if !f.prompt.Visible() || !strings.Contains(f.host.status, "already exists") {
		t.Errorf("prompt visible %v, status %q; want it open with an error", f.prompt.Visible(), f.host.status)
	}
}

func TestSnippetsViewNewFolderRejectsDotDot(t *testing.T) {
	f := newSnippetsFixture(t)
	f.v.browser.SetDir("orders")
	f.press('N')
	typeInto(f.prompt.Primitive(), "../escaped")
	if f.exists("escaped") || !f.prompt.Visible() {
		t.Errorf("created %v, prompt visible %v; want \"..\" refused as typed", f.exists("escaped"), f.prompt.Visible())
	}
}

func TestSnippetsViewRename(t *testing.T) {
	f := newSnippetsFixture(t)
	f.v.browser.SetDir("orders")
	f.cursorTo(t, "created.json")
	f.press('R')
	if !f.prompt.Visible() {
		t.Fatal("R didn't open the prompt")
	}
	if text := renderedScreenText(t, f.prompt.Primitive(), 64, 8); !strings.Contains(text, "orders/created.json") {
		t.Errorf("prompt isn't prefilled with the current path: %q", text)
	}

	typeInto(f.prompt.Primitive(), "archive/c.json")

	if f.exists("orders/created.json") || !f.exists("archive/c.json") {
		t.Fatalf("move didn't happen: old %v, new %v", f.exists("orders/created.json"), f.exists("archive/c.json"))
	}
	if f.v.browser.Dir() != "archive" || f.selectedName() != "c.json" {
		t.Errorf("view at %q on %q, want archive on c.json", f.v.browser.Dir(), f.selectedName())
	}
}

func TestSnippetsViewRenameRefusals(t *testing.T) {
	for _, tt := range []struct {
		name, entry, to, wantErr string
	}{
		{"onto an existing snippet", "ping.txt", "orders/created.json", "already exists"},
		{"a folder into itself", "orders", "orders/inner/orders", "into itself"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newSnippetsFixture(t)
			f.cursorTo(t, tt.entry)
			f.press('R')
			typeInto(f.prompt.Primitive(), tt.to)

			if !f.prompt.Visible() || !strings.Contains(f.host.status, tt.wantErr) {
				t.Errorf("prompt visible %v, status %q; want it open with %q", f.prompt.Visible(), f.host.status, tt.wantErr)
			}
			if !f.exists(tt.entry) {
				t.Errorf("%s was moved anyway", tt.entry)
			}
		})
	}
}

func TestSnippetsViewDeleteQuestions(t *testing.T) {
	f := newSnippetsFixture(t)
	linked := os.Symlink(t.TempDir(), filepath.Join(f.root, "shared")) == nil
	f.v.Activate()

	tests := []struct{ entry, want string }{
		{"ping.txt", `Delete snippet "ping.txt"?`},
		{"orders", `Delete folder "orders" and its 2 snippets, 1 subfolder?`},
		{"empty", `Delete empty folder "empty"?`},
	}
	if linked {
		tests = append(tests, struct{ entry, want string }{"shared", `Remove link "shared"? The folder it points to is kept.`})
	}
	for _, tt := range tests {
		f.cursorTo(t, tt.entry)
		f.press('d')
		if !f.confirm.Visible() {
			t.Fatalf("d on %s didn't ask", tt.entry)
		}
		if text := renderedScreenText(t, f.confirm.Primitive(), 90, 8); !strings.Contains(text, tt.want) {
			t.Errorf("d on %s asks %q, want %q", tt.entry, text, tt.want)
		}
		f.answer(false)
		if !f.exists(tt.entry) {
			t.Errorf("No deleted %s", tt.entry)
		}
	}
}

func TestSnippetsViewDeleteYes(t *testing.T) {
	f := newSnippetsFixture(t)
	f.cursorTo(t, "orders")
	f.press('d')
	f.answer(true)

	if f.exists("orders") {
		t.Error("folder still exists after Yes")
	}
	if !f.exists("ping.txt") {
		t.Error("deleted more than the folder")
	}
	if f.host.focused != f.v.browser.List() || f.confirm.Visible() {
		t.Errorf("focus = %T, confirm visible %v; want the list, closed", f.host.focused, f.confirm.Visible())
	}
}

func TestSnippetsViewRefreshAndActivateReread(t *testing.T) {
	f := newSnippetsFixture(t)
	if err := os.WriteFile(filepath.Join(f.root, "added.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	f.press('r')
	if !f.v.browser.Select("added.txt") {
		t.Error("r didn't pick up a file added outside the app")
	}

	if err := os.WriteFile(filepath.Join(f.root, "another.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	f.v.Activate()
	if !f.v.browser.Select("another.txt") {
		t.Error("Activate didn't re-read the folder")
	}
}

// ── Import ────────────────────────────────────────────────────────────────

// importSource writes content to a file named name outside the library
// and returns its absolute path.
func importSource(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// pressIn focuses the overlay prim and sends k to it.
func pressIn(prim tview.Primitive, k tcell.Key) {
	focusTree(prim)
	send(prim, key(k))
}

func (f *snippetsFixture) promptText(t *testing.T) string {
	t.Helper()
	return renderedScreenText(t, f.prompt.Primitive(), 64, 8)
}

func (f *snippetsFixture) load(t *testing.T, rel string) snippet.Snippet {
	t.Helper()
	sn, err := snippet.NewStore(f.root).Load(filepath.FromSlash(rel))
	if err != nil {
		t.Fatal(err)
	}
	return sn
}

// TestSnippetsViewImportPlainJSON is the common case: a plain JSON file
// is imported (formatted, like any library snippet), the view lands on
// it, and the editor opens on JMS Type so one can be added right away.
func TestSnippetsViewImportPlainJSON(t *testing.T) {
	f := newSnippetsFixture(t)
	const raw = `{"orderId":42,"items":["a","b"]}`
	src := importSource(t, "order.json", raw)
	f.v.browser.SetDir("orders")

	f.press('i')
	if !f.prompt.Visible() || !strings.Contains(f.promptText(t), "Import file") {
		t.Fatal("i didn't open the Import file prompt")
	}
	typeInto(f.prompt.Primitive(), src)

	if !f.prompt.Visible() || !strings.Contains(f.promptText(t), "Import as") || !strings.Contains(f.promptText(t), "order.json") {
		t.Fatalf("step 2 not shown with the file's name prefilled: %q", f.promptText(t))
	}
	pressIn(f.prompt.Primitive(), tcell.KeyEnter) // accept the prefilled name

	sn := f.load(t, "orders/order.json")
	if sn.JMSType != "" || sn.Body != snippet.FormatBody(raw) || sn.Body == raw {
		t.Errorf("imported snippet = %#v, want no JMS Type and the formatted body", sn)
	}
	if got, _ := os.ReadFile(src); string(got) != raw {
		t.Errorf("source file changed to %q", got)
	}
	if !strings.Contains(f.host.status, "Imported") || !strings.Contains(f.host.status, "orders/order.json") {
		t.Errorf("status = %q", f.host.status)
	}
	if f.v.browser.Dir() != "orders" || f.selectedName() != "order.json" {
		t.Errorf("view at %q on %q, want orders on order.json", f.v.browser.Dir(), f.selectedName())
	}

	if !f.editor.Visible() {
		t.Fatal("editor didn't open for a snippet without a JMS Type")
	}
	typeInto(f.editor.Primitive(), "OrderCreated") // lands in JMS Type, Enter saves
	if sn := f.load(t, "orders/order.json"); sn.JMSType != "OrderCreated" {
		t.Errorf("JMS Type after the editor = %q, want OrderCreated", sn.JMSType)
	}
	if f.editor.Visible() || f.host.focused != f.v.browser.List() {
		t.Errorf("editor visible %v, focus %T; want closed, the list", f.editor.Visible(), f.host.focused)
	}
}

// TestSnippetsViewImportSnippetFile covers importing a teammate's snippet
// file: its JMS Type and extra key are kept, and no editor opens.
func TestSnippetsViewImportSnippetFile(t *testing.T) {
	f := newSnippetsFixture(t)
	src := importSource(t, "shared.json", "---\n# from the team\nauthor: someone\njmsType: OrderCreated\n---\n{}")

	f.press('i')
	typeInto(f.prompt.Primitive(), src)
	pressIn(f.prompt.Primitive(), tcell.KeyEnter)

	sn := f.load(t, "shared.json")
	if sn.JMSType != "OrderCreated" || !strings.Contains(sn.Extra, "author: someone") || !strings.Contains(sn.Extra, "# from the team") {
		t.Errorf("imported snippet = %#v, want the JMS Type, key and comment kept", sn)
	}
	if f.editor.Visible() {
		t.Error("editor opened although the snippet has a JMS Type")
	}
	if f.host.focused != f.v.browser.List() {
		t.Errorf("focus = %T, want the list", f.host.focused)
	}
}

func TestSnippetsViewImportIntoSubfolderName(t *testing.T) {
	f := newSnippetsFixture(t)
	src := importSource(t, "order.json", "---\njmsType: T\n---\n{}")

	f.press('i')
	typeInto(f.prompt.Primitive(), src)
	typeInto(f.prompt.Primitive(), "eu/renamed.json")

	if f.load(t, "eu/renamed.json").JMSType != "T" {
		t.Error("eu/renamed.json not imported")
	}
	if f.v.browser.Dir() != "eu" || f.selectedName() != "renamed.json" {
		t.Errorf("view at %q on %q, want eu on renamed.json", f.v.browser.Dir(), f.selectedName())
	}
}

// TestSnippetsViewImportNeverOverwrites checks an existing name keeps
// step 2 open with an error, and the existing snippet is untouched.
func TestSnippetsViewImportNeverOverwrites(t *testing.T) {
	f := newSnippetsFixture(t)
	src := importSource(t, "ping.txt", "the imported one")

	f.press('i')
	typeInto(f.prompt.Primitive(), src)
	pressIn(f.prompt.Primitive(), tcell.KeyEnter) // prefilled "ping.txt" exists

	if !f.prompt.Visible() || !strings.Contains(f.promptText(t), "Import as") {
		t.Errorf("step 2 closed on an existing name")
	}
	if !strings.Contains(f.host.status, `"ping.txt": already exists`) {
		t.Errorf("status = %q", f.host.status)
	}
	if got := f.load(t, "ping.txt").Body; got != "ping" {
		t.Errorf("existing snippet changed to %q", got)
	}
	if f.confirm.Visible() {
		t.Error("an overwrite confirmation was offered; import never overwrites")
	}
}

func TestSnippetsViewImportStep1Refusals(t *testing.T) {
	for _, tt := range []struct {
		name    string
		path    func(t *testing.T) string
		wantErr string
	}{
		{"relative path", func(*testing.T) string { return "order.json" }, "absolute path"},
		{"a folder", func(t *testing.T) string { return t.TempDir() }, "not a file"},
		{"a binary file", func(t *testing.T) string { return importSource(t, "data.bin", "ab\x00cd") }, "not a text file"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newSnippetsFixture(t)
			f.press('i')
			typeInto(f.prompt.Primitive(), tt.path(t))

			if !f.prompt.Visible() || !strings.Contains(f.promptText(t), "Import file") {
				t.Errorf("the Import file prompt didn't stay open")
			}
			if !strings.Contains(f.host.status, tt.wantErr) {
				t.Errorf("status = %q, want %q", f.host.status, tt.wantErr)
			}
		})
	}
}

func TestSnippetsViewImportEscWritesNothing(t *testing.T) {
	t.Run("at step 1", func(t *testing.T) {
		f := newSnippetsFixture(t)
		f.press('i')
		pressIn(f.prompt.Primitive(), tcell.KeyEscape)
		if f.prompt.Visible() || f.host.focused != f.v.browser.List() {
			t.Errorf("prompt visible %v, focus %T; want closed, the list", f.prompt.Visible(), f.host.focused)
		}
	})
	t.Run("at step 2", func(t *testing.T) {
		f := newSnippetsFixture(t)
		src := importSource(t, "order.json", `{"a":1}`)
		f.press('i')
		typeInto(f.prompt.Primitive(), src)
		pressIn(f.prompt.Primitive(), tcell.KeyEscape)

		if f.exists("order.json") {
			t.Error("Esc at step 2 still imported the file")
		}
		if f.prompt.Visible() || f.editor.Visible() || f.host.focused != f.v.browser.List() {
			t.Errorf("prompt %v, editor %v, focus %T; want both closed, the list", f.prompt.Visible(), f.editor.Visible(), f.host.focused)
		}

		// A new import afterwards starts clean at step 1.
		f.press('i')
		if !strings.Contains(f.promptText(t), "Import file") {
			t.Errorf("next import didn't start at step 1: %q", f.promptText(t))
		}
	})
}

func TestSnippetsViewImportWorksInEmptyLibraryAndOnUpRow(t *testing.T) {
	t.Run("empty library", func(t *testing.T) {
		host := newFakeViewHost()
		store := snippet.NewStore(filepath.Join(t.TempDir(), "snippets"))
		confirm := dialog.NewConfirmDialog(host)
		prompt := dialog.NewTextPrompt(host)
		v := NewSnippetsView(host, store, confirm, dialog.NewSnippetEditor(host, store, confirm), prompt)
		focusTree(v.browser.List())
		send(v.browser.List(), runeKey('i'))
		if !prompt.Visible() {
			t.Error("i didn't open the prompt in an empty library")
		}
	})
	t.Run(`".." row`, func(t *testing.T) {
		f := newSnippetsFixture(t)
		f.v.browser.SetDir("orders")
		f.v.browser.List().SetCurrentItem(0)
		f.press('i')
		if !f.prompt.Visible() {
			t.Error(`i didn't open the prompt on ".."`)
		}
	})
}

func TestSnippetsViewShortcutImport(t *testing.T) {
	f := newSnippetsFixture(t)
	for _, s := range f.v.Shortcuts() {
		if s.Key == "i" && s.Description == "import file" {
			return
		}
	}
	t.Error(`Shortcuts() lacks {i, "import file"}`)
}

func TestSnippetsViewImportNameRejectsDotDot(t *testing.T) {
	f := newSnippetsFixture(t)
	src := importSource(t, "order.json", "---\njmsType: T\n---\n{}")
	f.v.browser.SetDir("orders")
	f.press('i')
	typeInto(f.prompt.Primitive(), src)
	typeInto(f.prompt.Primitive(), "../escaped.json")

	if f.exists("escaped.json") || !f.prompt.Visible() {
		t.Errorf("imported %v, prompt visible %v; want \"..\" refused as typed", f.exists("escaped.json"), f.prompt.Visible())
	}
}
