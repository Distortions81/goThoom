package main

import (
	"bytes"
	"fmt"
	"image/png"
	"reflect"
	"sort"
	"strings"
	"sync"

	"gothoom/eui"
	scriptapi "gt2"

	"github.com/hajimehoshi/ebiten/v2"
)

const maxScriptToolbarButtons = 8

type scriptToolbarButton struct {
	label   string
	tooltip string
	key     string
	image   *ebiten.Image
	onClick func()
}

type scriptToolbarRegistration struct {
	owner   string
	order   int
	label   string
	buttons []scriptToolbarButton
}

var (
	scriptToolbarMu   sync.RWMutex
	scriptToolbars    = map[string][]*scriptToolbarRegistration{}
	scriptToolbarNext = map[string]int{}
)

func (candidate *scriptCandidate) claimToolbar(options scriptapi.ToolbarOptions) bool {
	if candidate == nil {
		return true
	}
	candidate.mu.Lock()
	defer candidate.mu.Unlock()
	if candidate.failed {
		return false
	}
	if len(options.Buttons) == 0 || len(options.Buttons) > maxScriptToolbarButtons {
		candidate.conflicts = append(candidate.conflicts, fmt.Sprintf("toolbar must have 1 to %d buttons", maxScriptToolbarButtons))
		return false
	}
	bindings := append([]string(nil), candidate.bindings...)
	for index, button := range options.Buttons {
		buttonName := strings.TrimSpace(button.Label)
		if buttonName == "" {
			buttonName = fmt.Sprintf("%d", index+1)
		}
		if button.OnClick == nil || reflect.ValueOf(button.OnClick).IsNil() {
			candidate.conflicts = append(candidate.conflicts, "toolbar button "+buttonName+" needs OnClick")
			return false
		}
		icon := strings.TrimSpace(button.Icon)
		if icon != "" {
			if candidate.assets == nil || !validScriptRelativePath(icon) || !strings.EqualFold(pathExtension(icon), ".png") {
				candidate.conflicts = append(candidate.conflicts, "toolbar button "+buttonName+" has an invalid PNG icon path")
				return false
			}
		}
		key := strings.TrimSpace(button.Key)
		if key == "" {
			continue
		}
		if !validScriptBindingText(key) {
			candidate.conflicts = append(candidate.conflicts, "toolbar button "+buttonName+" has an invalid key")
			return false
		}
		for _, existing := range bindings {
			if sameCombo(existing, key) {
				candidate.conflicts = append(candidate.conflicts, "duplicate binding "+key+" in the same script")
				return false
			}
		}
		bindings = append(bindings, key)
	}
	candidate.bindings = bindings
	return true
}

func pathExtension(name string) string {
	if index := strings.LastIndexByte(name, '.'); index >= 0 {
		return name[index:]
	}
	return ""
}

func scriptRegisterToolbar(owner string, options scriptapi.ToolbarOptions, assets *scriptAssetSource) scriptRegistrationHandle {
	if scriptIsDisabled(owner) {
		return scriptRegistrationHandle{}
	}
	registration := &scriptToolbarRegistration{
		owner: owner, label: strings.TrimSpace(options.Label),
	}
	if registration.label == "" {
		registration.label = scriptDisplayName(owner)
	}
	eventQueue := currentScriptEventQueue(owner)
	var hotkeyHandles []scriptRegistrationHandle
	var handle scriptRegistrationHandle
	handle = registerScriptResource(owner, func() {
		for _, hotkey := range hotkeyHandles {
			hotkey.release()
		}
		removeScriptToolbar(owner, registration)
	})
	if !handle.valid() {
		return handle
	}
	for _, option := range options.Buttons {
		handler := option.OnClick
		button := scriptToolbarButton{
			label: strings.TrimSpace(option.Label), tooltip: strings.TrimSpace(option.Tooltip),
			key:     strings.TrimSpace(option.Key),
			onClick: func() { queueScriptCallbackOn(eventQueue, owner, "Toolbar", handler) },
		}
		if button.label == "" {
			button.label = "Button"
		}
		if icon := strings.TrimSpace(option.Icon); icon != "" {
			image, err := loadScriptToolbarIcon(assets, icon)
			if err != nil {
				reportScriptCommandError(owner, "toolbar icon "+icon+": "+err.Error())
			} else {
				button.image = image
			}
		}
		if button.key != "" {
			hotkeyHandles = append(hotkeyHandles, scriptAddHotkeyFn(owner, button.key, func(InputEvent) { handler() }))
		}
		registration.buttons = append(registration.buttons, button)
	}
	scriptToolbarMu.Lock()
	registration.order = scriptToolbarNext[owner]
	scriptToolbarNext[owner]++
	scriptToolbars[owner] = append(scriptToolbars[owner], registration)
	scriptToolbarMu.Unlock()
	refreshScriptToolbars()
	return handle
}

func loadScriptToolbarIcon(assets *scriptAssetSource, name string) (*ebiten.Image, error) {
	data, err := assets.read(name)
	if err != nil {
		return nil, err
	}
	config, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode PNG: %w", err)
	}
	if config.Width < 1 || config.Height < 1 || config.Width > 256 || config.Height > 256 {
		return nil, fmt.Errorf("PNG dimensions must be between 1 and 256 pixels")
	}
	decoded, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode PNG: %w", err)
	}
	return newImageFromImage(decoded), nil
}

func removeScriptToolbar(owner string, registration *scriptToolbarRegistration) {
	scriptToolbarMu.Lock()
	registrations := scriptToolbars[owner]
	for index, existing := range registrations {
		if existing == registration {
			registrations = append(registrations[:index], registrations[index+1:]...)
			break
		}
	}
	if len(registrations) == 0 {
		delete(scriptToolbars, owner)
		delete(scriptToolbarNext, owner)
	} else {
		scriptToolbars[owner] = registrations
	}
	scriptToolbarMu.Unlock()
	refreshScriptToolbars()
}

func refreshScriptToolbars() {
	if toolbarRoot != nil {
		placeToolbar(gs.ToolbarPlacement, false)
	}
}

func buildScriptToolbarRows() []*eui.ItemData {
	scriptToolbarMu.RLock()
	var registrations []*scriptToolbarRegistration
	for _, ownerToolbars := range scriptToolbars {
		registrations = append(registrations, ownerToolbars...)
	}
	scriptToolbarMu.RUnlock()
	sort.Slice(registrations, func(i, j int) bool {
		if registrations[i].owner == registrations[j].owner {
			return registrations[i].order < registrations[j].order
		}
		return registrations[i].owner < registrations[j].owner
	})
	rows := make([]*eui.ItemData, 0, len(registrations))
	for _, registration := range registrations {
		row := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_HORIZONTAL, Fixed: true}
		label, _ := eui.NewText()
		label.Text = registration.label
		label.FontSize = 11
		label.Size = eui.Point{X: 72, Y: 32}
		row.AddItem(label)
		for _, registeredButton := range registration.buttons {
			registeredButton := registeredButton
			button, events := eui.NewButton()
			button.Size = eui.Point{X: 34, Y: 32}
			button.FontSize = 10
			button.Image = registeredButton.image
			if button.Image == nil {
				button.Text = registeredButton.label
			}
			tooltip := registeredButton.tooltip
			if tooltip == "" {
				tooltip = registeredButton.label
			}
			if registeredButton.key != "" {
				tooltip += " [" + registeredButton.key + "]"
			}
			button.SetTooltip(tooltip)
			events.Handle = func(event eui.UIEvent) {
				if event.Type == eui.EventClick {
					registeredButton.onClick()
				}
			}
			row.AddItem(button)
		}
		rows = append(rows, row)
	}
	return rows
}
