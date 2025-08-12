package eui

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// DumpTree writes the window and overlay hierarchy to debug/tree.json.
func DumpTree() error {
	if err := os.MkdirAll("debug", 0755); err != nil {
		return err
	}
	tree := struct {
		Windows  []treeWindow `json:"windows"`
		Overlays []treeItem   `json:"overlays"`
	}{}
	for _, w := range windows {
		tw := treeWindow{
			Title:    w.Title,
			Position: w.Position,
			Size:     w.Size,
			PinTo:    w.PinTo,
		}
		for _, it := range w.Contents {
			tw.Items = append(tw.Items, makeTreeItem(it))
		}
		tree.Windows = append(tree.Windows, tw)
	}
	for _, ov := range overlays {
		tree.Overlays = append(tree.Overlays, makeTreeItem(ov))
	}
	data, err := json.MarshalIndent(tree, "", "  ")
	if err != nil {
		return err
	}
	fn := filepath.Join("debug", "tree.json")
	return os.WriteFile(fn, data, 0644)
}

type treeWindow struct {
	Title    string     `json:"title"`
	Position point      `json:"position"`
	Size     point      `json:"size"`
	PinTo    pinType    `json:"pin_to"`
	Items    []treeItem `json:"items"`
}

type treeItem struct {
	Name     string     `json:"name,omitempty"`
	Text     string     `json:"text,omitempty"`
	Type     string     `json:"type"`
	Position point      `json:"position"`
	Size     point      `json:"size"`
	Items    []treeItem `json:"items,omitempty"`
}

func makeTreeItem(it *itemData) treeItem {
	ti := treeItem{
		Name:     it.Name,
		Text:     it.Text,
		Type:     itemTypeName(it.ItemType),
		Position: it.Position,
		Size:     it.Size,
	}
	for _, c := range it.Contents {
		ti.Items = append(ti.Items, makeTreeItem(c))
	}
	for _, t := range it.Tabs {
		ti.Items = append(ti.Items, makeTreeItem(t))
	}
	return ti
}

func itemTypeName(t itemTypeData) string {
	switch t {
	case ITEM_FLOW:
		return "flow"
	case ITEM_TEXT:
		return "text"
	case ITEM_BUTTON:
		return "button"
	case ITEM_CHECKBOX:
		return "checkbox"
	case ITEM_RADIO:
		return "radio"
	case ITEM_INPUT:
		return "input"
	case ITEM_SLIDER:
		return "slider"
	case ITEM_DROPDOWN:
		return "dropdown"
	case ITEM_COLORWHEEL:
		return "colorwheel"
	default:
		return "unknown"
	}
}
