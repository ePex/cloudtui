# Tasks — Bugfix 37

1. [x] Reorder `classify()` in `internal/awsprofile/list.go` (SSO before
   credential_process), update its doc comment.
2. [x] Flip `TestListMixedSSOAndCredentialProcessPrefersCredentialProcess`
   in `internal/awsprofile/list_test.go` to assert SSO wins; rename and
   update its comment.
3. [x] Manual verification against the real `mlf-preprod` profile
   (re-run FE 36 task 8, now that classification is fixed). Confirmed
   working by the user.
