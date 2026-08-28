package dialog

import (
	"context"
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

// labelPassword, labelSecretName, and labelAWSProfile carry a 2-space
// indent so the field conditionally shown below "Authentication Mode"
// (exactly one of these three, depending on its value) reads as
// visually nested under it, rather than a peer of Name/Backend/URL/etc.
// labelAuthenticationMode itself isn't indented — it's a top-level Auth
// section field, just renamed from its original "Password Source".
// Defined once and reused everywhere these labels are created or looked
// up via GetFormItemByLabel, since GetFormItemByLabel matches the label
// string exactly (including the indent) — a literal without it would
// silently fail to find the field.
const (
	labelAuthenticationMode = "Authentication Mode"
	labelPassword           = "  Password"
	labelSecretName         = "  Secret Name"
	labelAWSProfile         = "  AWS Profile"
)

// sectionGeneral, sectionDestination, and sectionAuth are non-interactive
// section-header rows in the connection editor form, added via
// tview.Form's AddTextView (scrollable=false makes a TextView
// non-focusable when embedded in a Form — see TextView.Focus(), which
// immediately replays the last Tab/Backtab instead of stopping there).
// Held in the TextView's own label slot (not its body text) so they
// print flush against the form's left edge like every other field's
// label, rather than indented to align with the value column the way a
// body-text TextView would be (Form reserves the same label-width
// column for every item in a vertical form, uniformly, regardless of
// that item's own label length).
const (
	sectionGeneral     = "── General ──"
	sectionDestination = "── Destination ──"
	sectionAuth        = "── Auth ──"
)

// staticPrefixItemCount is the number of form items rebuildTail leaves
// alone at the front of the form: the General section header, Name, the
// Destination section header, and Backend. Everything from this index
// onward is backend-dependent and gets rebuilt by rebuildTail.
const staticPrefixItemCount = 4

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
	// awsProfileNames feeds the "AWS Profile" field's autocomplete —
	// refreshed once per Show() (see awsprofiles.go's own populate(), which
	// re-reads the same discovery source on every open without caching
	// across opens either) rather than per keystroke, so the AWS
	// Profile field's SetAutocompleteFunc callback stays cheap.
	awsProfileNames []string
}

// NewConnEditor builds the connection editor overlay's form: a General
// section (Name), a Destination section (Backend, plus whatever
// rebuildTail adds after it for that backend), and — built by the
// initial rebuildTail call below — an Auth section (Username,
// Authentication Mode, and whichever field(s) that mode implies).
func NewConnEditor(host ui.Host, manager *ConnManager) *ConnEditor {
	ce := &ConnEditor{host: host, manager: manager}
	ce.form = tview.NewForm()
	ce.form.SetBorder(true).SetTitle(" AMQ Connection ")
	ce.form.
		AddTextView(sectionGeneral, "", 0, 1, false, false).
		AddInputField("Name", "", 30, nil, nil).
		AddTextView(sectionDestination, "", 0, 1, false, false).
		AddDropDown("Backend", []string{"jolokia", "proxy"}, 0, nil).
		AddButton("Save", func() { ce.save() }).
		AddButton("Cancel", func() { ce.close() })
	if dd, ok := ce.form.GetFormItemByLabel("Backend").(*tview.DropDown); ok {
		ui.StyleDropDown(dd, host.Config().Colors)
		// Wired via SetSelectedFunc rather than passed to AddDropDown
		// itself: AddDropDown's initial SetCurrentOption(0) call would
		// otherwise fire the rebuild before the rest of the chain exists.
		dd.SetSelectedFunc(func(_ string, idx int) {
			backends := []string{"jolokia", "proxy"}
			ce.rebuildTail(backends[idx])
		})
	}
	ce.form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			ce.close()
			return nil
		}
		return event
	})
	// Builds everything from staticPrefixItemCount onward (Broker Name/
	// URL/Auth header/Username/Authentication Mode/Password) for the
	// default jolokia backend — including wiring Authentication Mode's
	// own SetSelectedFunc, which rebuildTail already does at its own end.
	ce.rebuildTail("jolokia")
	return ce
}

// Show opens the connection editor overlay. conn is pre-filled into the
// form; isNew=true means adding, false=editing. origName is the current
// name when editing (used for uniqueness validation).
func (ce *ConnEditor) Show(conn config.Connection, isNew bool, origName string) {
	ce.isNew = isNew
	ce.origName = origName
	ce.loadAWSProfileNames()

	title := " New AMQ Connection "
	if !isNew {
		title = fmt.Sprintf(" Edit — %s ", origName)
	}
	ce.form.SetTitle(title)

	ce.form.GetFormItemByLabel("Name").(*tview.InputField).SetText(conn.Name)
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
	ce.form.GetFormItemByLabel("Backend").(*tview.DropDown).SetCurrentOption(backendIdx)

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
	// SetCurrentOption fires Authentication Mode's selected callback
	// (setPasswordField), swapping the trailing field to match before
	// its text is set below.
	ce.form.GetFormItemByLabel(labelAuthenticationMode).(*tview.DropDown).SetCurrentOption(sourceIdx)
	if sourceIdx == 1 {
		profileItem := ce.form.GetFormItemByLabel(labelAWSProfile).(*tview.InputField)
		profileItem.SetText(passwordSecretProfile)
		// SetText doesn't itself refresh an active SetAutocompleteFunc
		// drop-down — without this, the field keeps whatever suggestions
		// were current at wireAWSProfileAutocomplete's eager wiring call
		// (fired above by SetCurrentOption, while the field was still
		// empty), pre-selecting an unrelated entry. Left uncorrected, Tab
		// out of this field (see setPasswordField/tview's own Tab-selects-
		// current-entry behavior) would silently replace the just-set
		// profile with that stale selection — same fix as MessageFilter's
		// jmsTypeItem in messagefilter.go's own Show().
		profileItem.Autocomplete()
		ce.form.GetFormItemByLabel(labelSecretName).(*tview.InputField).SetText(passwordSecret)
	} else {
		ce.form.GetFormItemByLabel(labelPassword).(*tview.InputField).SetText(password)
	}

	// The form remembers its focusedElement index across Show() calls
	// and defaults to 0 on a brand-new instance — now the non-focusable
	// "General" section header (TextView.Focus() handles landing there
	// gracefully by replaying the last Tab/Backtab, but there is none yet
	// on a fresh editor, so focus would otherwise appear stuck there).
	// Forcing it onto Name explicitly avoids relying on that fallback.
	ce.form.SetFocus(ce.form.GetFormItemIndex("Name"))

	ce.host.ShowPage("conn-editor")
	ce.host.SetFocus(ce.form)
	ce.visible = true
}

// setPasswordField swaps the trailing form item(s) — right before the
// Save/Cancel buttons — between a plain Password field (sourceIdx 0)
// and an AWS Profile + Secret Name pair (sourceIdx 1), driven by
// Authentication Mode. AddButton items aren't counted by GetFormItem,
// so these are always the last item(s) regardless of whether Broker
// Name is present (see rebuildTail). Whether one or two trailing items
// currently exist is checked (via whether "AWS Profile" is present)
// rather than tracked separately, so this works correctly regardless of
// which state it's coming from.
func (ce *ConnEditor) setPasswordField(sourceIdx int) {
	f := ce.form
	currentCount := 1
	if _, ok := f.GetFormItemByLabel(labelAWSProfile).(*tview.InputField); ok {
		currentCount = 2
	}
	for i := 0; i < currentCount; i++ {
		f.RemoveFormItem(f.GetFormItemCount() - 1)
	}
	if sourceIdx == 1 {
		f.AddInputField(labelAWSProfile, "", 20, nil, nil)
		f.AddInputField(labelSecretName, "", 30, nil, nil)
		ce.wireAWSProfileAutocomplete()
	} else {
		f.AddPasswordField(labelPassword, "", 20, '*', nil)
	}
}

// loadAWSProfileNames refreshes the cached names behind the "AWS
// Profile" field's autocomplete. Called once per Show(), mirroring
// AWSProfilesPicker.populate()'s own re-discovery on every open — a
// failed lookup (e.g. no AWS config file) just leaves autocomplete with
// nothing to suggest rather than surfacing an error inline, since the
// field stays valid freeform text either way.
func (ce *ConnEditor) loadAWSProfileNames() {
	profiles, err := ce.host.ListAWSProfiles(context.Background())
	if err != nil {
		ce.awsProfileNames = nil
		return
	}
	names := make([]string, len(profiles))
	for i, p := range profiles {
		names[i] = p.Name
	}
	ce.awsProfileNames = names
}

// awsProfileSuggestions returns the cached AWS profile names prefixed by
// currentText, for the "AWS Profile" field's SetAutocompleteFunc.
func (ce *ConnEditor) awsProfileSuggestions(currentText string) []string {
	var matches []string
	for _, name := range ce.awsProfileNames {
		if strings.HasPrefix(name, currentText) {
			matches = append(matches, name)
		}
	}
	return matches
}

// wireAWSProfileAutocomplete styles and attaches awsProfileSuggestions to
// the "AWS Profile" field. Must run after the field is added and styling
// must precede SetAutocompleteFunc — SetAutocompleteFunc eagerly calls
// Autocomplete() once, which lazily builds the drop-down and bakes in
// whatever autocompleteStyles are set at that exact moment (see the
// ':' prompt's own gotcha in app.go/jmstypeprompt.go/messagefilter.go).
func (ce *ConnEditor) wireAWSProfileAutocomplete() {
	item, ok := ce.form.GetFormItemByLabel(labelAWSProfile).(*tview.InputField)
	if !ok {
		return
	}
	ui.StyleInputFieldAutocomplete(item, ce.host.Config().Colors)
	item.SetAutocompleteFunc(ce.awsProfileSuggestions)
}

// rebuildTail rebuilds every connection-editor form item from
// staticPrefixItemCount onward — Broker Name (jolokia only), URL, the
// Auth section header, Username, Authentication Mode, and Password/
// AWS Profile+Secret Name — for the given backend. Broker Name only
// means anything for the jolokia backend (see
// spec/57-bugfix-broker-name-proxy-hidden); everything else is backend-
// agnostic and just gets rebuilt alongside it since none of it can be
// swapped in place the way setPasswordField swaps the trailing field
// alone. Also called once, directly, by NewConnEditor to build this
// same tail for the form's initial (jolokia) state, rather than
// duplicating this same field list in a separate static chain there.
//
// Whatever the user already typed/selected is captured via
// GetFormItemByLabel (nil-safe: an absent field just contributes its zero
// value) before anything is removed, then fed back in as each new field's
// initial value — toggling Backend mid-edit must not lose Broker Name, URL,
// Username, the Authentication Mode choice, or the Password/Secret text.
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
	if dd, ok := f.GetFormItemByLabel(labelAuthenticationMode).(*tview.DropDown); ok {
		sourceIdx, _ = dd.GetCurrentOption()
	}
	if item, ok := f.GetFormItemByLabel(labelPassword).(*tview.InputField); ok {
		passwordOrSecret = item.GetText()
	} else if item, ok := f.GetFormItemByLabel(labelSecretName).(*tview.InputField); ok {
		passwordOrSecret = item.GetText()
	}
	if item, ok := f.GetFormItemByLabel(labelAWSProfile).(*tview.InputField); ok {
		secretProfile = item.GetText()
	}

	for f.GetFormItemCount() > staticPrefixItemCount {
		f.RemoveFormItem(staticPrefixItemCount)
	}

	if backend != "proxy" {
		f.AddInputField("Broker Name", ce.brokerName, 30, nil, nil)
	}
	f.AddInputField("URL", url, 40, nil, nil)
	f.AddTextView(sectionAuth, "", 0, 1, false, false)
	f.AddInputField("Username", username, 20, nil, nil)
	// nil selected func here for the same reason as the initial
	// construction: wiring it before the trailing password-ish field is
	// added below would fire prematurely.
	f.AddDropDown(labelAuthenticationMode, []string{"Plain", "AWS Secret"}, sourceIdx, nil)
	if sourceIdx == 1 {
		f.AddInputField(labelAWSProfile, secretProfile, 20, nil, nil)
		f.AddInputField(labelSecretName, passwordOrSecret, 30, nil, nil)
		ce.wireAWSProfileAutocomplete()
	} else {
		f.AddPasswordField(labelPassword, passwordOrSecret, 20, '*', nil)
	}

	if dd, ok := f.GetFormItemByLabel(labelAuthenticationMode).(*tview.DropDown); ok {
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
	// Looked up by label rather than a fixed index — this used to target
	// GetFormItem(2), which had been a silent no-op since Password
	// Source was added at index 5 shifted Broker Name off index 2 (see
	// git history); fixed as part of moving every other fixed-index
	// lookup in this file to GetFormItemByLabel for the section-header
	// restructuring, since Backend's index moves around even more now.
	if dd, ok := ce.form.GetFormItemByLabel("Backend").(*tview.DropDown); ok {
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
	name := strings.TrimSpace(ce.form.GetFormItemByLabel("Name").(*tview.InputField).GetText())
	backendIdx, _ := ce.form.GetFormItemByLabel("Backend").(*tview.DropDown).GetCurrentOption()
	backends := []string{"jolokia", "proxy"}
	backend := backends[backendIdx]
	var brokerName string
	if item, ok := ce.form.GetFormItemByLabel("Broker Name").(*tview.InputField); ok {
		brokerName = item.GetText()
	}
	urlVal := ce.form.GetFormItemByLabel("URL").(*tview.InputField).GetText()
	username := ce.form.GetFormItemByLabel("Username").(*tview.InputField).GetText()
	sourceIdx, _ := ce.form.GetFormItemByLabel(labelAuthenticationMode).(*tview.DropDown).GetCurrentOption()
	var password, passwordSecret, passwordSecretProfile string
	if sourceIdx == 1 {
		passwordSecret = strings.TrimSpace(ce.form.GetFormItemByLabel(labelSecretName).(*tview.InputField).GetText())
		passwordSecretProfile = strings.TrimSpace(ce.form.GetFormItemByLabel(labelAWSProfile).(*tview.InputField).GetText())
	} else {
		password = ce.form.GetFormItemByLabel(labelPassword).(*tview.InputField).GetText()
	}

	if name == "" {
		ce.host.SetStatus("[red]Name is required[-]")
		return
	}
	if sourceIdx == 1 && passwordSecretProfile == "" {
		ce.host.SetStatus("[red]AWS Profile is required when Authentication Mode is AWS Secret[-]")
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
