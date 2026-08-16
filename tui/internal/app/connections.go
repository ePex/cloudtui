package app

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/rivo/tview"

	"github.com/ePex/cloudtui/tui/internal/config"
)

// showConnectionManager opens the connection manager overlay.
func (a *App) showConnectionManager() {
	a.populateConnManagerList()
	ac := a.cfg.Colors.Accent
	a.connManagerHints.SetText(fmt.Sprintf(
		"[%s]<Enter>[-] activate  [%s]<n>[-] new  [%s]<e>[-] edit  [%s]<d>[-] dup  [%s]<Del/x>[-] delete  [%s]<Esc>[-] close",
		ac, ac, ac, ac, ac, ac,
	))
	a.rootPages.ShowPage("conn-manager")
	a.tv.SetFocus(a.connManagerList)
	a.connManagerVisible = true
}

// closeConnManager hides the connection manager overlay.
func (a *App) closeConnManager() {
	a.rootPages.HidePage("conn-manager")
	a.connManagerVisible = false
	a.tv.SetFocus(a.pages)
}

// populateConnManagerList rebuilds the manager list from the current config.
func (a *App) populateConnManagerList() {
	a.connManagerList.Clear()
	for _, conn := range a.cfg.Connections {
		c := conn // capture per iteration
		star := "   "
		if c.Name == a.cfg.ActiveConnection {
			star = "⭐ "
		}
		label := fmt.Sprintf("%s%-24s (%s)", star, c.Name, c.Backend)
		a.connManagerList.AddItem(label, "", 0, func() {
			a.closeConnManager()
			a.switchConnection(c.Name)
		})
	}
}

// showConnEditor opens the connection editor overlay.
// conn is pre-filled into the form; isNew=true means adding, false=editing.
// origName is the current name when editing (used for uniqueness validation).
func (a *App) showConnEditor(conn config.Connection, isNew bool, origName string) {
	a.connEditorIsNew = isNew
	a.connEditorOrigName = origName

	title := " New AMQ Connection "
	if !isNew {
		title = fmt.Sprintf(" Edit — %s ", origName)
	}
	a.connEditorForm.SetTitle(title)

	// Name and Backend are always at fixed indices 0/1 — everything after
	// them is looked up by label instead, since rebuildConnEditorTail can
	// change which fields exist and where.
	a.connEditorForm.GetFormItem(0).(*tview.InputField).SetText(conn.Name)
	backendIdx := 0
	if conn.Backend == "proxy" {
		backendIdx = 1
	}
	// SetCurrentOption fires the Backend dropdown's selected callback
	// (rebuildConnEditorTail), rebuilding every item after it — including
	// whether Broker Name exists at all. rebuildConnEditorTail's own
	// capture step runs as part of this call and will read (and shadow)
	// whatever the form's *previous* Broker Name field held — not conn's —
	// so conn.Queue.BrokerName is applied explicitly afterward instead of
	// being set beforehand (setting it first would just get clobbered by
	// that capture step).
	a.connEditorForm.GetFormItem(1).(*tview.DropDown).SetCurrentOption(backendIdx)

	// URL and credentials are backend-specific; show whichever is set.
	urlVal := conn.Queue.URL
	username := conn.Queue.Username
	password := conn.Queue.Password
	passwordSecret := conn.Queue.PasswordSecret
	if conn.Backend == "proxy" {
		urlVal = conn.Proxy.URL
		username = conn.Proxy.Username
		password = conn.Proxy.Password
		passwordSecret = conn.Proxy.PasswordSecret
	} else {
		a.connEditorBrokerName = conn.Queue.BrokerName
		if item, ok := a.connEditorForm.GetFormItemByLabel("Broker Name").(*tview.InputField); ok {
			item.SetText(conn.Queue.BrokerName)
		}
	}
	a.connEditorForm.GetFormItemByLabel("URL").(*tview.InputField).SetText(urlVal)
	a.connEditorForm.GetFormItemByLabel("Username").(*tview.InputField).SetText(username)

	sourceIdx := 0
	if passwordSecret != "" {
		sourceIdx = 1
	}
	// SetCurrentOption fires the Password Source dropdown's selected
	// callback (setConnEditorPasswordField), swapping the trailing field to
	// match before its text is set below.
	a.connEditorForm.GetFormItemByLabel("Password Source").(*tview.DropDown).SetCurrentOption(sourceIdx)
	if sourceIdx == 1 {
		a.connEditorForm.GetFormItemByLabel("Password Secret (AWS)").(*tview.InputField).SetText(passwordSecret)
	} else {
		a.connEditorForm.GetFormItemByLabel("Password").(*tview.InputField).SetText(password)
	}

	a.rootPages.ShowPage("conn-editor")
	a.tv.SetFocus(a.connEditorForm)
	a.connEditorVisible = true
}

// setConnEditorPasswordField swaps the last form item — the item right
// before the Save/Cancel buttons — between a plain Password field
// (sourceIdx 0) and a Password Secret (AWS) field (sourceIdx 1), driven by
// the Password Source dropdown. AddButton items aren't counted by
// GetFormItem, so the last form item is always the password-ish field
// regardless of which one is showing or whether Broker Name is present
// (see rebuildConnEditorTail) — hence computing the index instead of
// assuming a fixed one.
func (a *App) setConnEditorPasswordField(sourceIdx int) {
	f := a.connEditorForm
	f.RemoveFormItem(f.GetFormItemCount() - 1)
	if sourceIdx == 1 {
		f.AddInputField("Password Secret (AWS)", "", 30, nil, nil)
	} else {
		f.AddPasswordField("Password", "", 20, '*', nil)
	}
}

// rebuildConnEditorTail rebuilds every connection-editor form item after
// Backend (item 1, never removed) — Broker Name (jolokia only), URL,
// Username, Password Source, and Password/Password Secret — for the given
// backend. Broker Name only means anything for the jolokia backend (see
// spec/57-bugfix-broker-name-proxy-hidden); everything else is backend-
// agnostic and just gets rebuilt alongside it since it isn't the form's
// last item and can't be swapped in place the way
// setConnEditorPasswordField swaps the trailing field.
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
// reset it to "". a.connEditorBrokerName shadows it across exactly that
// gap — updated here whenever the field exists, left untouched (and used
// as the restore value) whenever it doesn't.
func (a *App) rebuildConnEditorTail(backend string) {
	f := a.connEditorForm

	var url, username, passwordOrSecret string
	sourceIdx := 0
	if item, ok := f.GetFormItemByLabel("Broker Name").(*tview.InputField); ok {
		a.connEditorBrokerName = item.GetText()
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

	for f.GetFormItemCount() > 2 {
		f.RemoveFormItem(2)
	}

	if backend != "proxy" {
		f.AddInputField("Broker Name", a.connEditorBrokerName, 30, nil, nil)
	}
	f.AddInputField("URL", url, 40, nil, nil)
	f.AddInputField("Username", username, 20, nil, nil)
	// nil selected func here for the same reason as the initial
	// construction in app.go: wiring it before the trailing password-ish
	// field is added below would fire prematurely.
	f.AddDropDown("Password Source", []string{"Plain", "AWS Secret"}, sourceIdx, nil)
	if sourceIdx == 1 {
		f.AddInputField("Password Secret (AWS)", passwordOrSecret, 30, nil, nil)
	} else {
		f.AddPasswordField("Password", passwordOrSecret, 20, '*', nil)
	}

	if dd, ok := f.GetFormItemByLabel("Password Source").(*tview.DropDown); ok {
		styleDropDown(dd, a.cfg.Colors)
		dd.SetSelectedFunc(func(_ string, idx int) {
			a.setConnEditorPasswordField(idx)
		})
	}
}

// closeConnEditor hides the editor and returns focus to the manager or pages.
func (a *App) closeConnEditor() {
	a.rootPages.HidePage("conn-editor")
	a.connEditorVisible = false
	if a.connManagerVisible {
		a.tv.SetFocus(a.connManagerList)
	} else {
		a.tv.SetFocus(a.pages)
	}
}

// saveConnEditor validates and persists the editor form, then closes it.
func (a *App) saveConnEditor() {
	name := strings.TrimSpace(a.connEditorForm.GetFormItem(0).(*tview.InputField).GetText())
	backendIdx, _ := a.connEditorForm.GetFormItem(1).(*tview.DropDown).GetCurrentOption()
	backends := []string{"jolokia", "proxy"}
	backend := backends[backendIdx]
	var brokerName string
	if item, ok := a.connEditorForm.GetFormItemByLabel("Broker Name").(*tview.InputField); ok {
		brokerName = item.GetText()
	}
	urlVal := a.connEditorForm.GetFormItemByLabel("URL").(*tview.InputField).GetText()
	username := a.connEditorForm.GetFormItemByLabel("Username").(*tview.InputField).GetText()
	sourceIdx, _ := a.connEditorForm.GetFormItemByLabel("Password Source").(*tview.DropDown).GetCurrentOption()
	var password, passwordSecret string
	if sourceIdx == 1 {
		passwordSecret = strings.TrimSpace(a.connEditorForm.GetFormItemByLabel("Password Secret (AWS)").(*tview.InputField).GetText())
	} else {
		password = a.connEditorForm.GetFormItemByLabel("Password").(*tview.InputField).GetText()
	}

	if name == "" {
		a.statusBar.SetText("[red]Name is required[-]")
		return
	}
	for _, c := range a.cfg.Connections {
		if c.Name == name && c.Name != a.connEditorOrigName {
			a.statusBar.SetText(fmt.Sprintf("[red]Connection %q already exists[-]", name))
			return
		}
	}

	conn := config.Connection{Name: name, Backend: backend}
	if backend == "proxy" {
		conn.Proxy = config.ProxyConfig{URL: urlVal, Username: username, Password: password, PasswordSecret: passwordSecret}
	} else {
		conn.Queue = config.QueueConfig{BrokerName: brokerName, URL: urlVal, Username: username, Password: password, PasswordSecret: passwordSecret}
	}

	wasActive := a.cfg.ActiveConnection == a.connEditorOrigName

	if a.connEditorIsNew {
		a.cfg.Connections = append(a.cfg.Connections, conn)
	} else {
		for i, c := range a.cfg.Connections {
			if c.Name == a.connEditorOrigName {
				a.cfg.Connections[i] = conn
				break
			}
		}
		if wasActive {
			a.cfg.ActiveConnection = name
			a.backend = newBackendForConn(a, conn)
			a.queuesV.backend = a.backend
			a.infoPanel.SetText(infoPanelText(a.cfg))
		}
	}

	if err := config.SaveDefault(a.cfg); err != nil {
		slog.Error("saveConnEditor: save failed", "error", err)
	}
	a.refreshSettingsList()
	a.closeConnEditor()
	a.populateConnManagerList()
}

// deleteConnFromManager confirms and deletes the currently selected connection.
// Refuses if it is the only connection.
func (a *App) deleteConnFromManager() {
	if len(a.cfg.Connections) <= 1 {
		a.statusBar.SetText("[yellow]Cannot delete the only connection[-]")
		return
	}
	idx := a.connManagerList.GetCurrentItem()
	if idx < 0 || idx >= len(a.cfg.Connections) {
		return
	}
	toDelete := a.cfg.Connections[idx]
	a.showConfirm(fmt.Sprintf("Delete connection %q?", toDelete.Name), func() {
		wasActive := a.cfg.ActiveConnection == toDelete.Name
		conns := make([]config.Connection, 0, len(a.cfg.Connections)-1)
		for _, c := range a.cfg.Connections {
			if c.Name != toDelete.Name {
				conns = append(conns, c)
			}
		}
		a.cfg.Connections = conns
		if wasActive {
			a.cfg.ActiveConnection = a.cfg.Connections[0].Name
			a.closeConnManager()
			a.switchConnection(a.cfg.ActiveConnection)
		} else {
			if err := config.SaveDefault(a.cfg); err != nil {
				slog.Error("deleteConn: save failed", "error", err)
			}
			a.populateConnManagerList()
			a.tv.SetFocus(a.connManagerList)
		}
	})
}
