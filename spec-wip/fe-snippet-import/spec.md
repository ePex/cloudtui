# FE: Snippet import — add a file from anywhere to the library

_Date: 2026-09-23_

Third and last of the snippet features. It builds on
`spec/22-message-snippets`: the file format, the store with its body
formatting, and the Snippets view.

## What

In the Snippets view, `i` imports a message file from anywhere on disk
into the current folder of the library. You give it an absolute path;
the file is read as a snippet and saved into the library like any other.
The original file is left untouched.

## Why

Test messages often already exist as files: exported from another
tool, attached to a ticket, or sitting in a project's test data. Today
they reach the library only by copying files by hand into
`~/.cloudtui/snippets/`, or by pasting their content into the editor.
Import does it from where you already are, and applies the same rules
(naming, formatting, never overwriting) as everything else in the
library.

## Flow

The most common case is importing a plain JSON file (no front matter,
so no JMS Type).

1. **`i`** in the Snippets view opens a prompt titled **Import file**
   asking for a **Path**.
2. **The path:**
   - It must be **absolute**, e.g. `/Users/me/Downloads/order.json` or
     `C:\data\order.json`. `~` and `~/…` are expanded to your home
     folder.
   - Pasted paths are cleaned up: surrounding quotes are removed, and on
     macOS/Linux backslash-escaped spaces (`my\ file.json`, as terminals
     paste a dragged-in file) are unescaped.
   - A relative path is rejected with "enter an absolute path", since
     it's unclear what it would be relative to.
3. **The file must be importable.** Otherwise the prompt stays open with
   an error in the status bar, so you can fix the path:
   - it exists and is a regular file (a symlink to a file is fine; a
     folder isn't)
   - it's text: valid UTF-8 with no NUL bytes. Binary files are refused,
     since snippets are text.
   - it's at most **1 MiB** (1,048,576 bytes), since messages are small
     and a huge file would make the editor and preview slow
   - it parses as a snippet: a file starting with `---` must have a
     valid front matter, as for any snippet (spec/22)
4. **A second prompt, Import as,** asks for the **Name** in the library.
   - It's prefilled with the file's name (e.g. `order.json`), relative
     to the current folder. `/` creates or uses subfolders, and the
     usual naming rules apply (no `..`, absolute paths, hidden parts, or
     trailing `/`).
   - **Import never overwrites.** If a snippet of that name already
     exists, the prompt stays open with `"<name>": already exists` so
     you can choose another name.
   - Esc cancels the whole import; nothing is written.
5. **Saving:** the file is saved into the library exactly as if you'd
   created it there.
   - Front matter is recognized, so a snippet file from a teammate keeps
     its JMS Type and any other keys and comments.
   - A plain file becomes a snippet with no JMS Type.
   - A valid JSON/XML body is formatted by the library's usual rule
     (spec/22).
   - The source file is only read, never changed.
6. **Afterwards,** the view shows the folder the snippet landed in, with
   the cursor on it, and the status bar says
   `Imported <file> as <name>`.
7. **No JMS Type yet? The editor opens.** This is the common plain-JSON
   case. The send dialog needs a JMS Type, so when the imported snippet
   has none, the snippet editor opens on it straight away, with the
   cursor in **JMS Type**.
   - Type it and press Enter to save.
   - Esc (with nothing changed) closes the editor and keeps the snippet
     as imported.
   - A file that already had a JMS Type (e.g. a teammate's snippet
     file) just lands in the list.

The shortcut hint lists `i` "import file".

## Out of scope

- Importing several files, or a whole folder, at once.
- Tab-completion of paths in the prompt (pasting or dragging a file
  into the terminal covers most cases; could be a small follow-up).
- Importing from a URL.
- A one-off "send from file" in the send dialog that doesn't add to the
  library.
- Overwriting an existing snippet on import.
- Binary files, or files over 1 MiB.
