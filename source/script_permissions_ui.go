package main

import (
	"crypto/sha256"
	"gothoom/eui"
)

var scriptPermissionsWin *eui.WindowData
var scriptPermissionsOwner string
var scriptPermissionRequests []string

// Queue first-enable and newly requested access reviews without replacing a
// dialog the player is already reading. No script code runs during this scan.
func queueScriptPermissionReview(owner string) {
	if scriptPermissionsWin != nil && scriptPermissionsOwner == owner {
		return
	}
	for _, pending := range scriptPermissionRequests {
		if pending == owner {
			return
		}
	}
	scriptPermissionRequests = append(scriptPermissionRequests, owner)
	showNextScriptPermissionReview()
}

func showNextScriptPermissionReview() {
	if scriptPermissionsWin != nil {
		return
	}
	for len(scriptPermissionRequests) > 0 {
		owner := scriptPermissionRequests[0]
		scriptPermissionRequests = scriptPermissionRequests[1:]
		scriptMu.RLock()
		info, exists := scriptPackages[owner]
		selected := !scriptEnabledFor[owner].empty()
		scriptMu.RUnlock()
		if exists && selected && checkScriptPermissions(owner, info.src) != nil {
			openScriptPermissionsWindow(owner)
			return
		}
	}
}

func openScriptPermissionsWindow(owner string) {
	info, err := scriptPackageForPermissionReview(owner)
	if err != nil {
		recordScriptError(owner, err.Error(), scriptIsRunning(owner))
		consoleMessage("[script] permissions: " + err.Error())
		refreshscriptsWindow()
		return
	}
	scriptMu.RLock()
	name := scriptDisplayNames[owner]
	scope := scriptEnabledFor[owner]
	scriptMu.RUnlock()
	if name == "" {
		name = owner
	}
	if scriptPermissionsWin != nil {
		scriptPermissionsWin.Close()
	}
	win := eui.NewWindow()
	scriptPermissionsWin = win
	scriptPermissionsOwner = owner
	win.Title = "Permissions: " + name
	win.Closable, win.Movable, win.AutoSize = true, true, true
	win.Resizable = false
	win.SetZone(eui.HZoneCenterLeft, eui.VZoneMiddleTop)
	removed := false
	win.OnClose = func() {
		if removed {
			return
		}
		removed = true
		win.RemoveWindow()
		if scriptPermissionsWin == win {
			scriptPermissionsWin = nil
			scriptPermissionsOwner = ""
			dispatchScriptControl(showNextScriptPermissionReview)
		}
	}
	root := eui.NewColumn()
	win.AddItem(root)
	line := func(value string) *eui.ItemData {
		label, _ := eui.NewText()
		label.Text, label.FontSize = value, 12
		label.Size = eui.Point{X: 520, Y: 20}
		root.AddItem(label)
		return label
	}
	label := line("Enabled for: " + scriptScopeDescription(scope))
	label.SetTooltip("Saved script enablement: " + scriptScopeDescription(scope))
	line("Grants apply across characters. Unchecked capabilities stay blocked.")
	line("Greyed out: this source has no API references for that permission.")
	required, scanErr := scriptRequiredPermissions(info.src)
	selected := map[string]bool{}
	for _, permission := range scriptPermissionCatalog {
		cb, events := eui.NewCheckbox()
		cb.Text, cb.FontSize = permission.Label, 12
		cb.Size = eui.Point{X: 520, Y: 24}
		cb.Disabled = scanErr != nil || !required[permission.ID]
		scriptPermissionMu.RLock()
		reviewed := scriptPermissionReviews[owner][permission.ID]
		scriptPermissionMu.RUnlock()
		cb.Checked = !cb.Disabled && (!reviewed || scriptHasPermission(owner, permission.ID))
		cb.SetTooltip(permission.Help)
		selected[permission.ID] = cb.Checked
		events.Handle = func(event eui.UIEvent) {
			if event.Type == eui.EventCheckboxChanged && !cb.Disabled {
				selected[permission.ID] = event.Checked
			}
		}
		root.AddItem(cb)
	}
	status := line("")
	if scanErr != nil {
		status.Text = "Fix the source syntax before changing permissions."
	}
	row := eui.NewRow()
	root.AddItem(row)
	apply, applyEvents := eui.NewButton()
	apply.Text, apply.Size = "Grant", eui.Point{X: 100, Y: 28}
	apply.Disabled = scanErr != nil
	save := func(blockAll bool) {
		if apply.Disabled {
			return
		}
		current, err := scriptPackageForPermissionReview(owner)
		if err != nil {
			status.Text = "Could not read the current script. See the tooltip for details."
			status.SetTooltip(err.Error())
			win.Refresh()
			return
		}
		if sha256.Sum256(current.src) != sha256.Sum256(info.src) {
			status.Text = "Script changed. Reopen Permissions to review its current access."
			win.Refresh()
			return
		}
		if err := applyScriptPermissions(owner, selected, blockAll); err != nil {
			status.Text = "Could not save permissions. See the tooltip for details."
			status.SetTooltip(err.Error())
			win.Refresh()
			return
		}
		win.Close()
	}
	applyEvents.Handle = func(event eui.UIEvent) {
		if event.Type == eui.EventClick {
			save(false)
		}
	}
	row.AddItem(apply)
	cancel, cancelEvents := eui.NewButton()
	cancel.Text, cancel.Size = "Block all", eui.Point{X: 100, Y: 28}
	cancelEvents.Handle = func(event eui.UIEvent) {
		if event.Type == eui.EventClick {
			save(true)
		}
	}
	row.AddItem(cancel)
	win.AddWindow(false)
	win.MarkOpen()
	win.Refresh()
}
