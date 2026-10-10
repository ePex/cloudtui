package ui

import "context"

// OpKind is what Esc does to a running operation (spec-wip
// fe-progress-modal).
type OpKind int

const (
	// OpAbortable is for loads: Esc cancels the operation's context and
	// closes the progress modal at once. The caller discards the result.
	OpAbortable OpKind = iota
	// OpStoppable is for mutations: Esc cancels the context, but the
	// modal stays up (saying it's stopping) until the caller calls
	// Finish. Callers check the context only between steps, so a request
	// that's already been sent always finishes.
	OpStoppable
)

// Operation is one blocking operation, shown in the progress modal while
// it runs. Every method is safe to call from any goroutine.
type Operation interface {
	// Ctx is cancelled when the user presses Esc, and once the
	// operation finishes.
	Ctx() context.Context
	// Stopped reports whether the user pressed Esc.
	Stopped() bool
	// Step records counted progress: done items (failures included), of
	// which failed failed.
	Step(done, failed int)
	// SetDetail sets an extra line in the modal, e.g. AWS SSO login text.
	SetDetail(text string)
	// Finish ends the operation and closes the modal. Idempotent.
	Finish()
}
