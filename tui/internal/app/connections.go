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
		label := fmt.Sprintf("%s%-8s %-24s (%s)", star, c.Alias, c.Name, c.Backend)
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

	// Prefill form fields (indices match AddInputField/AddDropDown order in New()).
	a.connEditorForm.GetFormItem(0).(*tview.InputField).SetText(conn.Name)
	a.connEditorForm.GetFormItem(1).(*tview.InputField).SetText(conn.Alias)
	backendIdx := 0
	if conn.Backend == "proxy" {
		backendIdx = 1
	}
	a.connEditorForm.GetFormItem(2).(*tview.DropDown).SetCurrentOption(backendIdx)
	a.connEditorForm.GetFormItem(3).(*tview.InputField).SetText(conn.Queue.BrokerName)
	// URL and credentials are backend-specific; show whichever is set.
	urlVal := conn.Queue.URL
	username := conn.Queue.Username
	password := conn.Queue.Password
	if conn.Backend == "proxy" {
		urlVal = conn.Proxy.URL
		username = conn.Proxy.Username
		password = conn.Proxy.Password
	}
	a.connEditorForm.GetFormItem(4).(*tview.InputField).SetText(urlVal)
	a.connEditorForm.GetFormItem(5).(*tview.InputField).SetText(username)
	a.connEditorForm.GetFormItem(6).(*tview.InputField).SetText(password)

	a.rootPages.ShowPage("conn-editor")
	a.tv.SetFocus(a.connEditorForm)
	a.connEditorVisible = true
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
	alias := strings.TrimSpace(a.connEditorForm.GetFormItem(1).(*tview.InputField).GetText())
	backendIdx, _ := a.connEditorForm.GetFormItem(2).(*tview.DropDown).GetCurrentOption()
	backends := []string{"jolokia", "proxy"}
	backend := backends[backendIdx]
	brokerName := a.connEditorForm.GetFormItem(3).(*tview.InputField).GetText()
	urlVal := a.connEditorForm.GetFormItem(4).(*tview.InputField).GetText()
	username := a.connEditorForm.GetFormItem(5).(*tview.InputField).GetText()
	password := a.connEditorForm.GetFormItem(6).(*tview.InputField).GetText()

	if name == "" || alias == "" {
		a.statusBar.SetText("[red]Name and alias are required[-]")
		return
	}
	for _, c := range a.cfg.Connections {
		if c.Name == name && c.Name != a.connEditorOrigName {
			a.statusBar.SetText(fmt.Sprintf("[red]Connection %q already exists[-]", name))
			return
		}
	}

	conn := config.Connection{Name: name, Alias: alias, Backend: backend}
	if backend == "proxy" {
		conn.Proxy = config.ProxyConfig{URL: urlVal, Username: username, Password: password}
	} else {
		conn.Queue = config.QueueConfig{BrokerName: brokerName, URL: urlVal, Username: username, Password: password}
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
			a.backend = newBackendForConn(conn)
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
