# Package layout and internal architecture

_Condensed from spec/59 through spec/89 — see those folders for the incremental refactor history (31 CRs plus one bugfix) that arrived at this end state. This doc describes only the target shape, grounded in the actual current source under `tui/internal/`._

## Purpose (why this layered structure exists)

The TUI shell started as one flat package, `internal/app`, holding the composition root, generic chrome (top bar, status bar, help), ~10 modal overlays, and ~16 resource views all together (at its peak: 31 files, ~13,700 lines, `App` with 100+ fields). That mixed three different lifetimes and concerns in one package with no compiler-enforced boundary between them.

The end state is a k9s-inspired split into four layers, each a real Go package boundary (not just a file-naming convention):

- **`internal/ui`** — shared contracts and generic, domain-free chrome. No knowledge of queues, AWS, or Datadog.
- **`internal/dialog`** — modal overlays (confirm, pickers, editors). Depend on `ui.Host`, never on a concrete `*App`.
- **`internal/view`** — resource screens (queues, messages, SSM params, logs, ...). Depend on `ui.ViewHost` (which embeds `ui.Host`), never on a concrete `*App`.
- **`internal/app`** — the composition root. Imports and wires up `ui`, `dialog`, and `view`; implements `Host`/`ViewHost` on `*App`; owns global hotkeys, page routing, and the handful of things that don't belong to any one dialog/view (cross-view navigation trampolines, the theme switch, the CodePipeline background watcher's App-facing methods).

The reason for interface-mediated access rather than a shared `*App` pointer: `internal/dialog` and `internal/view` must not import `internal/app` (that would be circular, since `internal/app` imports both of them to construct and wire them up). Each overlay/view instead takes a `host ui.Host` (or `ui.ViewHost`) at construction time — `*App` satisfies the interface, but the dependency is declared as the interface, so the dialog/view packages compile independently of `internal/app` entirely.

## Package layout

Current file contents (production `.go` files; each also has a colocated `_test.go`):

**`internal/ui`** — interfaces and generic chrome, no resource knowledge:
- `host.go` — the `Host` interface (see below)
- `viewhost.go` — the `ViewHost` interface (see below)
- `view.go` — the `View` interface (`Name()`, `Title()`, `Primitive()`)
- `shortcuttable.go` — the optional `Shortcuttable` interface (`Shortcuts() []Shortcut`) views/dialogs implement to populate the top bar's context panel
- `theme.go` — the `Themeable` interface (recolor-on-theme-switch contract) and palette application
- `topbar.go`, `statusbar.go`, `help.go`, `notify.go` — generic chrome widgets (top bar layout, status bar, help modal, OS desktop notifications)
- `filter.go` — shared inline-filter-input widget behavior
- `style.go` — shared tview styling helpers (`StyleList`, `StyleDropDown`, ...) usable by both dialogs and views
- `timerange.go` — the `TimeRange`/`TimeRangeMode`/time-range-preset types shared by the CloudWatch and Datadog log views and the time-range modal dialog

**`internal/ui/views`** — the home dashboard's own rendering (`home.go`, `SectionInfo`/`ViewInfo` types, `NewHome`); a separate sub-package from `internal/view`, not to be confused with it.

**`internal/dialog`** — the ~10 modal overlay types, one file each: `confirm.go` (`ConfirmDialog`), `movepicker.go` (`MovePicker`), `sendmessage.go` (`SendMessageOverlay`), `connections.go` (`ConnManager` + `ConnEditor` — two types in one file, since they're each other's sibling), `messagefilter.go` (`MessageFilter`), `timerangemodal.go` (`TimeRangeModal`), `datadogsettings.go` (`DatadogEditor`), `themepicker.go` (`ThemePicker`), `awsprofiles.go` (`AWSProfilesPicker`). Plus `hosttest_test.go`/`dialogtest_test.go` — a shared `ui.Host` test double used across the dialog package's tests.

**`internal/view`** — the resource views and their detail-view companions, one file each: `queues.go`, `messages.go`, `message_detail.go`, `ssmparams.go`, `paramdetail.go`, `secrets.go`, `secretdetail.go`, `logs.go` (CloudWatch log-group list), `logsearch.go` (CloudWatch log search), `logdetail.go`, `datadoglogs.go`, `datadoglogdetail.go`, `codepipelinelist.go`, `codepipelinedetail.go`, `settings.go`, `log.go` (the app's own debug-log viewer — distinct from `logs.go`), `pipelinewatcher.go` (`PipelineWatcher` — the CodePipeline background poller, headless, no `ui.View`), `wraptext.go` (`dynamicWrapWidth` — shared free-text-column wrapping for tables, sized from the table's actual rendered width).

**`internal/app`** — the composition root: `app.go` (`App` struct + `New()` + global hotkeys + `SwitchTo`/theme switch/connection switch), `host.go` (implements `ui.Host` on `*App`), `viewhost.go` (implements `ui.ViewHost` on `*App`), `viewwiring.go` (the 8 `OpenX` cross-view navigation trampolines), `codepipelinewatch.go` (3 thin trampoline methods forwarding to `view.PipelineWatcher`), `theme.go` (`reapplyTheme`, iterates `a.themables`).

**`internal/queue/secretbackend`** — `secretbackend.go`: `SecretResolver` (caches AWS-Secrets-Manager-resolved passwords, keyed by profile+secret name, re-resolving on staleness) and a `queue.Backend` decorator that wraps the real backend and resolves its password lazily on first use via the resolver. Pure backend-construction plumbing, no UI surface — this is why it lives under `internal/queue/`, not `internal/dialog` or `internal/view`, even though it moved out of `internal/app` in the same refactor series.

## The Host interface (`internal/ui/host.go`)

The contract the 10 dialogs depend on:

```go
type Host interface {
    // Chrome / focus
    ShowPage(name string)
    HidePage(name string)
    SetFocus(p tview.Primitive)
    FocusMain()
    QueueUpdateDraw(f func())

    // Status / context feedback
    SetStatus(text string)
    SetContextHint(text string)

    // Config / theme
    Config() config.Config
    SwitchTheme(name string)
    SwitchConnection(name string)
    SaveConnection(conn config.Connection, origName string, isNew bool)
    DeleteConnection(name string) (wasActive bool)
    SaveDatadogConfig(cfg config.DatadogConfig)
    SetActiveAWSProfile(name string)
    ListAWSProfiles(ctx context.Context) ([]awsprofile.Profile, error)

    // Backend / queue data
    Backend() queue.Backend
    ReloadAfterSend(queueName string)

    // Messages view interaction
    MessagesFilter() queue.MessageFilter
    ApplyMessagesFilter(f queue.MessageFilter)
    FocusMessages()
}
```

`*App` implements this in `internal/app/host.go`. Config-mutating logic (e.g. `SaveConnection`, `DeleteConnection`, `SetActiveAWSProfile`) lives on `*App` itself, not inline in the dialog's `save`/`delete`/`activate` methods — the dialogs call these named, task-shaped `Host` methods rather than mutating `a.cfg` fields directly, which is what makes them portable to a separate package in the first place.

## The ViewHost interface (`internal/ui/viewhost.go`)

The contract the resource views depend on. It embeds `Host` (views get everything dialogs get, plus more):

```go
type ViewHost interface {
    Host

    // Chrome
    SwitchToPage(name string)
    UpdateContextPanel(v View)
    SwitchTo(name string)
    CopyToClipboard(data string)

    // Cross-view navigation (implemented by App's viewwiring.go)
    OpenMessages(queueName string)
    OpenMessageDetail(queueName string, msg queue.Message)
    OpenParamDetail(param awsssm.Parameter)
    OpenSecretDetail(secret awssecrets.Secret)
    OpenLogSearch(logGroupName string)
    OpenLogEventDetail(event awslogs.LogEvent)
    OpenDatadogLogDetail(event datadoglogs.LogEvent)
    OpenCodePipelineDetail(pipelineName string)

    SetPendingCloudWatchPattern(pattern string)

    // CodePipeline background watcher (forwards to view.PipelineWatcher)
    IsWatchingPipeline(name string) bool
    StartWatchingPipeline(name string)
    StopWatchingPipeline(name string)

    // Injectable data-fetchers, one pair per AWS/Datadog integration
    ListParameters(ctx context.Context, profile, path string) ([]awsssm.Parameter, error)
    RevealParameter(ctx context.Context, profile, name string) (string, error)
    ListSecrets(ctx context.Context, profile string) ([]awssecrets.Secret, error)
    RevealSecret(ctx context.Context, profile, name string) (value string, isBinary bool, err error)
    ListLogGroups(ctx context.Context, profile string) ([]awslogs.LogGroup, error)
    FilterLogEvents(ctx context.Context, profile, logGroupName string, start, end time.Time, pattern string) (events []awslogs.LogEvent, hasMore bool, err error)
    SearchDatadogLogs(ctx context.Context, cfg config.DatadogConfig, query string, from, to time.Time) (events []datadoglogs.LogEvent, hasMore bool, err error)
    ListDatadogFacetValues(ctx context.Context, cfg config.DatadogConfig, facet string, from, to time.Time) ([]string, error)
    ListPipelines(ctx context.Context, profile string) ([]awscodepipeline.Pipeline, error)
    GetPipelineState(ctx context.Context, profile, pipelineName string) ([]awscodepipeline.StageStatus, error)
    AWSAuthTypeFor(ctx context.Context, profile string) (awsprofile.AuthType, error)
    AWSSSOLogin(ctx context.Context, profile string) error
}
```

Why it's a separate, wider interface rather than folding everything into `Host`: dialogs never need cross-view navigation, page-vs-overlay switching, or the AWS/Datadog data-fetchers (those are view-only concerns) — keeping `Host` narrow keeps the dialog test double small and keeps the two families of consumers honestly scoped to what they actually use.

**Deliberate omission**: `ViewHost` does **not** expose access to open a modal dialog (no `ShowMovePicker()`-style method). Unlike dialogs (which only ever need `Host`), 5 of the ~16 views open a dialog directly — `queues.go`, `messages.go`, `message_detail.go` (confirm/move-picker/send-message), and `logsearch.go`/`datadoglogs.go` (time-range modal). Since `internal/view` can import `internal/dialog` directly (no cycle — dialogs don't import views), those 5 views simply take the specific `*dialog.X` instances they need as constructor parameters, exactly the pattern `ConnEditor` already uses for its `ConnManager` sibling reference (`NewConnEditor(host ui.Host, manager *ConnManager)`). The other ~11 dialog-free views only take `ui.ViewHost`.

## Dialogs (`internal/dialog`)

Construction pattern: every dialog's constructor takes `host ui.Host` as its first argument (`func NewX(host ui.Host, ...) *X`), stores it, and calls back through it for anything shared (status text, config mutation, focus). Two dialogs have a sibling reference instead of going through `Host`:

- `ConnManager` and `ConnEditor` (both in `connections.go`) reference each other directly — `ConnManager` opens `ConnEditor` for new/edit/duplicate (`n`/`e`/`d`) and `ConnEditor.save()`/`close()` refreshes/refocuses `ConnManager`'s list. `NewConnEditor(host, manager)` takes the already-constructed `ConnManager` as a second parameter; `ConnManager.SetEditor(editor)` closes the cycle after construction (see the construction order note below).
- `ConnManager` also calls into `ConfirmDialog` for its delete confirmation, taken as a constructor parameter (`NewConnManager(host, confirm)`).

Every dialog type exposes `Primitive()` (the tview widget to embed in an overlay page), `Visible() bool`, and `Show(...)`/`close()` methods; most implement `ui.Themeable` for live theme-switch recoloring.

**Keybinding hints are embedded in the dialog's own layout, not the top bar.** Each dialog with its own keybindings (e.g. `ConnManager`, `AWSProfilesPicker`) builds a `hints *tview.TextView` with color-tagged `<key> action` pairs (e.g. `<Enter> activate  <r> refresh  </> filter  <Esc> close`) and appends it as a fixed-height row at the bottom of its own `Flex`. This is a completely separate mechanism from `Host.SetContextHint`, which only applies to `ui.View`s open in the main content `Pages` and drives the top bar's context panel — a modal overlay is not a `ui.View` and never touches the top bar. Don't reach for `SetContextHint` when giving a dialog its own hints; build the footer `TextView` the same way the existing dialogs do.

## Views (`internal/view`)

Construction pattern: every view's constructor takes `host ui.ViewHost` as its first argument, plus (for the 5 dialog-coupled ones) the specific `*dialog.X` pointer(s) it opens, plus (for list→detail pairs) a callback the App wires up at construction time to open the corresponding detail view — e.g. `NewQueuesView(host, backend, confirm, movePicker, sendMessage, openMessages)`.

View-to-view navigation (e.g. Enter on a queues-table row opening the messages view for that queue) is **not** implemented inside the view types themselves. Neither view "owns" the pair, so that wiring lives centrally in `internal/app/viewwiring.go` as 8 `(a *App) OpenX(...)` trampoline methods, which is also literally what `ui.ViewHost`'s `OpenMessages`/`OpenMessageDetail`/etc. methods resolve to. Each view's constructor is handed the relevant `OpenX` function as a plain callback (e.g. `a.OpenMessages` passed into `NewQueuesView`), so the view calls it on row-selection without needing to know it's reaching back into `App`.

`view.PipelineWatcher` (`pipelinewatcher.go`) is the one file in this package that isn't a `ui.View` — a headless background poller (ticks every `pipelinePollInterval`, calls AWS via its host, dispatches results back onto the UI goroutine via `QueueUpdateDraw`) that both `codePipelineListV` and `codePipelineDetailV` share. `internal/app/codepipelinewatch.go` is 3 one-line trampolines (`IsWatchingPipeline`/`StartWatchingPipeline`/`StopWatchingPipeline`) forwarding to it, kept in `internal/app` purely to satisfy `ui.ViewHost`'s method set.

## Wiring it together — `internal/app/app.go`'s `New()`

Construction order matters and follows a strict dependency chain:

1. **Theme applied first** (`applyTheme(cfg.Colors)`), before any tview primitive is constructed, since `tview.Styles` must be set before primitives read them.
2. **Shell chrome** — `tview.Application`, `Pages`, the home dashboard (`views.NewHome`), the `:` command prompt, the top bar (`ui.NewTopBar`), the status bar.
3. **Injectable data-fetcher fields** on `App` (`listAWSProfiles`, `listParameters`, `searchDatadogLogs`, ...) are set to the real package-level functions (`awsprofile.List`, `awsssm.List`, `datadoglogs.Search`, ...) — these exist so tests can substitute fakes without a network/AWS SDK dependency, and so `ViewHost`'s data-fetcher methods (above) have something concrete to forward to.
4. **`secretResolver` and the initial `backend`** are built (`secretbackend.New(...)`).
5. **The 9 dialogs that views construct directly are built first** (`confirm`, `movePicker`, `sendMessage`, `messageFilter`, `timeRangeModal`, `connManager`, `datadogEditor`, `themePicker`, `awsProfiles`) — they must exist before the views that take them as constructor parameters. `connEditor` is the one exception: it's constructed *after* the dialog-coupled views below, right where it's wired into the overlay stack, because it needs `connManager` (already built) but nothing constructed after it depends on `connEditor` existing early.
6. **Views are constructed**, each passed `a` (satisfying both `ui.Host` and `ui.ViewHost`), its needed dialogs, and its `OpenX` navigation callback (or an inline closure, for the handful of `back`/`onSaved`-style callbacks that aren't full `ViewHost` methods, e.g. message-detail's "return to messages list" closure).
7. **`a.views` (the `[]ui.View` slice)** is populated with only the views that have a Home entry / are reachable via `SwitchTo` by name — `home`, `settings`, `log`, `queues`, `ssm-parameters`, `secrets-manager`, `cloudwatch-logs`, `datadog-logs`, `codepipeline`. Each is added to `a.pages` (the main content `Pages`). Detail views and other "opened, not switched-to" screens (`messages`, `message-detail`, `secret-detail`, `log-search`, `log-event-detail`, `datadog-log-detail`, `codepipeline-detail`, `ssm-param-detail`) are added to `a.pages` directly but not into `a.views`, since they're reached only via an `OpenX` trampoline, never `:command` or Home.
8. **The root layout** (`tb.Root` + `a.pages` + `a.statusBar` in a `FlexRow`) is wrapped in `a.rootPages`, a second, outer `Pages` that layers every modal overlay (centered via `ui.Centered(prim, width, height)`) on top of `"main"`. Overlay z-order is AddPage order — `"confirm"` is added last so it always draws above any other still-visible overlay underneath it (e.g. a delete-confirmation shown from within `conn-manager`).
9. **Bookkeeping slices** built once, after everything exists, so the rest of the code loops over them instead of hand-maintaining OR-chains: `focusExemptInputs` (inputs that swallow global hotkeys while focused), `overlayVisible` (every dialog via its `Visible()` accessor, checked by `anyOverlayVisible()`), `themables` (every view/dialog implementing `ui.Themeable`, looped by `reapplyTheme`).
10. `a.tv.SetRoot(a.rootPages, true)`, initial `SwitchTo(a.views[0].Name())` (home), global key capture installed.

## Notable design decisions worth preserving

- **Interface, not shared struct pointer, breaks the import cycle.** `internal/dialog`/`internal/view` importing `internal/app` for a concrete `*App` would cycle back against `internal/app` importing them to construct/wire everything. `Host`/`ViewHost` are declared in the neutral `internal/ui` package, which neither `dialog` nor `view` needs to avoid importing (they already do, for the interfaces themselves) and which `app` already imports for chrome.
- **Task-shaped Host methods over raw field mutation.** `Host.SaveConnection`/`DeleteConnection`/`SetActiveAWSProfile`/`SaveDatadogConfig` exist so dialogs describe *what* they want done (in domain terms) rather than reaching into `a.cfg.Connections` themselves — this is what actually makes a dialog's `save()` method portable off `*App`, not just the interface boundary by itself.
- **`ViewHost` deliberately doesn't expose dialog access.** Rather than widen the interface for the 5 views that need one, those views take the specific `*dialog.X` pointer(s) directly as constructor parameters — `internal/view` can safely import `internal/dialog` (no cycle risk, since dialogs never import views), so there's no reason to route that access through an interface at all.
- **Cross-view navigation is centralized, not peer-to-peer.** No view calls another view's methods directly; `viewwiring.go`'s `OpenX` trampolines are the only thing that reaches into a target view's state, because App (not either view) owns page routing and focus.
- **Two dialogs keep a direct sibling reference instead of going through `Host`.** `ConnManager`/`ConnEditor`'s mutual reference and `ConnManager`'s reference to `ConfirmDialog` are the only inter-dialog dependencies in the whole package; they're passed as constructor parameters rather than added to `Host`, since `Host` is meant to expose only shell-level capabilities, not let one dialog type discover another by name.
- **`secretbackend` lives under `internal/queue/`, not `internal/dialog`/`internal/view`.** It has no `ui.View`/`ui.Themeable` surface at all — it's a `queue.Backend` decorator, so it belongs next to the other `queue.Backend` implementations (`internal/queue/jolokia`, `internal/queue/proxy`), not in either UI layer.
- **A `ui.Host` test double (not a full `*App`) backs dialog/view unit tests.** `internal/dialog`'s `hosttest_test.go` provides a minimal struct satisfying `ui.Host` (and, in `internal/view`'s equivalent, `ui.ViewHost`) with injectable fields/fakes per method — this is what lets dialog/view tests build their subject without constructing a full shell, and is also load-bearing proof that the interfaces are complete (a test double that compiles against every method is itself a check that nothing was missed).
