# Tasks — CR 60: extract message filter + Datadog settings into dedicated structs

1. [x] Extract the message filter into a `messageFilter` struct within
       `tui/internal/app/messagefilter.go` (`newMessageFilter`, `show`,
       `close`, `apply`, `clear` — see plan.md). Update `app.go`: remove
       the 2 old fields, add `messageFilter *messageFilter`, `New()`
       calls `newMessageFilter`, both OR-chains' message-filter part
       becomes `a.messageFilter.visible`. Update `messages.go`'s one
       `a.showMessageFilter()` call site to `a.messageFilter.show()`.
2. [x] Extract Datadog settings into a `datadogEditor` struct within
       `tui/internal/app/datadogsettings.go` (`newDatadogEditor`, `show`,
       `close`, `save` — see plan.md). Update `app.go`: remove the 2 old
       fields, add `datadogEditor *datadogEditor`, `New()` calls
       `newDatadogEditor`, both OR-chains' datadog-editor part becomes
       `a.datadogEditor.visible`. Update `settings.go`'s two
       `a.showDatadogEditor()` call sites. Update
       `app_test.go`'s `TestOnGlobalKeyPassesThroughWhenDatadogEditorVisible`
       and all of `datadogsettings_test.go`'s field/method references to
       the new `a.datadogEditor.*` paths — same assertions, same intent.
3. [x] Verify: `go build ./...` and `go test ./...` pass in `tui/`
       (including `TestOnGlobalKeyPassesThroughWhenDatadogEditorVisible`
       with its original intent intact); manual live verification
       (`verify-live` skill) — message filter's Apply/Clear/Cancel/Esc
       flow (no unit-test safety net for this one, per spec.md) and
       Datadog settings' prefill/save/cancel, including confirming typed
       characters in its fields don't trigger global hotkeys.

       `gofmt`, `go vet`, `go build ./...`, `go test ./...` all clean,
       including `TestOnGlobalKeyPassesThroughWhenDatadogEditorVisible`.

       Manually verified 2026-08-16 via `verify-live` (tmux-driven real
       binary, real local `default`/jolokia connection, `orders` scratch
       queue). Message filter: opened (`f`), typed Max Count `5`, `Apply`
       — title correctly updated to `(filter: max=5)` and list reloaded;
       reopened, confirmed prefill showed `5`; `Clear` reset it back to
       the default `(filter: max=500)`; reopened again, confirmed all
       fields blank, typed a probe character into a field to confirm live
       typing works, then `Esc` closed without changing anything.

       Datadog settings: opened from Settings, confirmed prefill
       (`datadoghq.eu` + masked Access Token from the real local config);
       typed the literal character `q` into the Site field and confirmed
       it was appended as text (`datadoghq.euq`) rather than quitting the
       app — the exact scenario `TestOnGlobalKeyPassesThroughWhenDatadogEditorVisible`
       guards, now confirmed live too, not just at the unit level; backed
       out the typed character and `Esc`-cancelled, confirmed the
       Settings list still showed the original `datadoghq.eu` unchanged.
