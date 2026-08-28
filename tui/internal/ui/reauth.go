package ui

// ReauthStatusShower is implemented by views that show their own in-table
// transient loading message (e.g. QueuesView's "Loading queues…") and
// want it to reflect an AWS SSO re-auth happening underneath a fetch,
// rather than sitting unchanged for the whole wait. ShowReauthWaiting
// replaces that message while the browser SSO login is in progress;
// ShowReauthDone reverts it once login completes (success or failure),
// right before the underlying fetch retries.
type ReauthStatusShower interface {
	ShowReauthWaiting()
	ShowReauthDone()
}
