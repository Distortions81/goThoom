package main

import (
	"reflect"
	"testing"
	"unsafe"

	"gothoom/climg"
)

func testCLImages(items map[uint32]*climg.ClientItem) *climg.CLImages {
	ci := &climg.CLImages{}
	rv := reflect.ValueOf(ci).Elem().FieldByName("items")
	reflect.NewAt(rv.Type(), unsafe.Pointer(rv.UnsafeAddr())).Elem().Set(reflect.ValueOf(items))
	return ci
}

func TestSetFullInventoryGroupsNonTemplate(t *testing.T) {
	resetInventory()
	old := clImages
	defer func() { clImages = old }()
	clImages = testCLImages(map[uint32]*climg.ClientItem{
		100: {Flags: 0},
		200: {Flags: kItemFlagData},
	})
	ids := []uint16{100, 100, 200, 200}
	eq := []bool{false, false, false, false}
	setFullInventory(ids, eq)
	items := getInventory()
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
	if items[0].ID != 100 || items[0].Quantity != 2 || items[0].IDIndex != -1 {
		t.Fatalf("unexpected first item %+v", items[0])
	}
	if items[1].ID != 200 || items[1].IDIndex != 0 || items[1].Quantity != 1 {
		t.Fatalf("unexpected second item %+v", items[1])
	}
	if items[2].ID != 200 || items[2].IDIndex != 1 || items[2].Quantity != 1 {
		t.Fatalf("unexpected third item %+v", items[2])
	}
}
