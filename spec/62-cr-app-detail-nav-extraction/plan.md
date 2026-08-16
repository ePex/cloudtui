# Plan — CR 62: move detail-navigation trampolines out of app.go

## Approach

For each of the 8 pairs: cut the `open*` function from `app.go`, paste it
verbatim into the target file; cut the matching inline `SetSelectedFunc`
block from `New()`, wrap it in a new `wire<Source>Opens<Target>()` method
defined right after the pasted trampoline in the same target file; replace
the inline block in `New()` with a single `a.wireXxx()` call at the exact
same point (preserving order/comments). Repeat 8 times, build+test after
each to catch mistakes early rather than debugging one 8-way diff.

## The 8 moves

1. **`messages.go`**: `openMessages` + `wireQueuesOpensMessages()` (wires
   `a.queuesV.table`).
2. **`message_detail.go`**: `openMessageDetail` +
   `wireMessagesOpensDetail()` (wires `a.messagesV.table`).
3. **`paramdetail.go`**: `openParamDetail` + `wireSSMParamsOpensDetail()`
   (wires `a.ssmParamsV.table`).
4. **`secretdetail.go`**: `openSecretDetail` + `wireSecretsOpensDetail()`
   (wires `a.secretsV.table`).
5. **`logsearch.go`**: `openLogSearch` + `wireLogsOpensSearch()` (wires
   `a.logsV.table`).
6. **`logdetail.go`**: `openLogEventDetail` +
   `wireLogSearchOpensEventDetail()` (wires `a.logSearchV.table`).
7. **`datadoglogdetail.go`**: `openDatadogLogDetail` +
   `wireDatadogLogsOpensDetail()` (wires `a.datadogLogsV.table`).
8. **`codepipelinedetail.go`**: `openCodePipelineDetail` +
   `wireCodePipelineListOpensDetail()` (wires `a.codePipelineListV.table`).

Example (pair 3, `paramdetail.go`):

```go
// openParamDetail renders the full detail for param and switches to the
// ssm-param-detail page. paramDetailView.render sets the context panel
// itself (its shortcuts change once a SecureString is revealed).
func (a *App) openParamDetail(param awsssm.Parameter) {
	a.paramDetailV.render(param)
	a.paramDetailV.textView.SetTitle(fmt.Sprintf(" Parameter — %s ", param.Name))
	a.pages.SwitchToPage("ssm-param-detail")
	a.tv.SetFocus(a.pages)
}

// wireSSMParamsOpensDetail wires Enter in the SSM parameters table to
// open the detail view for the selected parameter. Called from New()
// once paramDetailV exists.
func (a *App) wireSSMParamsOpensDetail() {
	a.ssmParamsV.table.SetSelectedFunc(func(row, _ int) {
		idx := row - 1 // row 0 is the header
		if idx < 0 || idx >= len(a.ssmParamsV.filtered) {
			return
		}
		a.openParamDetail(a.ssmParamsV.filtered[idx])
	})
}
```

`New()`'s inline block becomes:

```go
a.wireSSMParamsOpensDetail()
```

at the exact point the original block sat (still after `paramDetailV` is
constructed).

## `app.go` changes

- Remove all 8 `open*` function bodies (they move, not duplicate).
- Remove all 8 inline `SetSelectedFunc` blocks from `New()`, replaced by 8
  one-line calls in the same order/position — construction-order
  correctness is unchanged, since each call site is identical to where
  its inline block used to sit.
- No import changes expected in `app.go` itself beyond what naturally
  drops out (e.g. if `awsssm`/`awssecrets`/`awslogs`/`datadoglogs` types
  were only referenced by the moved functions' signatures — checked
  per-move, not assumed).

## Testing

No new tests — pure code motion, same reasoning as CR 59–61. Each of the
8 target files already has its own `_test.go`
(`paramdetail_test.go`, `secretdetail_test.go`, `logsearch_test.go`,
`logdetail_test.go`, `datadoglogdetail_test.go`,
`codepipelinedetail_test.go`, `message_detail_test.go`, `messages_test.go`)
— check whether any of them call `a.openXxx(...)` or reference the
`wire*` closures directly (same-package access, so a file move alone
doesn't break them, but worth confirming no test assumed something lived
in `app.go` specifically, e.g. via a doc comment).

Manual (`verify-live` skill): pick 2–3 of the 8 to exercise live end to
end (e.g. queues→messages, messages→message-detail, SSM params→detail),
confirming Enter still opens the right view with the right content and
correct context-panel shortcuts. The rest lean on each view's existing
unit coverage of the same `open*` functions, per spec.md.
