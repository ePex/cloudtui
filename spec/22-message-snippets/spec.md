# Message snippets

_Condensed from spec-wip/fe-message-snippets. See that PR for the
incremental history and the reasoning behind each decision._

## Purpose

A library of reusable message snippets, stored as plain files on disk.
You can save a message you are viewing as a snippet, and load a snippet
into the send-message dialog (spec/09) when composing a message. Test or
requeue messages you send often are then one keystroke away, instead of
being retyped or pasted each time.

The library is an ordinary folder tree with no index or database. Teams
can share snippets by copying the folder or keeping it in a git repo.

## Behavior / user flow

### Storage

- The root folder is `~/.cloudtui/snippets/`, next to `config.yaml`. It is
  created the first time a snippet is saved.
- Subfolders are allowed and show up as folders in the picker. The folder
  tree on disk *is* the library structure; there is no manifest file.
- Every regular file under the root is a snippet, whatever its name or
  extension (`order-created.json`, `ping.txt`, `no-extension`). Its
  display name is its file name.
- Hidden entries (names starting with `.`) are skipped, so a `.git`
  folder in a shared checkout doesn't show up.
- Symlinks are followed: a link to a folder or file is listed like the
  thing it points to (e.g. a link to a shared team checkout). Broken links
  are skipped.

### File format

An optional YAML front-matter block, followed by the raw body:

```
---
jmsType: OrderCreated
---
{"orderId": 42}
```

- **Recognizing front matter:**
  - Front matter counts only when the file's very first line is exactly
    `---`, and it ends at the next line that is exactly `---`.
  - Everything after that closing line is the body, byte for byte,
    including any later `---` lines.
  - Delimiter lines may end in `\n` or `\r\n`, so files edited on Windows
    still load.
- **Keys:**
  - `jmsType` is the only key written.
  - Unknown keys are ignored on read, so later versions can add keys
    without breaking older ones.
- **No front matter:** the whole file is the body and there is no JMS
  Type. This lets any existing file be dropped into the folder and used
  as is.
- **Broken front matter:** if the first line is `---` but there is no
  closing `---`, or the YAML is invalid, loading the snippet shows a
  status-bar error that names the file. Nothing is filled in.
- **On save:**
  - Front matter is written only when the JMS Type is non-empty.
  - Exception: if the body's own first line is `---`, an empty
    front-matter block (`---` then `---`) is written in front of it, so
    the body isn't read back as front matter.
  - The body is written exactly as it is, with no pretty-printing and no
    added trailing newline.
- Body content can be anything textual: plaintext, JSON, XML, etc.

### Save a snippet from the message detail view

- `S` ("save as snippet") on the message detail view (spec/08) opens the
  **Save as Snippet** dialog.
- **Name** is the file name and is required.
  - It may contain `/` to put the snippet in a subfolder, e.g.
    `orders/created.json`. `/` works as the separator on every OS, and
    missing subfolders are created.
  - These names are rejected, so the file can't end up outside the
    snippets root or be hidden from the picker: absolute paths, `..`
    segments, a trailing `/`, and any segment starting with `.`.
- **Keys:** **Save** or Enter in the Name field saves. **Cancel** or Esc
  closes the dialog without saving.
- **Only two things are stored:**
  - the **body**, exactly as received (the detail view's pretty-printed
    JSON is only for display)
  - the **JMS Type**, but only when it is the message's real `JMSType`
    header. A type the backend *inferred* (`text`/`bytes`/`other`, see
    spec/08 and spec/11) is not saved.
- No other header or property is stored: no Correlation ID, Group ID,
  custom headers, timestamps, or IDs.
- **Existing name:** a confirmation asks `Overwrite snippet "<name>"?`.
  **No** (the default) returns to the Name field with the typed name kept.
  **Yes** overwrites.
- **Messages that can't be saved:** a message with no text body (e.g.
  binary) can't be saved. `S` shows a status-bar error instead of opening
  the dialog.
- **Errors and success:**
  - A validation or write error is shown in the status bar, and the
    dialog stays open with the cursor in Name.
  - Success closes the dialog and shows `Snippet saved: <name>`.
- The dialog's context hint reads `<Enter> save  <Esc> cancel  (use / for
  subfolders)`.
- `S` is listed in the detail view's shortcut hint. The `?` help overlay
  only lists global keys and doesn't mention it.

### Load a snippet in the send-message dialog

- The send-message dialog (spec/09) has a **Load snippet…** button after
  Submit/Cancel. It opens the **Snippets** picker, which always starts at
  the snippets root.
- **Listing:** folders come first, shown with a trailing `/`, then
  snippets. Each group is sorted alphabetically, ignoring case.
- **Moving through folders:**
  - Enter on a folder opens it, and the title shows the current path,
    e.g. ` Snippets — /orders `.
  - A `..` entry at the top of any subfolder, or Backspace, goes up one
    level. The cursor lands on the folder you just left.
  - Backspace at the root does nothing: the picker never leaves the
    snippets folder.
- **Keys:** Enter on a snippet loads it. Up/Down or `j`/`k` move the
  selection. Esc closes the picker and returns to the dialog unchanged.
- **Empty or missing library:**
  - An empty or missing root shows a three-line "No snippets yet" hint
    with the folder's path. It is split over three rows because
    `tview.List` doesn't wrap and the path would otherwise be cut off.
  - An empty subfolder shows only `..`.
- **Loading:**
  - Loading a snippet sets **Body** and **JMS Type**. If the snippet has
    no JMS Type, that field is cleared.
  - Correlation ID, Group ID, and Headers are never touched.
- **Confirm before replacing:** if JMS Type or Body already hold text, a
  confirmation asks `Replace JMS Type and Body with the snippet?`.
  **No** returns to the dialog unchanged.
- After loading, focus returns to the dialog. The values can be edited
  before sending, and Submit validates them as it always does (e.g. JMS
  Type is required).
- A snippet that can't be read or parsed shows a status-bar error, and
  the picker stays open.

## Data & config

No `config.yaml` additions. The snippets root is fixed at
`~/.cloudtui/snippets/`.

```go
// internal/snippet
type Snippet struct {
	JMSType string // "" = none
	Body    string
}

func Parse(data []byte) (Snippet, error)
func Format(s Snippet) []byte

type Store struct{ /* root string */ }
type Entry struct {
	Name  string
	IsDir bool
}

var ErrExists = errors.New("snippet already exists")

func DefaultRoot() (string, error) // ~/.cloudtui/snippets
func NewStore(root string) *Store  // "" root: every call fails with "snippets folder unavailable"
func (s *Store) Root() string
func (s *Store) List(dir string) ([]Entry, error)  // dir relative to root, "" = root; missing root = empty list
func (s *Store) Load(name string) (Snippet, error)
func (s *Store) Save(name string, sn Snippet, overwrite bool) error // ErrExists when !overwrite and it exists
func ValidateName(name string) (string, error)     // "/"-separated user input -> OS-relative path

// internal/queue: set by every backend where it infers a type
type Message struct {
	// ...
	JMSTypeInferred bool // JMSType was inferred from the body, not read from a header
}
```

## Implementation notes

- **`internal/snippet`** holds the file format and the file system and
  has no UI dependency. `Store` is built once in `app.New()` from
  `DefaultRoot()`. If the home directory can't be resolved, the error is
  logged and the store is rootless: every snippet action then reports
  "snippets folder unavailable" rather than failing startup over an
  optional feature.
- **`Save` without overwrite opens the file `O_CREATE|O_EXCL`,** so the
  existence check and the create happen in one step with no race between
  them.
- **`Load`/`List` refuse paths outside the root** via
  `filepath.IsLocal`, even though their paths normally come from `List`
  itself.
- **`JMSTypeInferred`** is set at every place a backend infers a type:
  - Jolokia's `browseMessages` path, and both kinds of item on its
    `browse()` fallback path (plain-string items and full-object items)
  - the proxy's `toQueueMessage`

  A flag is used rather than re-reading `RawFields` in the detail view
  because the header's key differs per path (`jMSType`, `JMSType`, or
  absent for the proxy); only the backend knows which applies.
- **Dialogs** (`internal/dialog`):
  - `SnippetPicker` is on page `snippet-picker`, 60×20.
  - `SnippetSaveDialog` is on page `snippet-save`, 64×8.
  - Both pages are added after `send-message` and before `confirm`, so
    the picker draws over the send dialog and the confirmation draws
    over both.
  - Both are in `overlayVisible` (global hotkeys are suppressed while
    they're open) and in `themables`.
  - `SendMessageOverlay` takes the picker and the `ConfirmDialog`;
    `SnippetSaveDialog` takes the store and the `ConfirmDialog`.
- **`ConfirmDialog.ShowWithCancel(question, onConfirm, onCancel)`**
  exists for confirmations raised on top of another overlay that stays
  open. "No"/Esc call `onCancel` (which gives focus back to that overlay)
  instead of `FocusMain`. Plain `Show` is `ShowWithCancel` with a nil
  `onCancel`.
- **List item names go through `tview.Escape`,** since a `[` in a file
  name would otherwise be read as a color tag.
- **`MessageDetailView.restoreFocus()`** is shared by the move picker
  (`m`) and the save dialog (`S`) to give focus and the shortcut hint
  back to the detail view.

## Notable gotchas

- **Enter in the save dialog's Name field is handled in the field's
  input capture, not `SetDoneFunc`.** `tview.Form` runs its own
  "finished" handler right after an item's done func. That handler moves
  focus to the next element using the application's `setFocus`, which
  would take focus away from the overwrite confirmation, or off the Name
  field after a validation error. The capture handles Enter and swallows
  it. `snippetsave_test.go` drives Enter through the form with a
  focus-applying `setFocus` so this is caught if it comes back.

## Out of scope (deliberate)

- A library view for browsing, editing, renaming, moving, or deleting
  snippets and folders in the TUI (planned: `fe-snippet-library`).
- Importing a file from an absolute path (planned: `fe-snippet-import`).
- A configurable snippets root path.
- Launching an external `$EDITOR`.
- Storing any header other than JMS Type.
- Binary (non-text) message bodies.
- Search or filter inside the picker.
