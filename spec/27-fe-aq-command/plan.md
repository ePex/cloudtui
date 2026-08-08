# Plan — FE 27: `:aq` command prompt shortcut

## `onPromptDone` changes

Add one case, alongside the existing `q`/`h`/`s` short aliases:

```go
case cmd == "aq" || cmd == "connections":
    a.showConnectionManager()
```

Guard the deferred focus reset:

```go
defer func() {
    a.prompt.SetText("")
    a.topLeft.SwitchToPage("info")
    if !a.connManagerVisible && !a.connEditorVisible && !a.themePickerVisible &&
        !a.confirmVisible && !a.movePickerVisible && !a.sendMessageVisible {
        a.tv.SetFocus(a.pages)
    }
}()
```

Same flag set already used in `onGlobalKey` to suppress global hotkeys
while an overlay is open — reused here for the same "is an overlay
currently owning focus" check.

## Testing

`app_test.go`: `:aq` and `:connections` both set `connManagerVisible` and
focus `connManagerList`; `:aq` from a non-Settings view (Log) still works,
proving it's not accidentally scoped to a particular starting view.

No regression risk to the existing `:settings`/`:home`/`:s`/`:h` tests —
those views live in `a.pages`, where the (now-conditional) `SetFocus(a.pages)`
still runs exactly as before.
