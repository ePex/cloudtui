# Tasks: Snippet import — add a file from anywhere to the library

Each task is implemented and approved on its own, then pushed. Every new
test is mutation-checked: remove the fix and confirm the test fails, and
make sure the mutated code still compiles.

1. [x] **Reading an outside file (`snippet/import.go`).**
   - `MaxImportSize`, `CleanImportPath` (with an injectable home folder
     and OS), and `ReadImportFile`.
   - Table-driven tests as listed in `plan.md`: Unix and Windows path
     rules, `~`, quotes, escaped spaces, relative paths rejected; plain
     JSON; a snippet file keeping `Extra`; a symlink; a folder; a
     missing file; exactly 1 MiB vs 1 MiB + 1; a NUL byte; invalid
     UTF-8; broken front matter; the source left unchanged.
2. [x] **`SnippetEditor.FocusJMSType()`.**
   - Moves focus to the JMS Type field of an open editor.
   - Test: after `ShowEdit` + `FocusJMSType`, typed text lands in JMS
     Type, and Enter saves.
3. [x] **The `i` key in the Snippets view.**
   - The two chained prompts, the pending import, saving without
     overwrite, landing on the snippet, the status message, and the
     editor opening (focused on JMS Type) when the snippet has none.
     `i` is in the shortcut hint.
   - View tests with real key events, as listed in `plan.md`.
4. [x] **README.**
   - Add `i` to the Snippets keys table and a short "Importing a file"
     note to the Message snippets section.
5. [x] **Live verification.**
   - Use the `verify-live` skill with a temporary `HOME`, run from the
     scratch folder (not `tui/`). Check each step on screen and record
     the results here.
   1. Import a plain JSON file by absolute path. The editor opens on
      JMS Type; type one and press Enter. The library file has the JMS
      Type and the formatted body, and the source file is unchanged.
   2. Import a file whose path has spaces, pasted with `\ ` escapes,
      and another pasted in quotes, then one via `~/…`.
   3. Import a teammate-style snippet file (JMS Type, `author:`, a
      comment). No editor opens; the key and the comment are kept.
   4. The refusals: a relative path, a folder, a binary file, a file
      over 1 MiB, broken front matter, and an existing name at step 2.
      Each keeps its prompt open with a clear error.
   5. Esc at step 1 and at step 2 write nothing.
   6. Send the imported snippet through the send dialog's **Load
      snippet…**.
   7. Switch the theme live and check the view during an import.

   **Results (2026-09-23).** The TUI ran in tmux with a temporary `HOME`,
   started from the scratch folder, against the local broker, on a
   throwaway `snippet-import-verify` queue (removed afterwards). The test
   files lived in a scratch folder. All 7 steps pass.

   1. `order.json` (compact plain JSON) imported by absolute path. Step
      2 prefilled `order.json`. The status says `Imported … as
      order.json`. The editor opened with the formatted body and the
      cursor in JMS Type: typing `OrderCreated` + Enter saved it. The
      library file has the JMS Type and the formatted body; the source
      is byte-for-byte unchanged.
   2. The path styles all work:
      - `…/my\ orders/shipped\ order.json`, pasted as a terminal does
      - `"…/quoted file.json"` in quotes
      - `~/Downloads/from-home.json`, with `~` expanded to the temporary
        home

      Each got the right prefilled name. The editor opened each time and
      Esc kept the snippet as imported.
   3. `team-snippet.json` (JMS Type `PaymentReceived`, a document
      comment, `author:` with an end-of-line comment) imported without
      opening the editor. The key and both comments are kept; only the
      body was formatted.
   4. Refusals, each keeping its prompt open:
      - a relative path: `is not an absolute path: enter an absolute
        path`
      - a folder: `is not a file`
      - a binary file: `is not a text file`
      - 1 MiB + 1 byte: `is larger than 1 MiB`
      - broken front matter: the parse error
      - an existing name at step 2: `"order.json": already exists`, with
        no overwrite offer, and the existing snippet unchanged
   5. Esc at step 2 and at step 1 close everything and return focus to
      the list. The next `i` starts at step 1. Afterwards the library
      holds exactly the 5 snippets from steps 1–3.
   6. **Load snippet…** lists all imported snippets. Loading
      `order.json` fills in `OrderCreated` and its body, and the sent
      message arrives on the queue with that JMS Type and body.
   7. After a live switch `dark` → `cyberpunk`, the view, both import
      prompts, and the editor that opens after import have no `dark`-only
      colors (per-cell scan).
6. [ ] **Merge-back** (needs your explicit go-ahead before it's
   committed).
   - `spec/22`: an Import subsection, `i` in the keys table, the new
     functions, and import taken off the out-of-scope list.
   - Delete `spec-wip/fe-snippet-import/` and mark the PR ready for
     review.
