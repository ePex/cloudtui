# Spec — Bugfix 40: Datadog editor leaked global hotkeys, quitting the app

Date: 2026-08-10

## Background

Reported live: typing (or pasting) into the Datadog editor's Site/Access
Token fields (FE 39, `spec/39-fe-datadog-logs`) sometimes quit the whole
application, and manually pressing `q` while editing always did.

## Root cause

`onGlobalKey` (`internal/app/app.go`) gates the app's global hotkeys
(`h`/`s`/`l`/`q`/`?`/`:`) behind a block that exempts every open overlay
via its `*Visible` flag:

```go
if a.confirmVisible || a.movePickerVisible || a.sendMessageVisible ||
   a.connManagerVisible || a.connEditorVisible || a.themePickerVisible ||
   a.awsProfilesVisible {
    return event
}
```

`datadogEditorVisible` was never added to this list when FE 39 introduced
it. So with the Datadog editor open, every keystroke fell through to the
hotkey switch below — `q` hit `case 'q': a.tv.Stop()` directly, and other
letters (`h`, `s`, `l`) navigated away mid-edit instead of being typed.
The same gap existed in `onPromptDone`'s parallel focus-restoration check.

## Fix

Add `a.datadogEditorVisible` to both exemption lists (`onGlobalKey` and
`onPromptDone`) — the same one-line-per-site fix pattern already used for
every other overlay.

## Scope

- `tui/internal/app/app.go`: both exemption lists.
- `tui/internal/app/app_test.go`: regression test
  (`TestOnGlobalKeyPassesThroughWhenDatadogEditorVisible`), documented
  the same way this file already documents its other "real bug found
  live" per-view exemption gaps.

## Out of scope

- Auditing every other overlay/view for the same class of gap — none
  reported live; not preemptively hunting for hypothetical ones.
