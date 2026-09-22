# Plan: Snippet import — add a file from anywhere to the library

## Approach

Everything about *reading* an outside file lives in `internal/snippet`,
next to the format and the store, with no UI dependency:
- turning the typed or pasted path into a clean absolute path
- the checks: regular file, text, 1 MiB, parses

The Snippets view adds the `i` key and chains two `TextPrompt` steps.
Saving goes through the existing `Store.Save` (without overwrite), so
naming, `ErrExists` and the JSON/XML body formatting from #33 come for
free.

No new dependencies and no new dialogs.

## `internal/snippet/import.go` (new)

- **`const MaxImportSize = 1 << 20`** (1 MiB).
- **`CleanImportPath(input string) (string, error)`:**
  - Trims surrounding spaces, then one pair of matching surrounding
    quotes (`"…"` or `'…'`).
  - On non-Windows (`runtime.GOOS != "windows"`), unescapes `\ ` to a
    space, as terminals paste dragged-in files. On Windows a backslash
    is a path separator and is left alone.
  - Expands `~` and a leading `~/` (or `~\` on Windows) to
    `os.UserHomeDir()`.
  - Rejects an empty result, and any path that isn't `filepath.IsAbs`,
    with "enter an absolute path".
  - Returns `filepath.Clean(path)`.
  - Home lookup and the OS are injectable for tests (an unexported
    variant takes them as parameters), so both the Unix and Windows
    rules are tested on any OS.
- **`ReadImportFile(path string) (Snippet, error)`:**
  - `os.Stat`, which follows symlinks. Missing → error. A folder or
    anything else that isn't a regular file → "not a file".
  - Size above `MaxImportSize` → "larger than 1 MiB". This is checked
    from `Stat` before reading, and again on the bytes actually read,
    in case the file grew in between.
  - `bytes.IndexByte(data, 0) >= 0` or `!utf8.Valid(data)` → "not a text
    file".
  - `Parse(data)`: a broken front matter returns the parse error.
  - Errors name the file, e.g. `"/tmp/order.json": larger than 1 MiB`.
  - It only reads; nothing is written.

## `internal/view/snippets.go`: the `i` key

- **`i`** works anywhere in the view, including an empty library and
  the `..` row, since it imports into the current folder.
- **Step 1**, `prompt.Show("Import file", "Path:", "", onSubmit, onClose)`:
  - `onSubmit` runs `CleanImportPath` and then `ReadImportFile`. Any
    error keeps the prompt open.
  - On success it stores the result as the view's pending import
    (source path and snippet) and returns `nil`.
  - `onClose` runs after the prompt has hidden itself (confirmed in
    `TextPrompt.close`). If an import is pending, it opens step 2 on the
    same `TextPrompt`. If not (Esc), it restores focus as usual.
- **Step 2**, `prompt.Show("Import as", "Name:", base name of the source, onSubmit, onClose)`:
  - `onSubmit` runs `ValidateName` on the typed name (before joining,
    as the editor does), joins it to the current folder, and calls
    `Store.Save(path, sn, false)`.
  - `ErrExists` becomes `"<name>": already exists` (other errors as
    they are) and keeps the prompt open.
  - On success: the status bar says `Imported <source> as <name>`, the
    view lands on the snippet, and it returns `nil`.
  - `onClose` clears the pending import and restores focus. That covers
    Esc at step 2 too, which writes nothing.
- Shortcuts gain `{i, "import file"}`, and the README's keys table gets
  an `i` row.

### Editor after import for the common case (approved)

A plain JSON file imports with **no** JMS Type, and the send dialog
requires one. So right after importing a snippet with no JMS Type, the
view opens the **editor** on it, with the cursor in JMS Type:
- type the type and press Enter to save it
- Esc (nothing changed) closes the editor and keeps the snippet as
  imported

A file that already had a JMS Type (a teammate's snippet file) just
lands in the list, as the spec says.

This needs a small `SnippetEditor` addition: `FocusJMSType()`, which
moves focus to that field after `ShowEdit`. The view calls it right
after step 2's prompt has closed, from its `onClose`, so the prompt
isn't hiding anything while the editor opens. Spec step 7.

## Testing

- **`snippet` (`import_test.go`, table-driven, `t.TempDir()`):**
  - `CleanImportPath`, with the Unix and Windows rules both tested
    through the injectable variant:
    - absolute paths
    - `~` and `~/x`
    - quoted paths, and escaped spaces (Unix only)
    - relative paths, `./x` and empty input rejected
  - `ReadImportFile`:
    - a plain JSON file
    - a snippet file with a JMS Type and extra keys (`Extra` kept)
    - a symlink to a file, when symlinks are available
    - rejected: a folder, a missing file, exactly 1 MiB (accepted) vs
      1 MiB + 1 (rejected), a NUL byte, invalid UTF-8, broken front
      matter
    - the source bytes are unchanged afterwards
- **`view` (driven with real key events, as in the library tests):**
  - a plain JSON import is formatted in the library, the cursor lands on
    it, the status bar is set, and the source is unchanged
  - an import into a subfolder name (`orders/x.json`)
  - an existing name keeps step 2 open with the error, and nothing is
    overwritten
  - a relative path and a folder path keep step 1 open
  - Esc at step 1 and at step 2 write nothing and return focus to the
    list
  - `i` works in an empty library
  - the editor opens for a file without a JMS Type, focused on JMS
    Type, and not for a file that has one
- **Mutation checks** for each rule, as before.
- **Live check** (temporary `HOME`, from the scratch folder):
  - importing a plain JSON file, including a path with spaces pasted
    with `\ ` escapes, and a quoted path
  - importing a teammate-style snippet file (JMS Type and `author:`
    kept)
  - the refusals: relative path, folder, binary file, too big, existing
    name
  - sending the imported snippet via **Load snippet…**
  - a live theme switch

## Spec merge-back target

- `spec/22-message-snippets`: an "Import" subsection under Library view,
  `i` in the keys table, `CleanImportPath`/`ReadImportFile`/
  `MaxImportSize` in Data & config, and import taken off the
  out-of-scope list.
- The README's snippet section.
