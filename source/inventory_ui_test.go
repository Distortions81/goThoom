package main

import (
	"reflect"
	"strings"
	"testing"

	"gothoom/climg"
	"gothoom/eui"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestInventoryWindowIncrementalUpdates(t *testing.T) {
	resetInventory()

	oldImages := clImages
	defer func() { clImages = oldImages }()
	clImages = testCLImages(map[uint32]*climg.ClientItem{
		100: {Name: "shadow bell", Slot: kItemSlotRightHand},
		200: {Name: "linen shirt", Slot: kItemSlotTorso},
	})

	inventoryWin = nil
	inventoryList = nil
	inventoryRowRefs = map[*eui.ItemData]invRef{}
	invRender = inventoryRenderState{}
	invRegularSrc = nil
	invBoldSrc = nil
	invItalicSrc = nil
	selectedInvID = 0
	selectedInvIdx = -1

	makeInventoryWindow()
	inventoryWin.MarkOpen()

	addInventoryItem(100, -1, "shadow bell", false)
	addInventoryItem(200, -1, "linen shirt", false)
	updateInventoryWindow()

	if inventoryList == nil {
		t.Fatal("inventory list not initialized")
	}
	initial := append([]*eui.ItemData(nil), inventoryList.Contents...)
	if len(initial) < 3 {
		t.Fatalf("expected at least 2 rows and spacer, got %d", len(initial))
	}
	shadowRow := findInventoryTestRow(t, "Shadow Bell")
	linenRow := findInventoryTestRow(t, "Linen Shirt")
	spacer := initial[len(initial)-1]

	addInventoryItem(100, -1, "shadow bell", false)
	updateInventoryWindow()
	assertInventoryRowsUseConfiguredHeight(t)

	if len(inventoryList.Contents) != len(initial) {
		t.Fatalf("expected %d items, got %d", len(initial), len(inventoryList.Contents))
	}
	if findInventoryTestRow(t, "Shadow Bell") != shadowRow {
		t.Fatalf("expected shadow bell row to be reused")
	}
	if findInventoryTestRow(t, "Linen Shirt") != linenRow {
		t.Fatalf("expected linen shirt row to be reused")
	}
	if inventoryList.Contents[len(inventoryList.Contents)-1] != spacer {
		t.Fatalf("expected spacer to be reused")
	}
	if len(shadowRow.Contents) < 2 {
		t.Fatalf("expected name text in first row")
	}
	if !strings.Contains(shadowRow.Contents[1].Text, "(2)") {
		t.Fatalf("expected quantity suffix in shadow bell row text, got %q", shadowRow.Contents[1].Text)
	}

	removeInventoryItem(100, -1)
	updateInventoryWindow()

	if findInventoryTestRow(t, "Shadow Bell") != shadowRow {
		t.Fatalf("expected shadow bell row pointer to remain after decrement")
	}
	if strings.Contains(shadowRow.Contents[1].Text, "(2)") {
		t.Fatalf("expected quantity suffix to be removed")
	}

	removeInventoryItem(100, -1)
	updateInventoryWindow()

	if len(inventoryList.Contents) != 2 {
		t.Fatalf("expected one row plus spacer after removal, got %d", len(inventoryList.Contents))
	}
	if findInventoryTestRow(t, "Linen Shirt") != linenRow {
		t.Fatalf("expected remaining row pointer to persist")
	}

	equipInventoryItem(200, -1, true)
	updateInventoryWindow()
	assertInventoryRowsUseConfiguredHeight(t)

	if len(inventoryList.Contents) != 2 {
		t.Fatalf("expected row count to remain stable after equip")
	}
	if findInventoryTestRow(t, "Linen Shirt") != linenRow {
		t.Fatalf("expected equip to reuse existing row pointer")
	}
	if len(linenRow.Contents) < 3 {
		t.Fatalf("expected slot label to be present when equipped")
	}
	if got := linenRow.Contents[len(linenRow.Contents)-1].Text; got != "[Torso]" {
		t.Fatalf("expected slot label [Torso], got %q", got)
	}

	equipInventoryItem(200, -1, false)
	updateInventoryWindow()

	if len(linenRow.Contents) != 2 {
		t.Fatalf("expected slot label removed after unequip")
	}
	if len(inventoryRowRefs) != 1 {
		t.Fatalf("expected row refs to contain single entry, got %d", len(inventoryRowRefs))
	}
}

func TestToggleInventoryEquipWaitsForServerUpdate(t *testing.T) {
	resetCommandStateForTest(t, 1)
	resetInventory()
	originalImages := clImages
	t.Cleanup(func() {
		clImages = originalImages
		resetInventory()
	})
	clImages = testCLImages(map[uint32]*climg.ClientItem{
		100: {Name: "training sword", Slot: kItemSlotRightHand},
	})
	addInventoryItem(100, -1, "training sword", false)

	toggleInventoryEquipAt(100, -1)
	items := getInventory()
	if len(items) != 1 || items[0].Equipped {
		t.Fatalf("equip changed before server update: %+v", items)
	}
	if got, want := getQueuedCommands(), []string{"/equip 100"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("equip commands=%v, want %v", got, want)
	}
	if _, ok := handleInvCmdOther(kInvCmdEquip, []byte{0, 100}); !ok {
		t.Fatal("server equip update failed")
	}
	if items = getInventory(); len(items) != 1 || !items[0].Equipped {
		t.Fatalf("server equip update not applied: %+v", items)
	}

	toggleInventoryEquipAt(100, -1)
	if items = getInventory(); len(items) != 1 || !items[0].Equipped {
		t.Fatalf("unequip changed before server update: %+v", items)
	}
	if _, ok := handleInvCmdOther(kInvCmdUnequip, []byte{0, 100}); !ok {
		t.Fatal("server unequip update failed")
	}
	if items = getInventory(); len(items) != 1 || items[0].Equipped {
		t.Fatalf("server unequip update not applied: %+v", items)
	}
}

func TestInventoryWindowCountsStackedSlotsAndUnderlinesEquippedItems(t *testing.T) {
	resetInventory()
	originalImages := clImages
	originalWindow := inventoryWin
	originalList := inventoryList
	originalRender := invRender
	originalGroups := gs.InventoryGroups
	t.Cleanup(func() {
		clImages = originalImages
		inventoryWin = originalWindow
		inventoryList = originalList
		invRender = originalRender
		gs.InventoryGroups = originalGroups
	})

	gs.InventoryGroups = customGroups{}
	clImages = testCLImages(map[uint32]*climg.ClientItem{
		100: {Name: "stone", Slot: kItemSlotNotWearable},
		200: {Name: "shirt", Slot: kItemSlotTorso},
	})
	inventoryWin = nil
	inventoryList = nil
	invRender = inventoryRenderState{}
	makeInventoryWindow()
	inventoryWin.MarkOpen()
	addInventoryItem(100, -1, "stone", false)
	addInventoryItem(100, -1, "stone", false)
	addInventoryItem(200, -1, "shirt", true)
	updateInventoryWindow()

	if got, want := inventoryWin.Title, "Inventory   Slots: 3/32"; got != want {
		t.Fatalf("inventory title = %q, want %q", got, want)
	}
	shirt := findInventoryTestRow(t, "Shirt")
	var underlined bool
	for _, child := range shirt.Contents {
		if len(child.Underlines) > 0 {
			underlined = true
			break
		}
	}
	if !underlined {
		t.Fatal("equipped inventory item was not underlined")
	}
}

func findInventoryTestRow(t *testing.T, name string) *eui.ItemData {
	t.Helper()
	for _, row := range inventoryList.Contents {
		if len(row.Contents) > 1 && strings.Contains(row.Contents[1].Text, name) {
			return row
		}
	}
	t.Fatalf("inventory row %q not found", name)
	return nil
}

func assertInventoryRowsUseConfiguredHeight(t *testing.T) {
	t.Helper()
	for _, row := range inventoryList.Contents {
		if _, ok := inventoryRowRefs[row]; !ok {
			continue
		}
		if row.Size.Y != invRender.rowUnits {
			t.Fatalf("expected row height %v, got %v", invRender.rowUnits, row.Size.Y)
		}
	}
}

func TestInventoryIconUpdateMarksWindowDirty(t *testing.T) {
	resetInventory()

	inventoryWin = nil
	inventoryList = nil
	inventoryRowRefs = map[*eui.ItemData]invRef{}
	invRender = inventoryRenderState{}
	invRegularSrc = nil
	invBoldSrc = nil
	invItalicSrc = nil

	makeInventoryWindow()
	invRender.clientWAvail = 200
	invRender.rowUnits = 20
	invRender.iconSize = 20
	invRender.fontSize = 12

	row := invRender.createRow(inventoryRowData{
		key:   invGroupKey{id: 100, name: "test"},
		id:    100,
		idx:   -1,
		label: "Test",
	})
	inventoryList.AddItem(row.row)
	inventoryWin.Dirty = false
	row.icon.Dirty = false

	img := ebiten.NewImage(8, 8)
	invRender.updateRow(row, inventoryRowData{
		key:      row.key,
		id:       row.id,
		idx:      row.idx,
		label:    "Test",
		icon:     img,
		iconName: "item:100",
	})

	if row.icon.Image != img {
		t.Fatalf("icon image was not updated")
	}
	if row.icon.Size.X != float32(invRender.iconSize) || row.icon.Size.Y != float32(invRender.iconSize) {
		t.Fatalf("icon size got %v, want %d square", row.icon.Size, invRender.iconSize)
	}
	if !row.icon.Dirty {
		t.Fatalf("icon was not marked dirty")
	}
	if !inventoryWin.Dirty {
		t.Fatalf("inventory window was not marked dirty")
	}
}

func TestSizeTextWindowListSubtractsDockedRows(t *testing.T) {
	parent := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Fixed: true}
	toolbar := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Fixed: true}
	toolbar.Size = eui.Point{X: 300, Y: 60}
	list := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Fixed: true, Scrollable: true}
	parent.AddItem(toolbar)
	parent.AddItem(list)

	eui.SizeTextWindowList(list, 300, 200)

	if got, want := list.Size.Y, float32(140); got != want {
		t.Fatalf("list height = %v, want %v after docked rows", got, want)
	}
	if got, want := list.Size.X, float32(300); got != want {
		t.Fatalf("list width = %v, want %v", got, want)
	}
}
