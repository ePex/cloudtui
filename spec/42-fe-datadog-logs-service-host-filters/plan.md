# Plan — FE 42

## Key implementation detail found while designing

`tview.DropDown.SetCurrentOption(idx)` **invokes the `selected`
callback** if one is set (confirmed in `tview`'s source: `if d.selected
!= nil { d.selected(...) }`). Rebuilding a dropdown's options after
every search and then restoring the (possibly still-valid) selection
via `SetCurrentOption` would otherwise recursively re-trigger `search()`
— an infinite loop. Fix: clear the callback via `SetOptions(options,
nil)` before `SetCurrentOption`, then reattach it via `SetSelectedFunc`
afterward. Only user-driven selections (via the list UI) go through the
live callback; programmatic reconciliation after a search does not.

## `internal/app/datadoglogs.go`

Fields added to `datadogLogsView`:
```go
serviceFilterDD *tview.DropDown
hostFilterDD    *tview.DropDown
serviceFilter   string
hostFilter      string
```

Layout: a new fixed-height horizontal `tview.Flex` row between `table`
and `queryInput`:
```go
filterRow := tview.NewFlex().SetDirection(tview.FlexColumn).
    AddItem(serviceFilterDD, 0, 1, false).
    AddItem(hostFilterDD, 0, 1, false)

flex := tview.NewFlex().SetDirection(tview.FlexRow).
    AddItem(table, 0, 1, true).
    AddItem(filterRow, 1, 0, false).
    AddItem(queryInput, 1, 0, false)
```

Each `DropDown` gets a label (`"Service: "` / `"Host: "`, matching the
label-color convention already used for `queryInput`) and a
`SetInputCapture` that sends `Esc` back to the table (everything else —
arrows, Enter, type-to-jump — stays native `DropDown` behavior, unlike
`queryInput`'s up/down redirect, since here the list navigation *is*
the point).

`const filterAnyOption = "(any)"`.

```go
func (dv *datadogLogsView) applyFilterOptions(dd *tview.DropDown, values []string, current *string, onSelect func(string)) {
    options := append([]string{filterAnyOption}, values...)
    idx := 0
    for i, v := range options {
        if v == *current {
            idx = i
            break
        }
    }
    if idx == 0 {
        *current = ""
    }
    dd.SetOptions(options, nil) // no callback yet — SetCurrentOption below must not fire it
    dd.SetCurrentOption(idx)
    dd.SetSelectedFunc(func(text string, index int) {
        if text == filterAnyOption {
            onSelect("")
            return
        }
        onSelect(text)
    })
}

func (dv *datadogLogsView) rebuildFilterOptions() {
    serviceSet, hostSet := map[string]bool{}, map[string]bool{}
    for _, e := range dv.results {
        if e.Service != "" {
            serviceSet[e.Service] = true
        }
        if e.Host != "" {
            hostSet[e.Host] = true
        }
    }
    dv.applyFilterOptions(dv.serviceFilterDD, sortedKeys(serviceSet), &dv.serviceFilter, func(v string) {
        dv.serviceFilter = v
        dv.search()
    })
    dv.applyFilterOptions(dv.hostFilterDD, sortedKeys(hostSet), &dv.hostFilter, func(v string) {
        dv.hostFilter = v
        dv.search()
    })
}
```
(`sortedKeys` — small local helper, `maps.Keys` + `slices.Sort` or a
plain loop+`sort.Strings`; matches this codebase's general preference
for plain loops over generic/stdlib-iterator ceremony for small cases.)

Called from `handleSearchResult` right after `dv.results = events`, so
option lists are always in sync with what's actually on screen — but
**only on success**; an error leaves the previous options/selection
alone rather than wiping them out from an unrelated failed request.

`search()` builds the combined query:
```go
func (dv *datadogLogsView) effectiveQuery() string {
    var parts []string
    if dv.serviceFilter != "" {
        parts = append(parts, fmt.Sprintf("service:%q", dv.serviceFilter))
    }
    if dv.hostFilter != "" {
        parts = append(parts, fmt.Sprintf("host:%q", dv.hostFilter))
    }
    if dv.query != "" {
        parts = append(parts, dv.query)
    }
    return strings.Join(parts, " ")
}
```
`search()` passes `dv.effectiveQuery()` to `dv.app.searchDatadogLogs`
instead of `dv.query` directly.

`Shortcuts()`: add `{Key: "S", Description: "filter service"}`,
`{Key: "H", Description: "filter host"}`.

`table`'s `SetInputCapture`: add `case 'S':` /
`case 'H':` → `dv.app.tv.SetFocus(dv.serviceFilterDD)` /
`dv.hostFilterDD`.

## `internal/app/app.go`

`onGlobalKey`: two more per-field exemptions, same shape as
`datadogLogsV.queryInput`'s existing one:
```go
if a.datadogLogsV != nil && a.tv.GetFocus() == a.datadogLogsV.serviceFilterDD {
    return event
}
if a.datadogLogsV != nil && a.tv.GetFocus() == a.datadogLogsV.hostFilterDD {
    return event
}
```

## Testing

- `applyFilterOptions`: unit tests — options rebuilt from given values
  (`(any)` always first), current selection preserved when still valid,
  reset to `""`/`(any)` when not. **The no-recursion guard is tested by
  calling `applyFilterOptions` directly with an instrumented `onSelect`
  callback**, not by faking `searchDatadogLogs` and counting async
  calls: since `search()`'s network call happens inside a goroutine, a
  fake-and-counter check running immediately after a synchronous call
  returns would be racing that goroutine in both directions (a real bug
  could still show 0 calls at assertion time, a correct implementation
  could show a leftover call from prior test state) — the same reason
  this file's other tests only ever assert on state mutated *before*
  `search()` is called, never on whether the async call itself
  happened. Passing a plain `onSelect` that sets a bool directly to
  `applyFilterOptions` sidesteps `search()`/the goroutine entirely and
  is fully deterministic: one test confirms `onSelect` is *not* invoked
  during reconciliation (`SetOptions(..., nil)` before
  `SetCurrentOption`), another confirms a *subsequent* `SetCurrentOption`
  call (simulating a real user selection, made after the real callback
  is reattached) still reaches it.
- `effectiveQuery`: table-driven — no filters (just free text), service
  only, host only, both + free text, confirming the quoting.
- `onGlobalKey`: exemption tests for both new dropdowns, mirroring the
  existing `queryInput` one.
- `Shortcuts()` includes `S`/`H`.
