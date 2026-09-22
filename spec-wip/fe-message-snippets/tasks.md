# Tasks: Message snippets — format, storage, save & load

Each task is implemented and approved on its own, then pushed.

1. [ ] **Snippet format.** Add `internal/snippet/snippet.go`, with
   `Snippet`, `Parse`, and `Format`, plus table-driven tests. The tests
   cover: no front matter, front matter, CRLF delimiters, a body
   containing `---`, a missing closing line, invalid YAML, unknown keys,
   an empty body, and a round-trip including values that need quoting.
2. [ ] **Snippet store.** Add `internal/snippet/store.go`, with `Store`,
   `DefaultRoot`, `Root`, `List`, `Load`, `Save`, `ErrExists`, and
   `ValidateName`, plus tests against `t.TempDir()`. The tests cover:
   - sorting, with folders first
   - hidden entries skipped
   - a missing root returning an empty list
   - nested save creating folders
   - `ErrExists` and overwrite
   - `ValidateName`: accepted and rejected names
   - `Load` and `List` refusing non-local paths
3. [ ] **Distinguish header vs. inferred JMS Type.** Add
   `queue.Message.JMSTypeInferred`, set it at the three Jolokia
   inference sites and the proxy one, and extend the existing backend
   tests to assert it for both a header-present and an inferred message.
4. [ ] **Confirm dialog cancel callback.** Add
   `ConfirmDialog.ShowWithCancel`, with `Show` delegating to it. Add
   tests that No and Esc call `onCancel`, and that `Show` still calls
   `FocusMain`.
5. [ ] **Snippet picker overlay.** Add `dialog/snippetpicker.go`, wired
   into `app.go` (the store, the `snippet-picker` page between
   `send-message` and `confirm`, theming, and sizing). Add tests for:
   - list contents and ordering
   - entering and leaving folders, never going above the root
   - the empty-root hint
   - a parse error keeping the picker open with a status error
   - Esc calling `onClose`
6. [ ] **Load snippet in send dialog.** Add the **Load snippet…** button
   and `applySnippet`/`setFromSnippet` to `SendMessageOverlay`, and
   update its constructor and the wiring in `app.go`. Add tests that:
   - only JMS Type and Body change, and a snippet with no JMS Type
     clears that field
   - a filled dialog asks for confirmation and an empty one doesn't
   - answering No leaves the fields unchanged and refocuses the form
7. [ ] **Snippet save dialog.** Add `dialog/snippetsave.go`, wired into
   `app.go` (the `snippet-save` page, theming, and sizing). Add tests
   for:
   - a validation error keeping the dialog open with a status error
   - a successful save writing the expected file
   - `ErrExists` leading to a confirmation, then Yes overwriting and No
     refocusing the name field
8. [ ] **`S` on the message detail view.** Add `snippetFromMessage`, the
   `S` handler, the `restoreFocus()` refactor shared with `m`, and the
   `S` entry in `Shortcuts()`. Add tests for `snippetFromMessage` with a
   header type (kept), an inferred type (dropped), and an empty or
   binary body (error).
9. [ ] **Live verification.** Use the `verify-live` skill against both
   the Jolokia and the mq-proxy backends, and record the results here.
   Check each of these in the running TUI:
   1. Move `~/.cloudtui/snippets` aside if it exists. Open the send
      dialog on a queue and press **Load snippet…**. The picker shows
      the "No snippets yet" hint with the folder path, and Esc returns
      to the dialog.
   2. Send a message with JMS Type `SnippetTest` and a JSON body. Open
      it in the detail view and press `S`. Save it as
      `orders/created.json`. The status bar confirms, and
      `~/.cloudtui/snippets/orders/created.json` contains front matter
      with `jmsType: SnippetTest` and the unformatted body.
   3. Save a message that has no JMS Type header (e.g. a seeded message
      shown as `text`). The file has no front matter.
   4. Press `S` again with the same name. An overwrite confirmation
      appears. No returns to the name field; Yes overwrites.
   5. Try the names `../x`, `/abs`, and `.hidden`. Each is rejected
      with a status error and the dialog stays open.
   6. Open the send dialog on a queue. **Load snippet…**, go into
      `orders/`, then back up with `..` and with Backspace. Backspace at
      the root does nothing. Load `created.json`: JMS Type and Body are
      filled, and Correlation ID, Group ID, and Headers are unchanged.
   7. With Body filled in, load a snippet again. A replace confirmation
      appears. No keeps the fields; Yes replaces them.
   8. Submit the loaded message. It arrives on the queue with the
      snippet's JMS Type and body.
   9. Add a file by hand whose first line is `---` and that has no
      closing `---`. Loading it shows a status error and the picker
      stays open.
   10. Switch the theme in Settings. The picker and the save dialog pick
       up the new colors.
10. [ ] **Merge-back** (needs your explicit go-ahead before it is
    committed). Add `spec/22-message-snippets/spec.md`, add
    cross-references in `spec/08` and `spec/09`, add `internal/snippet/`
    to `tui/CLAUDE.md`'s package layout, delete
    `spec-wip/fe-message-snippets/`, and mark the PR ready for review.
