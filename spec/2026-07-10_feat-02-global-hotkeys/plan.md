# Plan — Global hotkeys

Spec: [spec.md](spec.md)

## Approach

### New views

- `internal/ui/views/home.go` — `NewHome()`, a placeholder (same pattern
  as `NewSecrets`/`NewParams`/`NewQueues`), name `"home"`.
- `internal/ui/views/settings.go` — `NewSettings()`, a placeholder, name
  `"settings"`.
- `App.New()`'s `views` slice becomes `home, secrets, params, queues,
  settings` — `home` first, so it becomes the default view (`New()`
  already just does `switchTo(a.views[0].Name())`, no separate "default
  view" concept to introduce).

### `Filterable` contract (`internal/ui`)

```go
// Filterable is implemented by views that support live filtering via the
// global '/' key. No current view implements it — this exists so future
// list/table views have a contract to implement rather than filtering
// being bolted on ad hoc.
type Filterable interface {
    Filter(query string)
}
```

### Filter input (mirrors the existing command-prompt overlay)

- `topLeft` (the existing `"info"`/`"prompt"` `Pages`) gains a third page,
  `"filter"`, holding a second `*tview.InputField` (`filterInput`,
  labeled `/`).
- `/` (global key): if the active view (looked up via a new
  `a.activeView() ui.View` helper against `a.pages.GetFrontPage()`)
  implements `Filterable`, switch `topLeft` to `"filter"` and focus
  `filterInput`. If it doesn't, no-op — `topLeft` stays on `"info"`,
  focus doesn't move.
- `filterInput`'s `SetDoneFunc` (`onFilterDone`) mirrors `onPromptDone`:
  a `defer` clears the input, switches `topLeft` back to `"info"`, and
  refocuses `pages`; on Enter, if the active view is `Filterable`, calls
  `Filter(text)`.

### Help modal (`internal/app/help.go`, new)

- `newHelpModal(cfg) *tview.TextView` — a bordered, titled panel listing
  all bindings (`h`, `s`, `q`, `?`, `/`, `:`, `esc`), colored with
  `cfg.Colors.Accent`/`Value` like the shortcuts panel.
- `centered(p tview.Primitive, width, height int) tview.Primitive` — the
  standard tview nested-`Flex` pattern for a fixed-size, centered overlay.
- The root primitive changes from the bare `layout` `Flex` to a
  `tview.Pages` (`rootPages`): `"main"` (the existing layout, always
  visible) and `"help"` (the centered modal, hidden by default). Unlike
  `topLeft`'s mutually-exclusive pages, `rootPages` uses `ShowPage`/
  `HidePage` (not `SwitchToPage`) so `"help"` draws *on top of* `"main"`
  instead of replacing it.
- `App` tracks `helpVisible bool` (`Pages` has no "is this page visible"
  getter) so `onGlobalKey` knows whether to treat keys as
  help-dismissal or normal navigation.

### `onGlobalKey` restructuring

```go
func (a *App) onGlobalKey(event *tcell.EventKey) *tcell.EventKey {
    if a.tv.GetFocus() == a.prompt || a.tv.GetFocus() == a.filterInput {
        return event
    }
    if a.helpVisible {
        if event.Key() == tcell.KeyEscape || event.Rune() == '?' {
            a.closeHelp()
        }
        return nil // swallow everything else while help is open
    }
    switch event.Rune() {
    case ':':
        a.prompt.SetText("")
        a.topLeft.SwitchToPage("prompt")
        a.tv.SetFocus(a.prompt)
        return nil
    case 'h':
        a.switchTo("home")
        return nil
    case 's':
        a.switchTo("settings")
        return nil
    case 'q':
        a.tv.Stop()
        return nil
    case '?':
        a.openHelp()
        return nil
    case '/':
        a.beginFilter()
        return nil
    }
    return event
}
```

`onPromptDone`'s existing Enter-key routing (`:q`/`:quit`, `:secrets`
etc.) is untouched.

## Files touched

- `tui/internal/ui/filterable.go` (new) — `Filterable` interface.
- `tui/internal/ui/views/home.go`, `settings.go` (new)
- `tui/internal/ui/views/views_test.go` (modified) — add `home`/
  `settings` to the table-driven constructor test.
- `tui/internal/app/help.go`, `help_test.go` (new)
- `tui/internal/app/topbar.go` (modified) — `newTopBar` takes a second
  `*tview.InputField` (`filterInput`) and adds the `"filter"` page.
- `tui/internal/app/topbar_test.go` (modified) — update `newTopBar` call
  sites for the new parameter.
- `tui/internal/app/app.go` (modified) — `App` gains `rootPages`,
  `filterInput`, `helpVisible`; `New()` builds the `rootPages`/help-modal
  wrapping and the 5-view slice; `onGlobalKey` restructured as above;
  new `openHelp`/`closeHelp`/`beginFilter`/`onFilterDone`/`activeView`.
- `tui/internal/app/app_test.go` (modified) — existing assertions that
  assumed `"secrets"` was the default view now expect `"home"`; new
  tests for `h`/`s`/`q`/`?`/`/` routing, help open/close, and the filter
  overlay (using a small `Filterable`-implementing fake view appended to
  `a.views` in the test, since no real view implements it yet).

## Key decisions / trade-offs

- **Filter reuses the command-prompt overlay pattern** (a third
  `topLeft` page + a second `InputField`) rather than inventing a
  different UI, since the interaction (activate on a key, type, Enter to
  commit, Escape to cancel, revert to info panel) is identical to `:`.
- **Help modal blocks all other keys except `?`/`Escape` while open.**
  Simple, predictable, standard modal behavior — avoids reasoning about
  what "view switch while help is open" should mean.
- **`rootPages` uses `ShowPage`/`HidePage`, not `SwitchToPage`**, because
  the help modal must overlay the main layout, not replace it — this is
  the one place in the app where two pages are visible simultaneously.
- **`Filterable` is a single-method interface** (`Filter(query string)`,
  no return value) — the minimum needed to scaffold the contract per the
  approved spec; no view implements it yet, so there's nothing to
  validate beyond "the plumbing correctly calls it when present and
  no-ops when absent," which is what `app_test.go`'s fake view tests.
- **No test file for `filterable.go` itself** — it's a bare interface
  with no logic (same "genuinely untestable" carve-out already used for
  `Taskfile.yml`); it's exercised indirectly through `app_test.go`'s
  fake `Filterable` view.

## Testing

- `views_test.go`: `NewHome`/`NewSettings` return the expected
  `Name()`/`Title()`.
- `topbar_test.go`: update existing calls for the new `filterInput`
  param; assert the `"filter"` page exists and top bar still defaults to
  `"info"`.
- `help_test.go`: `newHelpModal` text contains all documented key
  tokens; `centered` returns a `*tview.Flex` with three items (spacer,
  sized column, spacer).
- `app_test.go`:
  - Default view is now `"home"` (update the existing default-view
    test); `len(views) == 5` with the new ordering.
  - `h`/`s` switch to `"home"`/`"settings"`; `q` calls `Stop` (verified
    the same indirect way as the existing quit test, since `Stop()` is a
    no-op without a real screen).
  - `?` shows/hides `"help"` on `rootPages` and toggles `helpVisible`;
    while open, other keys (e.g. `h`) are swallowed and don't switch
    views.
  - `/` on a view that doesn't implement `Filterable` (e.g. `"home"`) is
    a no-op: `topLeft` stays on `"info"`, focus unchanged.
  - `/` on a fake `Filterable` view (appended to `a.views`/`a.pages` in
    the test) switches `topLeft` to `"filter"` and focuses
    `filterInput`; typing text and pressing Enter calls `Filter` with
    that text and reverts `topLeft` to `"info"`.
