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
// Most views that don't implement Shortcuttable simply have no bindings
// beyond the global ones, which the bottom status bar shows at idle. Home is
// the one exception: it implements Shortcuttable to show the global hotkey
// legend in its own otherwise-empty panel, since a transient status bar
// message (an error, a confirmation) can overwrite that legend with nothing
// to restore it.
type Shortcuttable interface {
	Shortcuts() []Shortcut
}
