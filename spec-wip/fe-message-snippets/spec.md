# FE: Message snippets — format, storage, save & load

_Date: 2026-09-22_

First of three planned snippet features. This one covers the
end-to-end path from saving a snippet to loading it. The library
management UI (`fe-snippet-library`) and importing from an absolute path
(`fe-snippet-import`) come in later features.

## What

A library of reusable message snippets stored as plain files on disk.
From this feature on you can:

1. **Save** a message you are viewing as a snippet.
2. **Load** a snippet into the send-message dialog when composing a
   message for a queue.

## Why

Sending test or requeue messages currently means retyping or pasting the
body and JMS Type each time. Snippets make recurring messages one
keystroke away. Because they are ordinary files in an ordinary folder
tree, teams can share them by copying the folder or keeping it in a git
repo.

## Storage

- Root folder: `~/.cloudtui/snippets/`, next to the existing
  `config.yaml`. It is created the first time something is saved.
- Subfolders are allowed and are shown as folders in the picker. The
  folder tree on disk *is* the library structure. There is no index or
  manifest file.
- Every regular file under the root is a snippet, whatever its name or
  extension (`order-created.json`, `ping.txt`, `no-extension`). Hidden
  entries (names starting with `.`) are skipped, so a `.git` folder in a
  shared checkout doesn't show up.
- The snippet's display name is its file name.

## File format

An optional YAML front-matter block, followed by the raw body:

```
---
jmsType: OrderCreated
---
{"orderId": 42}
```

- Front matter is recognized **only** when the file's very first line is
  exactly `---`. It ends at the next line that is exactly `---`.
  Everything after that closing line is the body, byte for byte
  (including any later `---` lines).
- `jmsType` is the only key that gets written. Unknown keys are ignored
  on read, so later versions can add keys.
- A file without front matter is a snippet whose whole content is the
  body and which has no JMS Type. This lets any existing file be dropped
  into the folder and used as is.
- If a file starts with `---` but has no closing `---`, or its front
  matter isn't valid YAML, it is reported as an error in the status bar
  when loaded. Nothing is filled in.
- On save, front matter is written only when the JMS Type is non-empty.
  The body is written exactly as it is, with no pretty-printing and no
  added trailing newline.
- Body content can be anything textual: plaintext, JSON, XML, etc.

## Save a snippet from the message detail view

- On the message detail view, `S` ("save as snippet") opens a small
  dialog with:
  - **Name**: the file name, required. It may contain `/` to put the
    snippet in a subfolder (e.g. `orders/created.json`). Missing
    subfolders are created. Absolute paths and `..` segments are
    rejected, so the file can't end up outside the snippets root.
  - **Save** / **Cancel** buttons. Esc cancels.
- Only two things are stored:
  - the **body**, as shown in the detail view but unformatted (not the
    pretty-printed JSON rendering), and
  - the **JMS Type**, but only when it is the message's real `JMSType`
    header. A type the backend *inferred* (`text`/`bytes`/`other`) is not
    saved.
  - No other headers or properties are stored (no Correlation ID, Group
    ID, custom headers, timestamps, or IDs).
- If a snippet with that name already exists, a confirmation asks
  whether to overwrite it. **No** is the default and returns to the name
  dialog so you can choose a different name.
- A message with no text body (e.g. a binary message) can't be saved.
  `S` shows a status-bar error instead of opening the dialog.
- Success and failure are reported in the status bar.
- `S` is listed in the detail view's shortcut hint (the context panel).
  The `?` help overlay only lists global keys, so it doesn't change.

## Load a snippet in the send-message dialog

- The send-message dialog gets a **Load snippet…** button alongside
  Submit/Cancel.
- It opens a snippet picker that starts at the snippets root:
  - folders are listed first, then snippets, each group sorted
    alphabetically
  - Enter on a folder opens it; a `..` entry (or Backspace) goes up one
    level, but never above the root
  - Enter on a snippet loads it; Esc closes the picker and returns to the
    dialog unchanged
  - `j`/`k`/arrows move the selection, matching the other pickers
- If the root folder doesn't exist or is empty, the picker shows a
  "No snippets yet" hint that says where the folder is.
- Loading a snippet sets **Body** and **JMS Type**. If the snippet has no
  JMS Type, the field is cleared. Correlation ID, Group ID, and Headers
  are never touched.
- **Confirm before overwriting:** if Body or JMS Type is non-empty
  before loading, a confirmation asks whether to replace them. **No**
  returns to the dialog unchanged.
- After loading, focus returns to the dialog. The loaded values can be
  edited before sending, and sending still validates them as it does
  today (e.g. JMS Type is required when sending).

## Out of scope

- A snippet library view for browsing, editing, renaming, moving, or
  deleting snippets and folders in the TUI (`fe-snippet-library`).
- Importing a file from an absolute path (`fe-snippet-import`).
- A configurable snippets root path (e.g. pointing at a shared git
  checkout).
- Launching an external `$EDITOR`.
- Storing any header other than JMS Type.
- Binary (non-text) message bodies.
- Search or filter inside the picker.
