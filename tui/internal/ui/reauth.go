package ui

// ReauthStatusShower is implemented by views that show their own in-table
// transient loading message (e.g. QueuesView's "Loading queues…") and
// want it to reflect an AWS SSO re-auth happening underneath a fetch,
// rather than sitting unchanged for the whole wait. ShowReauthWaiting
// replaces that message with msg while the browser SSO login is in
// progress — called once with the initial "opening browser" text, then
// again with the same text plus the device verification code/URL once
// the login subprocess prints them (see awsauth.Login), so the same
// display can be updated in place rather than needing a second one.
// ShowReauthDone reverts to the view's own default message once login
// completes (success or failure), right before the underlying fetch
// retries.
type ReauthStatusShower interface {
	ShowReauthWaiting(msg string)
	ShowReauthDone()
}
