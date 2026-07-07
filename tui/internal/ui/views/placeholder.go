// Package views contains the resource views shown in the app's main pages area.
package views

import (
	"fmt"

	"github.com/rivo/tview"
)

// placeholder is a stand-in resource view used until a view's real AWS-backed
// table/detail pane is implemented.
type placeholder struct {
	name        string
	title       string
	description string
}

func (p *placeholder) Name() string { return p.name }

func (p *placeholder) Title() string { return p.title }

func (p *placeholder) Primitive() tview.Primitive {
	tv := tview.NewTextView().
		SetDynamicColors(true).
		SetText(fmt.Sprintf("[::b]%s[::-]\n\n%s\n\n[gray]not yet implemented[-]", p.title, p.description))
	tv.SetBorder(true).SetTitle(fmt.Sprintf(" %s ", p.title))
	return tv
}
