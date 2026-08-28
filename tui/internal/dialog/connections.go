package dialog

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
	"github.com/ePex/cloudtui/tui/internal/ui"
)

// ConnManager is the AMQ connection manager overlay: lists all configured
// connections and lets the user activate/add/edit/duplicate/delete them.
type ConnManager struct {
	host    ui.Host
	confirm *ConfirmDialog
	editor  *ConnEditor // set after construction — see SetEditor
	flex    *tview.Flex
	list    *tview.List
	hints   *tview.TextView
	visible bool
}

// NewConnManager builds the connection manager overlay's widgets.
func NewConnManager(host ui.Host, confirm *ConfirmDialog) *ConnManager {
	cm := &ConnManager{host: host, confirm: confirm}
	cm.list = tview.NewList().ShowSecondaryText(false)
	cm.hints = tview.NewTextView().SetDynamicColors(true)
	cm.flex = tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(cm.list, 0, 1, true).
		AddItem(cm.hints, 2, 0, false)
	cm.flex.SetBorder(true).SetTitle(" AMQ Connections ")

	cm.list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch {
		case event.Key() == tcell.KeyEscape:
			cm.close()
			return nil
		case event.Rune() == 'n':
			cm.editor.Show(config.Connection{}, true, "")
			return nil
		case event.Rune() == 'e':
			idx := cm.list.GetCurrentItem()
			conns := cm.host.Config().Connections
			if idx >= 0 && idx < len(conns) {
				c := conns[idx]
				cm.editor.Show(c, false, c.Name)
			}
			return nil
		case event.Rune() == 'd':
			idx := cm.list.GetCurrentItem()
			conns := cm.host.Config().Connections
			if idx >= 0 && idx < len(conns) {
				dup := conns[idx]
				dup.Name = dup.Name + "-copy"
				cm.editor.Show(dup, true, "")
			}
			return nil
		case event.Key() == tcell.KeyDelete || event.Rune() == 'x':
			cm.delete()
			return nil
		case event.Rune() == 'j':
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case event.Rune() == 'k':
			return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
		}
		return event
	})

	cm.list.SetSelectedFunc(func(idx int, _ string, _ string, _ rune) {
		conns := cm.host.Config().Connections
		if idx >= 0 && idx < len(conns) {
			name := conns[idx].Name
			cm.close()
			cm.host.SwitchConnection(name)
		}
	})
	return cm
}

// SetEditor wires cm to the connection editor it opens for "new"/"edit"/
// "duplicate". Called once, right after NewConnEditor constructs the
// editor — ConnEditor doesn't exist yet when NewConnManager runs, so
// this can't be passed to the constructor itself.
func (cm *ConnManager) SetEditor(ce *ConnEditor) {
	cm.editor = ce
}

// Show opens the connection manager overlay.
func (cm *ConnManager) Show() {
	cm.populate()
	ac := cm.host.Config().Colors.Accent
	cm.hints.SetText(fmt.Sprintf(
		"[%s]<Enter>[-] activate  [%s]<n>[-] new  [%s]<e>[-] edit  [%s]<d>[-] dup  [%s]<Del/x>[-] delete  [%s]<Esc>[-] close",
		ac, ac, ac, ac, ac, ac,
	))
	cm.host.ShowPage("conn-manager")
	cm.host.SetFocus(cm.list)
	cm.visible = true
}

// close hides the connection manager overlay.
func (cm *ConnManager) close() {
	cm.host.HidePage("conn-manager")
	cm.visible = false
	cm.host.FocusMain()
}

// ApplyPalette recolors the connection manager overlay for a live theme switch.
func (cm *ConnManager) ApplyPalette(p config.Palette) {
	bg := tcell.GetColor(p.Background)
	cm.flex.SetBackgroundColor(bg)
	cm.flex.SetBorderColor(tcell.GetColor(p.Border))
	cm.flex.SetTitleColor(tcell.GetColor(p.Border))
	ui.StyleList(cm.list, p)
	cm.list.SetBackgroundColor(bg)
	cm.hints.SetBackgroundColor(bg)
	cm.hints.SetTextColor(tcell.GetColor(p.Text))
}

var _ ui.Themeable = (*ConnManager)(nil)

// Primitive returns ConnManager's root widget, for sizing/embedding.
func (cm *ConnManager) Primitive() tview.Primitive { return cm.flex }

// Visible reports whether ConnManager is currently shown.
func (cm *ConnManager) Visible() bool { return cm.visible }

// populate rebuilds the manager list from the current config.
func (cm *ConnManager) populate() {
	cm.list.Clear()
	cfg := cm.host.Config()
	for _, conn := range cfg.Connections {
		c := conn // capture per iteration
		star := "   "
		if c.Name == cfg.ActiveConnection {
			star = "⭐ "
		}
		label := fmt.Sprintf("%s%-24s (%s)", star, c.Name, c.Backend)
		cm.list.AddItem(label, "", 0, func() {
			cm.close()
			cm.host.SwitchConnection(c.Name)
		})
	}
}

// delete confirms and deletes the currently selected connection. Refuses
// if it is the only connection.
func (cm *ConnManager) delete() {
	conns := cm.host.Config().Connections
	if len(conns) <= 1 {
		cm.host.SetStatus("[yellow]Cannot delete the only connection[-]")
		return
	}
	idx := cm.list.GetCurrentItem()
	if idx < 0 || idx >= len(conns) {
		return
	}
	toDelete := conns[idx]
	cm.confirm.Show(fmt.Sprintf("Delete connection %q?", toDelete.Name), func() {
		if cm.host.DeleteConnection(toDelete.Name) {
			cm.close()
		} else {
			cm.populate()
			cm.host.SetFocus(cm.list)
		}
	})
}

// ConnEditor is the AMQ connection editor overlay, shared by "new",
// "edit", and "duplicate" from the connection manager.
type ConnEditor struct {
	host     ui.Host
	manager  *ConnManager
	form     *tview.Form
	visible  bool
	isNew    bool
	origName string
	// brokerName shadows the Broker Name field's value across a jolokia ->
	// proxy -> jolokia round trip, since the field itself doesn't exist
	// while Backend is proxy (rebuildTail has nothing to read it back from
	// otherwise) — see spec/57-bugfix-broker-name-proxy-hidden.
	brokerName string
}

// NewConnEditor builds the connection editor overlay's form.
func NewConnEditor(host ui.Host, manager *ConnManager) *ConnEditor {
	ce := &ConnEditor{host: host, manager: manager}
	ce.form = tview.NewForm()
	ce.form.SetBorder(true).SetTitle(" AMQ Connection ")
	ce.form.
		AddInputField("Name", "", 30, nil, nil).
		AddDropDown("Backend", []string{"jolokia", "proxy"}, 0, nil).
		AddInputField("Broker Name", "", 30, nil, nil).
		AddInputField("URL", "", 40, nil, nil).
		AddInputField("Username", "", 20, nil, nil).
		AddDropDown("Password Source", []string{"Plain", "AWS Secret"}, 0, nil).
		AddPasswordField("Password", "", 20, '*', nil).
		AddButton("Save", func() { ce.save() }).
		AddButton("Cancel", func() { ce.close() })
	if dd, ok := ce.form.GetFormItem(1).(*tview.DropDown); ok {
		ui.StyleDropDown(dd, host.Config().Colors)
		// Wired via SetSelectedFunc rather than passed to AddDropDown
		// itself, for the same reason as the Password Source dropdown
		// below: AddDropDown's initial SetCurrentOption(0) call would
		// otherwise fire the rebuild before the rest of the chain exists.
		dd.SetSelectedFunc(func(_ string, idx int) {
			backends := []string{"jolokia", "proxy"}
			ce.rebuildTail(backends[idx])
		})
	}
	// The Password Source dropdown swaps the last form item (before the
	// Save/Cancel buttons, which AddButton keeps separate from GetFormItem)
	// between a plain Password field and a Password Secret (AWS) field —
	// see setPasswordField. Wired via SetSelectedFunc rather than passed
	// to AddDropDown itself, since AddDropDown's initial
	// SetCurrentOption(0) call would otherwise fire the swap before the
	// Password field even exists yet. GetFormItem(5) is safe here because
	// this runs once, right after the static chain above built the default
	// (jolokia) layout — Password Source is always at index 5 at this
	// specific point, even though it can move once rebuildTail starts
	// rebuilding items after a Backend change.
	if dd, ok := ce.form.GetFormItem(5).(*tview.DropDown); ok {
		ui.StyleDropDown(dd, host.Config().Colors)
		dd.SetSelectedFunc(func(_ string, sourceIdx int) {
			ce.setPasswordField(sourceIdx)
		})
	}
	ce.form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			ce.close()
			return nil
		}
		return event
	})
	return ce
}

// Show opens the connection editor overlay. conn is pre-filled into the
// form; isNew=true means adding, false=editing. origName is the current
// name when editing (used for uniqueness validation).
func (ce *ConnEditor) Show(conn config.Connection, isNew bool, origName string) {
	ce.isNew = isNew
	ce.origName = origName

	title := " New AMQ Connection "
	if !isNew {
		title = fmt.Sprintf(" Edit — %s ", origName)
	}
	ce.form.SetTitle(title)

	// Name and Backend are always at fixed indices 0/1 — everything after
	// them is looked up by label instead, since rebuildTail can change
	// which fields exist and where.
	ce.form.GetFormItem(0).(*tview.InputField).SetText(conn.Name)
	backendIdx := 0
	if conn.Backend == "proxy" {
		backendIdx = 1
	}
	// SetCurrentOption fires the Backend dropdown's selected callback
	// (rebuildTail), rebuilding every item after it — including whether
	// Broker Name exists at all. rebuildTail's own capture step runs as
	// part of this call and will read (and shadow) whatever the form's
	// *previous* Broker Name field held — not conn's — so
	// conn.Queue.BrokerName is applied explicitly afterward instead of
	// being set beforehand (setting it first would just get clobbered by
	// that capture step).
	ce.form.GetFormItem(1).(*tview.DropDown).SetCurrentOption(backendIdx)

	// URL and credentials are backend-specific; show whichever is set.
	urlVal := conn.Queue.URL
	username := conn.Queue.Username
	password := conn.Queue.Password
	passwordSecret := conn.Queue.PasswordSecret
	passwordSecretProfile := conn.Queue.PasswordSecretAWSProfile
	if conn.Backend == "proxy" {
		urlVal = conn.Proxy.URL
		username = conn.Proxy.Username
		password = conn.Proxy.Password
		passwordSecret = conn.Proxy.PasswordSecret
		passwordSecretProfile = conn.Proxy.PasswordSecretAWSProfile
	} else {
		ce.brokerName = conn.Queue.BrokerName
		if item, ok := ce.form.GetFormItemByLabel("Broker Name").(*tview.InputField); ok {
			item.SetText(conn.Queue.BrokerName)
		}
	}
	ce.form.GetFormItemByLabel("URL").(*tview.InputField).SetText(urlVal)
	ce.form.GetFormItemByLabel("Username").(*tview.InputField).SetText(username)

	sourceIdx := 0
	if passwordSecret != "" {
		sourceIdx = 1
	}
	// SetCurrentOption fires the Password Source dropdown's selected
	// callback (setPasswordField), swapping the trailing field to match
	// before its text is set below.
	ce.form.GetFormItemByLabel("Password Source").(*tview.DropDown).SetCurrentOption(sourceIdx)
	if sourceIdx == 1 {
		ce.form.GetFormItemByLabel("Password Secret (AWS)").(*tview.InputField).SetText(passwordSecret)
		ce.form.GetFormItemByLabel("Secret AWS Profile").(*tview.InputField).SetText(passwordSecretProfile)
	} else {
		ce.form.GetFormItemByLabel("Password").(*tview.InputField).SetText(password)
	}

	ce.host.ShowPage("conn-editor")
	ce.host.SetFocus(ce.form)
	ce.visible = true
}

// setPasswordField swaps the trailing form item(s) — right before the
// Save/Cancel buttons — between a plain Password field (sourceIdx 0)
// and a Password Secret (AWS) + Secret AWS Profile pair (sourceIdx 1),
// driven by the Password Source dropdown. AddButton items aren't
// counted by GetFormItem, so these are always the last item(s)
// regardless of whether Broker Name is present (see rebuildTail).
// Whether one or two trailing items currently exist is checked (via
// whether "Secret AWS Profile" is present) rather than tracked
// separately, so this works correctly regardless of which state it's
// coming from.
func (ce *ConnEditor) setPasswordField(sourceIdx int) {
	f := ce.form
	currentCount := 1
	if _, ok := f.GetFormItemByLabel("Secret AWS Profile").(*tview.InputField); ok {
		currentCount = 2
	}
	for i := 0; i < currentCount; i++ {
		f.RemoveFormItem(f.GetFormItemCount() - 1)
	}
	if sourceIdx == 1 {
		f.AddInputField("Password Secret (AWS)", "", 30, nil, nil)
		f.AddInputField("Secret AWS Profile", "", 20, nil, nil)
	} else {
		f.AddPasswordField("Password", "", 20, '*', nil)
	}
}

// rebuildTail rebuilds every connection-editor form item after Backend
// (item 1, never removed) — Broker Name (jolokia only), URL, Username,
// Password Source, and Password/Password Secret — for the given backend.
// Broker Name only means anything for the jolokia backend (see
// spec/57-bugfix-broker-name-proxy-hidden); everything else is backend-
// agnostic and just gets rebuilt alongside it since it isn't the form's
// last item and can't be swapped in place the way setPasswordField swaps
// the trailing field.
//
// Whatever the user already typed/selected is captured via
// GetFormItemByLabel (nil-safe: an absent field just contributes its zero
// value) before anything is removed, then fed back in as each new field's
// initial value — toggling Backend mid-edit must not lose Broker Name, URL,
// Username, the Password Source choice, or the Password/Secret text.
//
// Broker Name is the one field that can't be captured this way alone: once
// Backend is proxy, the field doesn't exist, so a later jolokia->proxy->
// jolokia round trip would have nothing to read it back from and silently
// reset it to "". ce.brokerName shadows it across exactly that gap —
// updated here whenever the field exists, left untouched (and used as the
// restore value) whenever it doesn't.
func (ce *ConnEditor) rebuildTail(backend string) {
	f := ce.form

	var url, username, passwordOrSecret, secretProfile string
	sourceIdx := 0
	if item, ok := f.GetFormItemByLabel("Broker Name").(*tview.InputField); ok {
		ce.brokerName = item.GetText()
	}
	if item, ok := f.GetFormItemByLabel("URL").(*tview.InputField); ok {
		url = item.GetText()
	}
	if item, ok := f.GetFormItemByLabel("Username").(*tview.InputField); ok {
		username = item.GetText()
	}
	if dd, ok := f.GetFormItemByLabel("Password Source").(*tview.DropDown); ok {
		sourceIdx, _ = dd.GetCurrentOption()
	}
	if item, ok := f.GetFormItemByLabel("Password").(*tview.InputField); ok {
		passwordOrSecret = item.GetText()
	} else if item, ok := f.GetFormItemByLabel("Password Secret (AWS)").(*tview.InputField); ok {
		passwordOrSecret = item.GetText()
	}
	if item, ok := f.GetFormItemByLabel("Secret AWS Profile").(*tview.InputField); ok {
		secretProfile = item.GetText()
	}

	for f.GetFormItemCount() > 2 {
		f.RemoveFormItem(2)
	}

	if backend != "proxy" {
		f.AddInputField("Broker Name", ce.brokerName, 30, nil, nil)
	}
	f.AddInputField("URL", url, 40, nil, nil)
	f.AddInputField("Username", username, 20, nil, nil)
	// nil selected func here for the same reason as the initial
	// construction: wiring it before the trailing password-ish field is
	// added below would fire prematurely.
	f.AddDropDown("Password Source", []string{"Plain", "AWS Secret"}, sourceIdx, nil)
	if sourceIdx == 1 {
		f.AddInputField("Password Secret (AWS)", passwordOrSecret, 30, nil, nil)
		f.AddInputField("Secret AWS Profile", secretProfile, 20, nil, nil)
	} else {
		f.AddPasswordField("Password", passwordOrSecret, 20, '*', nil)
	}

	if dd, ok := f.GetFormItemByLabel("Password Source").(*tview.DropDown); ok {
		ui.StyleDropDown(dd, ce.host.Config().Colors)
		dd.SetSelectedFunc(func(_ string, idx int) {
			ce.setPasswordField(idx)
		})
	}
}

// close hides the editor and returns focus to the manager or pages.
func (ce *ConnEditor) close() {
	ce.host.HidePage("conn-editor")
	ce.visible = false
	if ce.manager.visible {
		ce.host.SetFocus(ce.manager.list)
	} else {
		ce.host.FocusMain()
	}
}

// ApplyPalette recolors the connection editor overlay for a live theme switch.
func (ce *ConnEditor) ApplyPalette(p config.Palette) {
	ce.form.SetBackgroundColor(tcell.GetColor(p.Background))
	ce.form.SetBorderColor(tcell.GetColor(p.Border))
	ce.form.SetTitleColor(tcell.GetColor(p.Border))
	// GetFormItem(2) is Broker Name (an InputField), never a DropDown —
	// this type assertion has been a silent no-op since Password Source
	// was added at index 5 (it originally targeted the Backend dropdown
	// before Broker Name/Password Source shifted indices around it).
	// Preserved as-is: fixing it is a behavior change, out of scope for
	// this structural move.
	if dd, ok := ce.form.GetFormItem(2).(*tview.DropDown); ok {
		ui.StyleDropDown(dd, p)
	}
}

var _ ui.Themeable = (*ConnEditor)(nil)

// Primitive returns ConnEditor's root widget, for sizing/embedding.
func (ce *ConnEditor) Primitive() tview.Primitive { return ce.form }

// Visible reports whether ConnEditor is currently shown.
func (ce *ConnEditor) Visible() bool { return ce.visible }

// save validates and persists the editor form, then closes it.
func (ce *ConnEditor) save() {
	name := strings.TrimSpace(ce.form.GetFormItem(0).(*tview.InputField).GetText())
	backendIdx, _ := ce.form.GetFormItem(1).(*tview.DropDown).GetCurrentOption()
	backends := []string{"jolokia", "proxy"}
	backend := backends[backendIdx]
	var brokerName string
	if item, ok := ce.form.GetFormItemByLabel("Broker Name").(*tview.InputField); ok {
		brokerName = item.GetText()
	}
	urlVal := ce.form.GetFormItemByLabel("URL").(*tview.InputField).GetText()
	username := ce.form.GetFormItemByLabel("Username").(*tview.InputField).GetText()
	sourceIdx, _ := ce.form.GetFormItemByLabel("Password Source").(*tview.DropDown).GetCurrentOption()
	var password, passwordSecret, passwordSecretProfile string
	if sourceIdx == 1 {
		passwordSecret = strings.TrimSpace(ce.form.GetFormItemByLabel("Password Secret (AWS)").(*tview.InputField).GetText())
		passwordSecretProfile = strings.TrimSpace(ce.form.GetFormItemByLabel("Secret AWS Profile").(*tview.InputField).GetText())
	} else {
		password = ce.form.GetFormItemByLabel("Password").(*tview.InputField).GetText()
	}

	if name == "" {
		ce.host.SetStatus("[red]Name is required[-]")
		return
	}
	if sourceIdx == 1 && passwordSecretProfile == "" {
		ce.host.SetStatus("[red]AWS Profile is required when Password Source is AWS Secret[-]")
		return
	}
	for _, c := range ce.host.Config().Connections {
		if c.Name == name && c.Name != ce.origName {
			ce.host.SetStatus(fmt.Sprintf("[red]Connection %q already exists[-]", name))
			return
		}
	}

	conn := config.Connection{Name: name, Backend: backend}
	if backend == "proxy" {
		conn.Proxy = config.ProxyConfig{URL: urlVal, Username: username, Password: password, PasswordSecret: passwordSecret, PasswordSecretAWSProfile: passwordSecretProfile}
	} else {
		conn.Queue = config.QueueConfig{BrokerName: brokerName, URL: urlVal, Username: username, Password: password, PasswordSecret: passwordSecret, PasswordSecretAWSProfile: passwordSecretProfile}
	}

	ce.host.SaveConnection(conn, ce.origName, ce.isNew)
	ce.close()
	ce.manager.populate()
}
