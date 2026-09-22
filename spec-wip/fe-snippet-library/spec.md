# FE: Snippet library — manage snippets and folders in the TUI

_Date: 2026-09-22_

Second of the three snippet features. It builds on `spec/22-message-snippets`
(the file format, the `~/.cloudtui/snippets/` store, saving from a
message, and loading in the send dialog). Importing a file from an
absolute path (`fe-snippet-import`) comes later.

## What

A **Snippets** view for managing the snippet library without leaving
the app. From it you can browse the folder tree, look at a snippet, and
create, edit, rename, move and delete both snippets and folders.

## Why

Today snippets can only be created by saving a received message
(`S` on the detail view). Anything else (writing a snippet from scratch,
fixing a typo, tidying into folders, deleting old ones) means leaving
the app for a file manager or editor. The library view makes the
snippet folder manageable where it's used.

## Opening the view

- **Home:** a new "snippets — Manage message snippets" entry in the
  ActiveMQ section, under "queues".
- **Command prompt:** `:snippets`, which also appears in the `:`
  autocomplete.
- The view re-reads the folder from disk every time it's opened, and on
  `r` (refresh), since files may change outside the app (e.g. a
  `git pull` in a shared folder).

## Layout

Two panes side by side:

- **Left: the folder listing.** It browses exactly like the snippet
  picker (spec/22):
  - folders first (with a trailing `/`), then snippets, each group
    sorted alphabetically
  - `..` at the top of any subfolder
  - Enter on a folder opens it; `..` or Backspace goes up and keeps the
    cursor on the folder just left; Backspace at the root does nothing
  - Up/Down and `j`/`k` move the cursor
  - hidden entries are skipped and symlinks are followed
  - the title shows the current folder, e.g. ` Snippets — /orders `
- **Right: a preview of the selected snippet.** It shows the JMS Type (or
  "(none)") and the body exactly as stored (no pretty-printing). It
  updates as the cursor moves. A folder shows how many snippets and
  subfolders it contains. A file that can't be parsed shows the parse
  error instead.
- An empty or missing library shows the same "No snippets yet" hint as
  the picker, plus a pointer to `n` for creating the first one.

## Actions

Keys, shown in the view's shortcut hint:

| Key | Action |
|---|---|
| Enter | open the selected folder, or edit the selected snippet |
| `n` | new snippet in the current folder |
| `N` | new folder in the current folder |
| `e` | edit the selected snippet |
| `R` | rename or move the selected snippet or folder |
| `d` | delete the selected snippet or folder |
| `r` | refresh from disk |

Like the other top-level views, it has no Esc action: Backspace and `..`
go up a folder, and `h` goes Home.

### New snippet and edit

- `n` and `e` (or Enter on a snippet) open the **snippet editor**, a
  dialog with three fields:
  - **Name**: the file name, relative to the current folder. `/` creates
    or uses subfolders. It follows the same rules as the save dialog:
    absolute paths, `..`, a trailing `/` and hidden segments are
    rejected.
  - **JMS Type**: optional.
  - **Body**: multi-line.

  Buttons are **Save** and **Cancel**.
- **New snippet:** all fields start empty. If the name already exists,
  saving asks whether to overwrite (No is the default), the same as the
  save dialog.
- **Edit:**
  - The fields start with the snippet's current name, JMS Type and body.
  - Saving writes the file in the spec/22 format. Front-matter keys the
    app doesn't know (e.g. an `author:` a teammate added by hand) are
    kept unchanged, so editing a shared snippet never silently drops
    someone else's metadata. Only `jmsType` is added, changed, or
    removed (when cleared).
  - Changing the Name while editing renames the snippet. If the new name
    already exists, it's an error; nothing is overwritten.
- **Cancel or Esc with unsaved changes** asks "Discard changes?"
  (No is the default). Without changes it just closes.
- **After saving,** the listing refreshes and the cursor lands on the
  saved snippet.
- A file that can't be parsed can't be opened in the editor. The status
  bar shows the parse error.

### New folder

- `N` asks for a folder name, relative to the current folder, with the
  same naming rules. Nested names like `eu/orders` create every missing
  level.
- An existing folder or file of that name is an error.
- Afterwards the cursor lands on the new folder.

### Rename or move

- `R` asks for a new path for the selected snippet or folder, relative
  to the **snippets root**. It's prefilled with the current path, e.g.
  `orders/created.json`.
- Editing just the last part renames it. Changing the folder part moves
  it; missing folders are created.
- Same naming rules as above. The target must not exist; nothing is
  overwritten.
- Moving a folder into itself or one of its own subfolders is refused.
- Afterwards the view shows the folder the item landed in, with the
  cursor on it.

### Delete

- `d` on a snippet asks `Delete snippet "<name>"?` (No is the default).
- `d` on a folder asks a confirmation that names what goes, e.g.
  `Delete folder "orders" and its 3 snippets, 1 subfolder?`. The counts
  include everything nested inside, and No is the default. Yes deletes
  the folder with everything in it. An empty folder asks
  `Delete empty folder "orders"?`.
- `d` on a **symlinked** folder (e.g. a link to a shared team checkout)
  removes only the link and asks
  `Remove link "<name>"? The folder it points to is kept.`. The linked
  folder and its snippets are never deleted from here. Folder counts
  (in the preview and in delete confirmations) don't look inside linked
  folders, for the same reason.
- `..` can't be deleted, renamed or edited.

### General

- Every action reports success or failure in the status bar.
- A failure (permissions, a file removed outside the app, ...) leaves
  the view usable, and `r` re-reads the folder.
- Changes made here are immediately visible in the send dialog's snippet
  picker, since both read the same folder.

## Out of scope

- Importing a file from an absolute path (`fe-snippet-import`).
- Sending a snippet to a queue directly from the library (the send
  dialog's **Load snippet…** covers that).
- Duplicating or copying snippets.
- Search or filter inside the library.
- Launching an external `$EDITOR`.
- A configurable snippets root path.
- Undo.
- Watching the folder for outside changes automatically (`r` refreshes
  manually).
