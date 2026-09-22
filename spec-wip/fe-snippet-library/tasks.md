# Tasks: Snippet library — manage snippets and folders in the TUI

Each task is implemented and approved on its own, then pushed. Every new
test is mutation-checked: remove the fix and confirm the test fails.

1. [x] **Keep unknown front-matter keys.**
   - Add `Snippet.Extra` (the whole front matter when it holds more than
     a lone `jmsType`). `Parse` goes through `yaml.Node`; `Format` uses
     `Extra` as a template and only sets, adds or removes `jmsType`.
   - Tests:
     - unknown keys kept, including order and a comment
     - `jmsType` added, changed, or cleared with `Extra` intact
     - a round trip with `Extra`
     - a non-mapping front matter is an error
     - the existing format tests still pass unchanged
2. [x] **Store operations.**
   - `MkDir`, `Move`, `Delete`, `DeleteFolder`, `Count`, `Stat`, and
     `Entry.IsLink` (set by `List`).
   - Table-driven tests, as listed in `plan.md`, including the
     symlinked-folder cases.
3. [x] **Extract `SnippetBrowser`.**
   - Move the picker's folder browsing into `dialog/snippetbrowser.go`,
     with a "cursor moved" callback, `Selected`, `Dir`, `SetDir`,
     `Select`, `Reload`, and an optional extra hint line.
   - `SnippetPicker` becomes a thin overlay around it. The existing
     picker tests pass **unchanged**; new tests cover the added
     callbacks and methods.
4. [ ] **`TextPrompt` dialog.**
   - A one-field prompt with **OK**/**Cancel**. An `onSubmit` error keeps
     it open. Enter is handled in the input capture.
   - Wired into `app.go` (page, overlay list, theming). Added to the
     dialog theme-switch regression test.
5. [ ] **`SnippetEditor` dialog.**
   - New and edit modes: overwrite prompt, rename via `Move`, `Extra`
     kept, discard-changes prompt, Enter in Name/JMS Type saves.
   - Wired into `app.go`. Added to the dialog theme-switch regression
     test.
6. [ ] **`SnippetsView`.**
   - Two-pane layout, preview, keys `n`/`N`/`e`/Enter/`R`/`d`/`r`, the
     delete confirmations (snippet, folder with counts, empty folder,
     symlinked folder), and focus restore.
   - Registered in `a.views`, `a.pages` and `themables`; Home entry in
     the ActiveMQ section; `:snippets`.
   - View tests as listed in `plan.md`. Added to the view theme-switch
     regression test. `app` tests for `:snippets` and the Home entry.
7. [ ] **Live verification.**
   - Use the `verify-live` skill with a temporary `HOME`, run from the
     scratch folder (not `tui/`). No broker is needed except for step
     10. Check each step on screen and record the results here.
   1. Open the view from Home and with `:snippets` on an empty library:
      the hint mentions `n`.
   2. `n`: create `orders/created.json` with a JMS Type and a multi-line
      JSON body. The file on disk matches, and the cursor lands on it.
   3. The preview shows the JMS Type and the raw body. On a folder it
      shows the counts.
   4. Edit it: change the body and JMS Type and save. Then add an
      `author:` key and a comment by hand, edit and save again. The
      key and comment survive.
   5. Esc after changing a field asks "Discard changes?"; No keeps you
      in the editor.
   6. `N`: create `eu/archive` (nested). `R` the snippet to
      `eu/archive/created.json`. The view follows it. `R` onto an
      existing name is refused.
   7. `R` a folder into its own subfolder is refused.
   8. `d` on a snippet, on a folder with contents (the counts are
      right), and on an empty folder. No keeps everything each time.
   9. Symlink a scratch folder into the library. `d` shows the
      link-only wording, and after Yes the target folder still exists.
   10. Open the send dialog on a queue (local broker): **Load snippet…**
       shows the library's current state.
   11. Switch the theme live: the view, the editor and the prompt pick
       up the new colors.
   12. Edit a file outside the app, then press `r`: the preview updates.
8. [ ] **Example snippets** (requested during task 2).
   - **Decided:** both of the following, with generic content only (no
     real internal system, queue or company names).
     - an `examples/snippets/` folder in the repo that can be copied into
       `~/.cloudtui/snippets/`: JSON, XML and plain-text messages, some
       with a JMS Type, some in subfolders, and one with an extra
       front-matter key and a comment
     - a "Message snippets" section in the README: where the library
       lives, the file format with an example, how to use the examples
       folder, and the Snippets view's keys
   - Nothing is written into the user's home automatically.
   - A test loads every file in `examples/snippets/` through
     `snippet.Parse`, so a broken example fails CI.
9. [ ] **Merge-back** (needs your explicit go-ahead before it is
   committed).
   - `spec/22`: add a library section, the new `Store` methods, `Extra`
     and the generalized `ErrExists` text, and take the library view off
     the out-of-scope list.
   - `spec/05`: the Home entry.
   - `spec/03`: the new dialogs, the dialog-to-dialog links, the view,
     and `ShowWithCancel` users.
   - `tui/CLAUDE.md`: the package-layout dialog list.
   - Delete `spec-wip/fe-snippet-library/` and mark the PR ready for
     review.
