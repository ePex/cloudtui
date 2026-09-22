# Plan: Message snippets — format, storage, save & load

## Approach

A new package, `internal/snippet`, holds everything to do with the file
format and the file system. It knows nothing about the UI and is fully
unit-testable against `t.TempDir()`.

Two new overlays in `internal/dialog` use it:

- a **save dialog**, opened from the message detail view
- a **picker**, opened from the send-message dialog

A small change to `queue.Message` records whether its JMS Type was
inferred rather than read from the header, so the save path can tell the
two apart.

No new dependencies. Front matter is parsed with `gopkg.in/yaml.v3`,
which is already in `go.mod`.

## `internal/snippet` (new)

`snippet.go`: the format.

- `type Snippet struct { JMSType, Body string }`
- `Parse(data []byte) (Snippet, error)`
  - If the first line isn't exactly `---`, the whole of `data` is the
    body.
  - Otherwise scan for the next line that is exactly `---`; the lines in
    between are YAML, decoded into a struct with a `jmsType` field.
    Unknown keys are ignored (yaml.v3's default).
  - The body is every byte after the closing line's line ending.
  - A missing closing line or invalid YAML returns a wrapped error.
  - Delimiter lines may end in `\n` or `\r\n`, so files edited on Windows
    still parse. The body keeps its bytes untouched either way.
- `Format(s Snippet) []byte`
  - Writes `---\njmsType: <v>\n---\n` + body when `JMSType != ""`.
  - Otherwise writes just the body, unless the body's first line is
    `---`: then an empty front-matter block goes in front of it, so
    `Parse` doesn't read the body as front matter.
  - The front matter is produced with `yaml.Marshal`, so values that
    need quoting (e.g. `jmsType: "yes"` or a leading `#`) round-trip
    correctly.

`store.go`: the file system.

- `type Store struct { root string }`, `NewStore(root string)`, and
  `DefaultRoot() (string, error)` →
  `filepath.Join(home, ".cloudtui", "snippets")`, the same
  `os.UserHomeDir` pattern as `config.DefaultPath`.
- `Root() string`, used by the picker's "No snippets yet" hint.
- `type Entry struct { Name string; IsDir bool }`
- `List(dir string) ([]Entry, error)`
  - `dir` is relative to the root, `""` meaning the root itself.
  - Returns folders first, then files, each sorted case-insensitively.
  - Hidden entries (names starting with `.`) and anything that is
    neither a regular file nor a folder are skipped. Symlinks are
    classified by their target (`os.Stat`), so a link to a shared folder
    works; broken links are skipped.
  - A root that doesn't exist yet returns an empty list, not an error.
- `Load(name string) (Snippet, error)`: reads the file and calls `Parse`.
- `Save(name string, s Snippet, overwrite bool) error`
  - Validates `name` and creates missing parent folders
    (`os.MkdirAll`, `0o755`).
  - Writes the file with `0o644`. With `overwrite == false` the file is
    opened `O_CREATE|O_EXCL`, and `ErrExists` is returned when the file
    already exists (checked with `errors.Is`), so the check and the
    create happen in one step with no race between them.
- `ValidateName(name string) (string, error)`
  - Trims surrounding spaces and converts `/` to the OS separator with
    `filepath.FromSlash`, so `orders/created.json` also works on Windows.
  - Rejects an empty name, a trailing separator, and anything
    `filepath.IsLocal` refuses (absolute paths, `..`, reserved Windows
    names).
  - Also rejects any path segment that starts with `.`, because `List`
    would hide that snippet from the picker. The spec doesn't mention
    this rule; it follows from the "hidden entries are skipped" rule.
- Names passed to `Load` and `List` always come from `List` itself (the
  picker), but they go through the same `filepath.IsLocal` guard anyway.

## `queue.Message`: telling header from inferred type

- Add `JMSTypeInferred bool` to `queue.Message` (in
  `internal/queue/backend.go`), with a short comment.
- Set it to `true` at the four places that currently infer a type:
  - `jolokia/messages.go`: the `browseMessages` path
  - `jolokia/messages.go`: the legacy plain-string `browse()` item
  - `jolokia/messages.go`: the `browse()` object path
  - `proxy/proxy.go`: `toQueueMessage`
- I chose a flag over re-reading `RawFields` in the detail view because
  the header's key differs per path (`jMSType`, `JMSType`, or absent for
  the proxy). Only the backends know which one applies.

## `internal/dialog`

**`ConfirmDialog`: add a cancel callback.** Today, "No" and Esc call
`FocusMain`. That is wrong when the confirmation sits on top of another
overlay (the send dialog, the save dialog): focus would land behind the
overlay that is still visible.

- Add `ShowWithCancel(question, onConfirm, onCancel func())`.
- `Show` calls it with `onCancel == nil`, which keeps today's behavior.
- A non-nil `onCancel` replaces the `FocusMain` call.
- Existing callers are unchanged.

**`snippetsave.go` (new): `SnippetSaveDialog`**

- A `tview.Form` with a "Name" input field and **Save**/**Cancel**
  buttons, shown on page `snippet-save`. Built with the same structure
  as `SendMessageOverlay` (`Show`, `close`, `ApplyPalette`, `Primitive`,
  `Visible`).
- `Show(s snippet.Snippet, onClose func())`: clears the name field and
  focuses it.
- **Save:**
  1. Run `ValidateName`. An error goes to the status bar and the dialog
     stays open (the same pattern as `SendMessageOverlay.doSend`).
  2. Call `Save(name, s, false)`.
  3. On `ErrExists`, call `confirm.ShowWithCancel("Overwrite snippet
     %q?")`. **Yes** calls `Save(…, true)`. **No** refocuses the name
     field.
  4. Report success or failure in the status bar.
- Enter in the Name field saves. It's caught in the field's input
  capture, not `SetDoneFunc`: `tview.Form` runs its own "finished"
  handler right after the done func and moves focus to the next
  element, which would take focus away from the overwrite confirmation.
- File I/O is small and local, so it runs synchronously on the UI
  goroutine with no `QueueUpdateDraw` round-trip. This keeps
  "exists → confirm" a simple sequential flow.

**`snippetpicker.go` (new): `SnippetPicker`**

- A bordered `tview.List` on page `snippet-picker`, following the
  `MovePicker` pattern: `j`/`k` are mapped to arrows, and Esc closes.
- Stores the current folder (relative to the root) and titles itself
  ` Snippets — /orders ` so you can see where you are.
- `fill()` rebuilds the list from `List`:
  - a `..` entry comes first when not at the root
  - folders are shown with a trailing `/`
  - an empty root shows three disabled rows (`tview.List` doesn't wrap,
    so one long row would cut off the path): `No snippets yet.`,
    `Save one from a message (S), or add files to:`, and the root path
  - names are passed through `tview.Escape`, so a `[` in a file name
    isn't read as a color tag
- Backspace also goes up a level (and does nothing at the root).
- Enter on a snippet calls `Load`. A parse error is shown in the status
  bar and the picker stays open. On success, the picker closes and calls
  `onSelect(snippet)`.
- `Show(onSelect func(snippet.Snippet), onClose func())` always starts at
  the root.

**`sendmessage.go`**

- The constructor now takes `*SnippetPicker` and `*ConfirmDialog`.
- Add a third button, **Load snippet…**, after Submit/Cancel, so
  `GetButton(0)` (Submit) keeps its index.
- The button opens the picker. Its `onClose` refocuses the form and
  restores the form's context hint. `onSelect` calls `applySnippet`.
- `applySnippet(s)`:
  - If JMS Type or Body is currently non-empty, it first calls
    `confirm.ShowWithCancel("Replace JMS Type and Body with snippet?")`.
    **No** refocuses the form.
  - Setting the values goes through a separate `setFromSnippet(s)`
    method that writes only `jmsTypeItem` and `bodyItem`, so the rule
    that nothing else is touched is directly testable.

## `internal/view/message_detail.go`

- The constructor now takes `*dialog.SnippetSaveDialog`.
- `S` builds the snippet with a pure helper
  `snippetFromMessage(msg queue.Message) (snippet.Snippet, error)`:
  - the body is `RawFields["text"]`, unformatted
  - an empty body returns an error ("message has no text body")
  - `JMSType` is set only when `!msg.JMSTypeInferred`
- On success `S` opens the save dialog. Its `onClose` restores focus and
  the hint in the same way as the `m` handler's `restoreDetail`. I'll
  pull that into a small `restoreFocus()` method rather than copying it,
  since both keys need it.
- Add `{Key: "S", Description: "save as snippet"}` to `Shortcuts()`.

## `internal/app/app.go`: wiring

- Resolve `snippet.DefaultRoot()` once in `New()`. If the home directory
  can't be resolved, log it and use a `Store` with an empty root. Every
  operation on it then returns a clear "snippets folder unavailable"
  error in the status bar, rather than failing app startup over an
  optional feature.
- Build `SnippetPicker` and `SnippetSaveDialog` next to the other
  dialogs, before the views that take them.
- Add the pages `snippet-save` and `snippet-picker` after `send-message`
  and before `confirm`. `tview.Pages` draws in `AddPage` order, so the
  picker draws above the send dialog and the confirmation above both.
- Add both to the theme-applied (`ui.Themeable`) set, like the other
  dialogs.
- Sizes: picker `ui.Centered(…, 60, 20)`; save dialog
  `ui.Centered(…, 64, 8)` (border and padding 4 + one field 2 + buttons
  1 + a spare row, per the sizing comments already in `app.go`).

## Spec merge-back target (step 4)

A new `spec/22-message-snippets/spec.md`, since snippets are a new
capability and two more snippet features will extend it. Short
cross-references go into:

- `spec/08` (the detail view's `S` key)
- `spec/09` (the send dialog's **Load snippet…** button)

## Testing

- **`snippet`** (table-driven, `t.TempDir()`):
  - `Parse`: no front matter, front matter, CRLF delimiters, body
    containing `---`, missing closing line, invalid YAML, unknown keys,
    empty body
  - `Format`/`Parse` round-trip, including values that need quoting
  - `ValidateName`: good names, nested names, empty, absolute, `..`,
    hidden segment, trailing slash
  - `List`: sorting, folders first, hidden entries skipped, missing root
  - `Save`: creates folders, `ErrExists`, overwrite
  - `Load`
- **Backends:** extend the existing message-parsing tests to assert
  `JMSTypeInferred` for header-present and inferred cases on all four
  paths.
- **`dialog`** (existing `testHost`):
  - picker: fill, navigation into and out of folders, never above root,
    empty-root hint, parse error keeps it open
  - save dialog: validation error, `ErrExists` leads to a confirmation,
    overwrite
  - send dialog: `setFromSnippet` touches only JMS Type and Body; a
    non-empty dialog asks for confirmation and an empty one doesn't
  - `ConfirmDialog.ShowWithCancel` calls `onCancel` on No and Esc
- **`view`:** `snippetFromMessage` for a header type, an inferred type,
  and a binary/empty body.
- **Manual:** use the `verify-live` skill against both backends. Save a
  message that has a real JMS Type and one that doesn't, save into a
  subfolder, overwrite (both prompts), load into an empty dialog and a
  filled one, send the loaded message, and try the empty-root hint. The
  exact steps go into `tasks.md`.
