# Plan — Bugfix 37

`internal/awsprofile/list.go`, `classify()`: move the SSO case above the
`CredentialProcess` case:

```go
func classify(sc config.SharedConfig) AuthType {
	switch {
	case sc.RoleARN != "":
		return AuthAssumeRole
	case sc.SSOSessionName != "" || sc.SSOStartURL != "":
		return AuthSSO
	case sc.CredentialProcess != "":
		return AuthCredentialProcess
	case sc.Credentials.AccessKeyID != "":
		return AuthStaticKeys
	default:
		return AuthUnknown
	}
}
```

Update the doc comment above it (currently claims credential_process
wins on purpose, "seen in the wild" — that claim was the bug).

`internal/awsprofile/list_test.go`: rename
`TestListMixedSSOAndCredentialProcessPrefersCredentialProcess` →
`TestListMixedSSOAndCredentialProcessPrefersSSO`, flip the expected
`AuthType` to `AuthSSO`, update its doc comment to explain the real-world
shape (aws-sso-util populate-profiles) and cite this fix.

## Testing

Existing table-driven fixture already covers the mixed case; flipping
the one assertion is sufficient — no new test needed. Re-run
`go test ./internal/awsprofile/...` and the full suite.

## Manual verification

Re-run FE 36's task 8 against the same real `example-preprod` profile (SSO
cache already deleted from the FE 36 conversation) — confirm the status
message now appears, the browser opens, and SSM Parameters loads after
completing login.
