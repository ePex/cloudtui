package ui

// Shortcut describes a single key binding for display in the top bar's
// context panel when the view that owns it is active.
type Shortcut struct {
	Key         string
	Description string
}

// Shortcuttable is an optional extension of View implemented by views that
// expose their own key bindings. The top bar's middle panel renders these
// shortcuts while the view is active and clears when it isn't.
//
// Views that don't implement Shortcuttable leave the panel blank — the status
// bar already carries the global hotkey legend, so there is nothing to repeat.
type Shortcuttable interface {
	Shortcuts() []Shortcut
}
