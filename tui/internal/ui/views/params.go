package views

import "github.com/ePex/cloudtui/tui/internal/ui"

// NewParams returns the placeholder view for SSM Parameter Store:
// browse by path, get/put parameters.
func NewParams() ui.View {
	return &placeholder{
		name:        "params",
		title:       "Parameter Store",
		description: "Browse parameters by path, get and put values.",
	}
}
