1. [ ] Add the reusable blocking operation progress overlay, including determinate and indeterminate Nyan-style rendering, theme support, and focused tests.
2. [ ] Wire overlay lifecycle methods through the app host and root page stack, with tests for show, update, hide, and overlapping operation ownership.
3. [ ] Integrate indeterminate progress into queue loading and purge/move-all backend calls; preserve stale-load protection and existing completion/error behavior.
4. [ ] Integrate determinate progress into bulk message delete and move loops; preserve partial-failure reporting and reload/focus behavior.
5. [ ] Run focused unit tests and the queue/message smoke path; manually verify the requested flows against a broker, including through the proxy, and record results.
6. [ ] Update the relevant end-state spec documentation after implementation is complete.
