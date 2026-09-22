# Message snippets

_Condensed from spec-wip/fe-message-snippets,
spec-wip/fe-snippet-library, and spec-wip/fe-snippet-import. See those
PRs for the incremental history and the reasoning behind each decision._

## Purpose

A library of reusable message snippets, stored as plain files on disk.
You can save a message you are viewing as a snippet, load a snippet into
the send-message dialog (spec/09) when composing a message, and manage
the whole library (create, edit, rename, move, delete snippets and
folders, and import files from anywhere on disk) in the **Snippets**
view. Test or requeue messages you send
often are then one keystroke away, instead of being retyped or pasted
each time.

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
  - `jmsType` is the only key the app reads and writes.
  - **Other keys and comments are kept.** A snippet written by hand or
    by a teammate may carry more (e.g. `author:`, `tags:`, comments).
    Saving from the app changes only the `jmsType` line: it's set in
    place (keeping its position and comments), added as the first key
    if missing, or removed with its own comments when cleared.
    Everything else stays byte-for-byte, apart from YAML normalizing
    unusual spacing or quoting.
  - The front matter must be a mapping of keys to values (or empty);
    anything else, like a list, is an error.
- **No front matter:** the whole file is the body and there is no JMS
  Type. This lets any existing file be dropped into the folder and used
  as is.
- **Broken front matter:** if the first line is `---` but there is no
  closing `---`, or the YAML is invalid, loading the snippet shows a
  status-bar error that names the file. Nothing is filled in.
- **On save:**
  - Front matter is written only when there's a JMS Type or other
    front-matter content to keep.
  - Exception: if the body's own first line is `---`, an empty
    front-matter block (`---` then `---`) is written in front of it, so
    the body isn't read back as front matter.
  - Valid JSON bodies are pretty-printed with two-space indentation.
    Valid XML is indented with two spaces when it has no mixed text and
    child elements; mixed-content XML and XML using `xml:space="preserve"`
    are left unchanged to avoid changing text whitespace. JSON/XML that
    can't be parsed and all other text are stored unchanged.
  - Formatting does not add or remove a trailing newline.
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
  - the **body**, formatted if it is valid JSON/XML (the detail view's
    pretty-printed JSON is only for display)
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

### Library view

- **Opening it:** Home → ActiveMQ → **snippets** ("Manage message
  snippets"), or `:snippets`, which is also in the `:` autocomplete. It
  re-reads the folder from disk every time it's opened, and on `r`,
  since files may change outside the app (e.g. a `git pull` in a shared
  folder).
- **Layout:** two panes.
  - **Left:** the folder listing, browsed exactly like the snippet
    picker: folders first, `..` and Backspace go up (keeping the cursor
    on the folder left, never above the root), `j`/`k`, the current
    path in the title.
  - **Right:** a preview of the entry under the cursor:
    - a snippet: `JMS Type: <type>` (or `(none)`) and the body; valid
      JSON/XML is formatted for display even when the file is compact.
      Preview formatting never writes to the file.
    - a folder: its nested counts, e.g. `2 snippets, 1 subfolder`, or
      `Empty folder`
    - a symlinked folder: `Linked folder`
    - a file that can't be parsed: the parse error
  - An empty or missing library shows the picker's "No snippets yet"
    hint plus "Or press n to create one here."
- **Keys** (listed in the shortcut hint; like other top-level views
  there's no Esc action, and `h` goes Home):

  | Key | Action |
  |---|---|
  | Enter | open the selected folder, or edit the selected snippet |
  | `n` | new snippet in the current folder |
  | `N` | new folder in the current folder |
  | `i` | import a file from anywhere on disk (see Import below) |
  | `e` | edit the selected snippet |
  | `R` | rename or move the selected snippet or folder |
  | `d` | delete the selected snippet or folder |
  | `r` | re-read from disk |
  | Backspace | up a folder |

  `e`, `R` and `d` do nothing on `..` and hint rows, and `e` does
  nothing on a folder.
- **The snippet editor** (`n`, `e`, Enter on a snippet) has three fields
  plus **Save**/**Cancel**:
  - **Name**, relative to the snippet's folder, with `/` for subfolders.
    It follows the save dialog's naming rules, and is checked as typed
    before being joined with the folder, so `..` can't slip through.
  - **JMS Type**, optional; surrounding spaces are trimmed.
  - **Body**, multi-line; Enter inserts a new line.

  Enter in Name or JMS Type saves.
  - **New:** an existing name asks `Overwrite snippet "<name>"?` (No by
    default).
  - **Edit:** only JMS Type and Body change on disk; other front-matter
    keys and comments are kept (see File format). Changing the Name
    renames or moves the snippet. If the new name already exists, that's
    an error and nothing is overwritten.
  - Cancel or Esc with unsaved changes asks `Discard changes?` (No by
    default).
  - A file that can't be parsed doesn't open; the status bar shows the
    error.
  - When an existing snippet is opened, a valid JSON/XML body is formatted
    in the editor and the normalized body is written to the same file
    before the editor opens. Plain text, invalid JSON/XML, and XML mixed
    content are left unchanged.
- **New folder** (`N`): a prompt for a name relative to the current
  folder. Nested names like `eu/archive` create every level. An existing
  folder or file of that name is an error.
- **Rename or move** (`R`): a prompt prefilled with the item's path
  relative to the snippets root. Editing the last part renames it;
  changing the folder part moves it, creating missing folders. The
  target must not exist (nothing is overwritten), and moving a folder
  into itself or a subfolder of itself is refused.
- **Delete** (`d`) always asks, with No as the default:
  - `Delete snippet "<name>"?`
  - `Delete folder "<name>" and its 3 snippets, 1 subfolder?`, with
    counts covering everything nested inside. Yes deletes the folder with
    its contents.
  - `Delete empty folder "<name>"?`
  - `Remove link "<name>"? The folder it points to is kept.` for a
    symlinked folder. Only the link is removed, and folder counts never
    look inside linked folders.
- **Afterwards:** after creating, saving, or moving something, the view
  shows the folder it landed in with the cursor on it. Every action
  reports success or failure in the status bar. A failure leaves the
  view usable, and if the open folder disappeared from disk, the view
  falls back to the nearest folder that still exists.
- Changes are immediately visible in the send dialog's snippet picker,
  since both read the same folder.

### Import

`i` in the Snippets view copies a message file from anywhere on disk
into the current folder. It works on any row, including an empty
library and `..`. The most common case is a plain JSON file.

1. **Import file** asks for the file's **Path**.
   - It must be absolute: a leading `/`, or on Windows a drive path
     (`C:\…`) or UNC path (`\\server\share`). `~` and `~/…` (also `~\…` on
     Windows) expand to the home folder; `~otheruser` doesn't.
   - Pasted paths are cleaned: surrounding spaces and one pair of
     surrounding quotes are removed. On macOS/Linux `\ ` becomes a
     space, as terminals paste a dragged-in file; on Windows backslashes
     stay separators.
   - The file must be a regular file (a symlink to one is followed), at
     most 1 MiB, text (valid UTF-8, no NUL bytes), and have valid front
     matter if it starts with `---`.
   - A problem shows an error naming the file (e.g. `is not an absolute
     path: enter an absolute path`, `is not a file`, `is not a text
     file`, `is larger than 1 MiB`, or the parse error), and the prompt
     stays open.
2. **Import as** asks for the **Name**, prefilled with the file's name
   and relative to the current folder, with the usual naming rules
   (checked as typed, before joining).
   - Import **never overwrites**: an existing name shows
     `"<name>": already exists` and the prompt stays open.
   - Esc at either step cancels, and nothing is written.
3. **The file is saved through the store like any snippet:**
   - front matter is recognized, so a snippet file keeps its JMS Type,
     other keys and comments
   - a JSON/XML body is formatted by the usual rule
   - the source file is only read

   The view lands on the new snippet, and the status bar says
   `Imported <file> as <name>`.
4. **No JMS Type yet (e.g. plain JSON):** the send dialog needs one, so
   the snippet editor opens on the imported snippet right away, with the
   cursor in JMS Type. Type it and press Enter; Esc keeps the snippet as
   imported. A file that already has a JMS Type just lands in the list.

### Examples

- The repo ships generic example snippets in `examples/snippets/`:
  - JSON order events with a JMS Type
  - an XML payment message
  - a JSON event with extra front-matter keys and a comment
  - a plain-text file without front matter
- The README's "Message snippets" section explains the format, the
  keys, sharing, and how to copy the examples into
  `~/.cloudtui/snippets/` (macOS/Linux and PowerShell).
- Nothing is written into the user's home automatically.
- `TestExampleSnippets` (`internal/snippet/store_test.go`) loads every
  example through a `Store`, so a broken or unlisted example fails CI.

## Data & config

No `config.yaml` additions. The snippets root is fixed at
`~/.cloudtui/snippets/`.

```go
// internal/snippet
type Snippet struct {
	JMSType string // "" = none
	Body    string
	Extra   string // the whole front matter as YAML when it holds more than a lone jmsType; "" otherwise
}

func Parse(data []byte) (Snippet, error)
func Format(s Snippet) []byte

type Store struct{ /* root string */ }
type Entry struct {
	Name   string
	IsDir  bool // for a symlink: what it points to
	IsLink bool
}

var ErrExists = errors.New("already exists") // Save (without overwrite), MkDir, Move

func DefaultRoot() (string, error) // ~/.cloudtui/snippets
func NewStore(root string) *Store  // "" root: every call fails with "snippets folder unavailable"
func (s *Store) Root() string
func (s *Store) List(dir string) ([]Entry, error)  // dir relative to root, "" = root; missing root = empty list
func (s *Store) Load(name string) (Snippet, error)
func (s *Store) Save(name string, sn Snippet, overwrite bool) error // ErrExists when !overwrite and it exists
func (s *Store) MkDir(name string) error          // with parents; ErrExists if anything is there
func (s *Store) Move(from, to string) error       // rename/move; never overwrites; refuses a folder into itself
func (s *Store) Delete(name string) error         // a snippet; refuses folders
func (s *Store) DeleteFolder(dir string) error    // recursive; refuses the root; a symlink loses only the link
func (s *Store) Count(dir string) (snippets, folders int, err error) // nested, not inside linked folders
func (s *Store) Stat(name string) (Entry, error)
func ValidateName(name string) (string, error)     // "/"-separated user input -> OS-relative path

const MaxImportSize = 1 << 20                     // 1 MiB
func CleanImportPath(input string) (string, error) // pasted path -> clean absolute path (quotes, "\ ", ~)
func ReadImportFile(path string) (Snippet, error)  // regular file, <= 1 MiB, UTF-8 without NUL, parses; read-only

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
- **`Parse`/`Format` use yaml.v3's node API.** `Extra` is the whole
  front matter (normalized) whenever it holds more than a lone
  `jmsType`, checked structurally rather than by text. `Format` uses
  `Extra` as a template and only sets, adds, or removes the `jmsType`
  pair. `Parse` then `Format` is stable: saving without edits
  reproduces the same `Snippet`.
- **`SnippetBrowser`** (`dialog/snippetbrowser.go`) is the folder list
  shared by `SnippetPicker` and `SnippetsView`.
  - It owns the list's single input capture and runs the owner's key
    handler first (`SetKeys`), then its own `j`/`k`/Backspace.
  - It reports cursor moves (`SetChangedFunc`, the preview's hook) and
    exposes `Selected`, `Dir`, `SetDir`, `Select` and `Reload`.
- **Library view:**
  - `SnippetEditor` is on page `snippet-editor` (90×26) and `TextPrompt`
    (a reusable one-field prompt) on page `text-prompt` (64×8). Both
    pages go before `confirm`, and both are in `overlayVisible` and
    `themables`.
  - `SnippetsView` (`view/snippets.go`) is in `a.views` (hence
    `:snippets`) and `themables`. It implements `Activate` to re-read on
    open.
- **Import:**
  - Reading the outside file lives in `snippet/import.go`, with no UI
    dependency. `CleanImportPath` delegates to a variant that takes the
    OS name and a home lookup, so both the Unix and the Windows path
    rules are tested on any OS; the public function then applies the
    running OS's `filepath.Clean`.
  - The 1 MiB limit is checked from `Stat` and again on the bytes read,
    in case the file grew in between.
  - The view chains its two steps on the one `TextPrompt`. Step 1's
    `onSubmit` keeps the checked file as the pending import, and its
    `onClose` (which runs after the prompt has hidden itself) opens step
    2.
  - After a save, step 2's `onClose` reloads the snippet (so the editor
    shows the formatted body) and calls `SnippetEditor.ShowEdit` +
    `FocusJMSType` when it has no JMS Type.
- **`ConfirmDialog` shows 3 question rows** (about 150 characters at its
  52×8 size), so a delete question naming a deep folder plus its counts
  isn't cut off.
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
  focus-applying `setFocus` so this is caught if it comes back. The
  snippet editor and `TextPrompt` handle Enter the same way, with the
  same kind of test.
- **yaml.v3 attaches comments to nodes** (a comment above `jmsType`
  belongs to the `jmsType` key, a line comment to its value). So `Extra`
  holds the *whole* front matter rather than "everything except
  `jmsType`": removing the pair would have orphaned its comments.

## Out of scope (deliberate)

- Importing several files or a whole folder at once, importing from a
  URL, tab-completion of paths in the import prompt, overwriting on
  import, and a one-off "send from file" that bypasses the library.
- Sending a snippet directly from the library view (**Load snippet…** in
  the send dialog covers that).
- Duplicating or copying snippets, undo, and search or filter in the
  library view.
- Watching the folder for outside changes automatically (`r` refreshes).
- A configurable snippets root path.
- Launching an external `$EDITOR`.
- Storing any header other than JMS Type.
- Binary (non-text) message bodies.
- Search or filter inside the picker.
