# Plan: Snippet library — manage snippets and folders in the TUI

## Approach

Three layers, same split as spec/22:

1. **`internal/snippet`** gets the file operations the library needs:
   delete, move, create folder, count. It also gets the format change
   that keeps unknown front-matter keys.
2. **`internal/dialog`** gets two new overlays:
   - the snippet editor
   - a small single-field path prompt, used for new folder and
     rename/move

   It also gets the folder-browsing list now inside `SnippetPicker`,
   pulled out so the picker and the new view share it.
3. **`internal/view`** gets `SnippetsView`, wired into the app like the
   other top-level views.

No new dependencies: `gopkg.in/yaml.v3`, already used, handles the front
matter.

## `internal/snippet`

### Keeping unknown front-matter keys (spec: edit)

- **Changed from the first draft after a yaml.v3 experiment.** yaml.v3
  attaches comments to specific nodes: a comment above `jmsType` belongs
  to the `jmsType` key, and a line comment to its value. So *removing*
  `jmsType` from the saved YAML would have orphaned its comments.
  Instead, `Extra` holds the **whole** front matter.
- **What `Extra` holds:** the front matter, as normalized YAML, whenever
  it contains more than a lone `jmsType` (other keys, or comments
  anywhere). It's `""` otherwise, so every existing `Snippet{...}`
  literal and `==` comparison still holds for plain files. "Lone
  `jmsType`" is checked structurally (no comments, and either no keys or
  one `jmsType` pair, however quoted), not by comparing text.
- **`Parse`** decodes into a `yaml.Node`.
  - The root must be a mapping, empty, or null; anything else is an
    error ("front matter must be a mapping of keys to values").
  - `jmsType` is read with `Node.Decode`, so a non-scalar `jmsType` is
    still an error, as before.
- **`Format`** uses `Extra` as a template:
  - it sets `jmsType`'s value in place (position, comments and quote
    style kept)
  - it adds `jmsType` as the first key if missing
  - it removes the pair, with its own comments, when `JMSType` is empty

  Without `Extra` it writes the lone `jmsType` line, as before.
- **Result:** after an edit, the file is byte-for-byte the same except
  for the `jmsType` line. Other keys, values, order, comments, blank
  lines after a document comment, and flow lists are all kept (checked
  by printing an edited file). Because `Extra` is normalized, `Parse`
  then `Format` is stable: saving without edits reproduces the same
  `Snippet`.
- The editor passes `Extra` through untouched: it only edits `JMSType`
  and `Body`.

### New `Store` methods

All take root-relative paths and refuse anything outside the root, as
the existing ones do. New names go through `ValidateName`.

- **`MkDir(name string) error`:** creates the folder and any missing
  parents. It's an error if something already exists at `name`.
- **`Move(from, to string) error`:** renames or moves a snippet or a
  folder.
  - `to` must not exist (checked with `os.Lstat`).
  - Missing parents of `to` are created (`MkdirAll`).
  - Moving a folder into itself or a subfolder of itself is refused.
  - Uses `os.Rename`. Moving between different filesystems (e.g. into a
    symlinked shared folder on another volume) returns the OS error,
    which is reported, not worked around.
- **`Delete(name string) error`:** removes a snippet file; refuses
  folders.
- **`DeleteFolder(dir string) error`:** removes a folder and everything
  in it (`os.RemoveAll`). It refuses the root itself (`""`).
  - **A symlinked folder:** `RemoveAll` on a symlink removes only the
    link, never what it points to, so a linked shared folder is safe.
    The confirmation says so (see below).
- **`Count(dir string) (snippets, folders int, err error)`:** counts
  everything nested inside `dir`, using `List`'s rules (hidden entries
  skipped, broken links skipped). It doesn't descend into symlinked
  folders, because deleting removes only the link.
- **`Stat(name string) (Entry, error)`:** whether `name` is a folder or
  a file. Used to tell a symlinked folder apart when deleting (`Entry`
  gains `IsLink bool`, set by `List` too).

## `internal/dialog`

### `SnippetBrowser` (extracted from `SnippetPicker`)

- A new exported type in `snippetbrowser.go`. It holds the `tview.List`,
  the current folder, and the navigation `SnippetPicker` has today:
  - filling the list
  - `..` and Backspace (keeping the cursor on the folder just left)
  - `j`/`k`
  - escaping names with `tview.Escape`
  - the title with the current path
  - the empty-root hint
- It takes callbacks for "snippet chosen" (Enter on a file) and "cursor
  moved" (for the view's preview), and exposes `Selected()`
  (the entry under the cursor, or none for `..` and hint rows), `Dir()`,
  `Reload()` and `Select(name)`.
- `SnippetPicker` becomes a thin overlay around a `SnippetBrowser`. Its
  behavior and its tests stay as they are; the tests are the check that
  the extraction changed nothing.
- The empty-root hint gains an optional extra line, which the view uses
  for "Press n to create one".

### `SnippetEditor` (new, `snippeteditor.go`, page `snippet-editor`)

- A `tview.Form`: Name (`InputField`), JMS Type (`InputField`), Body
  (`TextArea`, several rows), and **Save**/**Cancel**. Styled through
  `ui.StyleForm` in `ApplyPalette`.
- **Opening:**
  - `ShowNew(dir string, onSaved func(path string))` starts with empty
    fields.
  - `ShowEdit(path string, sn snippet.Snippet, onSaved ...)` prefills the
    Name with the path relative to the snippet's own folder, and fills
    JMS Type and Body from `sn`.
- **Save:**
  1. Validate `dir + "/" + name`.
  2. **New:** `Save(..., overwrite=false)`. On `ErrExists`, ask
     `Overwrite snippet "<name>"?` (No by default), as the save dialog
     does.
  3. **Edit, name unchanged:** `Save(..., overwrite=true)`, keeping `sn.Extra`.
  4. **Edit, name changed:** `Move(old, new)` first, which fails if the
     target exists and overwrites nothing, then `Save(new, ...,
     overwrite=true)`.
  5. On success: close, report in the status bar, and call
     `onSaved(newPath)`.
  6. On error: report it and keep the dialog open, as the save dialog
     does.
- **Unsaved changes:** Cancel or Esc compares the fields with their
  starting values. If anything changed, it asks `Discard changes?`
  (No by default).
- **Enter** in the Name and JMS Type fields saves. It's handled in the
  field's input capture, not `SetDoneFunc` (see tui/CLAUDE.md's
  `tview.Form` focus gotcha). In Body, Enter inserts a newline, as a
  `TextArea` does.
- **Size:** `ui.Centered(…, 90, 26)`, like the send dialog, which has
  the same body area.

### `TextPrompt` (new, `textprompt.go`, page `text-prompt`)

- A one-field form (title, label, prefilled text, **OK**/**Cancel**)
  used for "New folder" and "Rename / move".
- `Show(title, label, initial string, onSubmit func(text string) error, onClose func())`.
  - A non-nil error from `onSubmit` is shown in the status bar, and the
    prompt stays open for correction.
  - `nil` closes it.
- It has no snippet knowledge, so any later feature can reuse it (e.g.
  `fe-snippet-import`'s path prompt).
- **Size:** 64×8, like the save-as-snippet dialog.

## `internal/view/snippets.go`: `SnippetsView`

- **Layout:** a `tview.Flex`, columns:
  - the `SnippetBrowser` list, about 40% width, bordered, titled with the
    path
  - a bordered preview `TextView`, about 60% width, with dynamic colors
    on and all file content passed through `tview.Escape`
- **Preview** (via the browser's "cursor moved" callback):
  - **snippet:** `Load`, then `JMS Type: <type or (none)>`, a blank
    line, and the raw body. A load error shows the error.
  - **folder:** `Count`, then e.g. `3 snippets, 1 subfolder`, or
    `→ link` for a symlinked folder.
  - **`..` and hint rows:** empty.
- Implements `ui.View` (`Name()` returns `"snippets"`),
  `ui.Shortcuttable`, `ui.Themeable`, and `activatable` (`Activate`
  reloads from disk, so the view re-reads every time it's opened).
- **Keys** go in the list's input capture, alongside the browser's own
  (`j`, `k`, Backspace):

  | Key | Action |
  |---|---|
  | `n` | `SnippetEditor.ShowNew(browser.Dir(), …)` |
  | `N` | `TextPrompt`, then `MkDir` |
  | `e`, Enter on a snippet | `Load`, then `ShowEdit` (a parse error goes to the status bar instead) |
  | `R` | `TextPrompt` prefilled with the root-relative path, then `Move` |
  | `d` | `ConfirmDialog.ShowWithCancel` with the spec's wording; a symlinked folder asks `Remove link "<name>"? The folder it points to is kept.` |
  | `r` | `Reload` |

  - `n` and `r` also work on an empty library.
  - Everything else does nothing when the cursor is on `..` or a hint
    row.
- **After an action** that changes the tree, the view reloads. For
  new, edit, rename/move and new folder, the browser opens the folder
  the item landed in and selects it (`SetDir` + `Select`).
- **Focus:**
  - Each dialog's `onClose` returns focus and the context hint to the
    list, as `MessageDetailView.restoreFocus` does.
  - The confirmation uses `ShowWithCancel` with that same restore.
- **`ApplyPalette`:**
  - the border and title use `p.ViewColor("snippets")`, so the theme
    files can give it a color; it falls back like other views
  - the list goes through `ui.StyleList`
  - the preview's background and base text color are reset (spec/04)
  - the preview is redrawn

## `internal/app/app.go`: wiring

- **Build:** `SnippetEditor` and `TextPrompt` with the other dialogs.
  `SnippetsView` gets the store, the confirm dialog, the editor and the
  prompt.
- **Register:**
  - add the view to `a.views` (so `:snippets` and its autocomplete work)
    and to `a.pages`
  - add a Home entry `{Name: "snippets", Description: "Manage message snippets"}`
    in the ActiveMQ section, after "queues"
- **Overlay pages:** `snippet-editor` and `text-prompt` go before
  `confirm`, so confirmations draw on top. Add both to `overlayVisible`
  and `themables`, and the view to `themables`.

## Spec merge-back target (step 4)

- `spec/22-message-snippets/spec.md` gains a "Library view" section and
  the new `Store` methods and `Extra` field; the library view comes off
  its out-of-scope list.
- `spec/05-home-navigation` gets the new Home entry.
- `spec/03` gets the new dialog files, the new dialog-to-dialog links
  (`SnippetsView`'s dialogs, `SnippetPicker` → `SnippetBrowser`) and
  the view.
- `tui/CLAUDE.md`'s package layout gets the new dialog files.

## Testing

- **`snippet` (table-driven, `t.TempDir()`):**
  - `Parse`/`Format` with `Extra`:
    - unknown keys preserved, including order and a comment
    - `jmsType` added, changed, or cleared while `Extra` survives
    - a round trip with `Extra`
    - a non-mapping front matter is an error
  - `MkDir`: nested; exists as a folder; exists as a file
  - `Move`:
    - rename in place, and into a new subfolder
    - target exists
    - a folder into itself or its subfolder
    - outside the root
  - `Delete`: refuses a folder
  - `DeleteFolder`: recursive; refuses the root; a symlinked folder
    removes only the link and the target survives (skipped where
    symlinks aren't available, as today)
  - `Count`: nested, hidden entries skipped, links not followed
  - `Stat`/`IsLink`
- **`dialog`:**
  - `SnippetBrowser`: the existing picker tests keep passing unchanged,
    plus the "cursor moved" callback and `Select`/`SetDir`.
  - `SnippetEditor`:
    - new, including the overwrite prompt (Yes/No)
    - edit with the same name, keeping `Extra`
    - edit with a rename, and a rename onto an existing name (error, no
      overwrite)
    - validation errors keep the dialog open
    - the discard prompt only when something changed
    - Enter in Name saves without focus being stolen
  - `TextPrompt`: an `onSubmit` error keeps it open; `nil` closes it.
  - Add both new dialogs to `TestDialogsFullyRecolorOnLiveThemeSwitch`.
- **`view`:**
  - `SnippetsView`: preview for a snippet, a folder, a symlinked folder
    and a parse error, and each key's dispatch:
    - `n` and `e` open the editor, `N` and `R` the prompt, `d` the right
      confirmation wording, `r` reloads
    - nothing happens on `..`
    - after new/rename, the cursor lands on the item
  - Add it to `TestViewsFullyRecolorOnLiveThemeSwitch`.
- **`app`:** `:snippets` resolves and appears in the autocomplete; the
  Home entry exists.
- **Live check** (`verify-live`, temporary `HOME` run from the scratch
  folder): the full step list goes into `tasks.md`.
