# Plan — FE 41

## `internal/app/datadoglogdetail.go`

```go
var correlationIDPattern = regexp.MustCompile(`(?i)correlationid:\s*([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})`)

// extractCorrelationID pulls a "CorrelationID: <uuid>" value out of a
// Datadog log message, case-insensitive on the label. Returns ("", false)
// if the message doesn't contain one.
func extractCorrelationID(message string) (string, bool) {
    m := correlationIDPattern.FindStringSubmatch(message)
    if m == nil {
        return "", false
    }
    return m[1], true
}
```

`Shortcuts()`:
```go
return []ui.Shortcut{
    {Key: "c", Description: "copy message"},
    {Key: "g", Description: "go to CloudWatch"},
    {Key: "Esc", Description: "back"},
}
```

Input capture, new case alongside the existing `c`/Esc handling:
```go
case event.Rune() == 'g':
    id, ok := extractCorrelationID(dv.event.Message)
    if !ok {
        dv.app.statusBar.SetText("[yellow]No CorrelationID found in this log message[-]")
        return nil
    }
    dv.app.pendingCloudWatchPattern = id
    dv.app.switchTo("cloudwatch-logs")
    return nil
```

## `internal/app/app.go`

New field: `pendingCloudWatchPattern string` (plain field, not a
`*Visible`/overlay-style flag — it's a one-shot value, not a mode).

`switchTo` — clear the queued pattern on any navigation away from the
group list (the abandonment case, spec decision 4):
```go
func (a *App) switchTo(name string) {
    if name != "cloudwatch-logs" {
        a.pendingCloudWatchPattern = ""
    }
    for _, v := range a.views {
        ...
```
This runs *before* the loop, so `switchTo("cloudwatch-logs")` itself
(the jump) never clears the value it was just asked to carry — the
guard only fires for every *other* destination.

`openLogSearch` — consume-and-clear, pass into `open`:
```go
func (a *App) openLogSearch(logGroupName string) {
    pattern := a.pendingCloudWatchPattern
    a.pendingCloudWatchPattern = ""
    a.logSearchV.open(logGroupName, pattern)
    ...
```

## `internal/app/logsearch.go`

```go
// open resets the view for a freshly-selected log group and runs the
// first search with the default time range. initialPattern pre-fills
// the filter pattern (used when arriving via FE 41's CorrelationID
// jump); pass "" for the normal empty-pattern default.
func (sv *logSearchView) open(logGroupName, initialPattern string) {
    sv.logGroupName = logGroupName
    sv.pattern = initialPattern
    sv.patternInput.SetText(initialPattern)
    sv.presetIdx = defaultPresetIdx
    ...
```
(same body otherwise — `search()` at the end already picks up
`sv.pattern`, so this is the only change needed; no double-search.)

## Testing

- `datadoglogdetail_test.go`: `extractCorrelationID` table-driven cases
  — the confirmed real format, case-insensitive label, no match in a
  message without one, doesn't over-match trailing text. `g` key
  handler: with a CorrelationID → `pendingCloudWatchPattern` set +
  front page becomes `cloudwatch-logs`; without one → status bar message
  set, front page unchanged.
- `app_test.go`: `openLogSearch` consumes and clears
  `pendingCloudWatchPattern`, passing it through to
  `logSearchV.pattern`/`patternInput`; `switchTo` to a non-`cloudwatch-logs`
  destination clears a stale queued pattern; `switchTo("cloudwatch-logs")`
  itself does not clear one just set.
- `logsearch_test.go`: update the existing
  `TestLogSearchViewOpenResetsStateAndSearches` call site for the new
  `open(name, pattern)` signature; add a case confirming a non-empty
  `initialPattern` is reflected in `sv.pattern`/`patternInput` after
  `open`.
