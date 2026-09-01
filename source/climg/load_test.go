package climg

import (
	"encoding/binary"
	"errors"
	"io"
	"testing"
)

type testImageResource struct {
	typeID uint32
	id     uint32
	data   []byte
}

func TestLoadBytesParsesChunkedResourceData(t *testing.T) {
	idref := make([]byte, 42)
	binary.BigEndian.PutUint32(idref[0:4], 1)
	binary.BigEndian.PutUint32(idref[4:8], 10)
	binary.BigEndian.PutUint32(idref[8:12], 20)
	binary.BigEndian.PutUint32(idref[12:16], 0x12345678)
	binary.BigEndian.PutUint32(idref[16:20], pictDefFlagTransparent)
	binary.BigEndian.PutUint32(idref[28:32], 30)
	binary.BigEndian.PutUint16(idref[32:34], 0xfff9)
	binary.BigEndian.PutUint16(idref[34:36], 3)
	binary.BigEndian.PutUint16(idref[36:38], 2)
	binary.BigEndian.PutUint16(idref[38:40], 5)
	binary.BigEndian.PutUint16(idref[40:42], 0xffff)

	item := make([]byte, 20+7)
	binary.BigEndian.PutUint32(item[0:4], 0xaabbccdd)
	binary.BigEndian.PutUint32(item[4:8], 0xfffffffd)
	binary.BigEndian.PutUint32(item[8:12], 101)
	binary.BigEndian.PutUint32(item[12:16], 102)
	binary.BigEndian.PutUint32(item[16:20], 103)
	copy(item[20:], "healer\x00")

	archive := testCLImagesFile([]testImageResource{
		{typeID: TYPE_IMAGE, id: 10, data: []byte{1, 2, 3}},
		{typeID: TYPE_COLOR, id: 20, data: []byte{0, 7, 11, 255}},
		{typeID: TYPE_LIGHT, id: 30, data: []byte{1, 2, 3, 4, 0, 9, 0xff, 0xfe}},
		{typeID: TYPE_IDREF, id: 40, data: idref},
		{typeID: TYPE_CLIENT_ITEM, id: 50, data: item},
	})
	images, err := LoadBytes(archive)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(images.locations); got != 5 {
		t.Fatalf("location count = %d, want 5", got)
	}
	if got := images.colors[20].colorBytes; len(got) != 4 || got[1] != 7 || got[3] != 255 {
		t.Fatalf("color table = %v", got)
	}
	ref := images.idrefs[40]
	if ref == nil || ref.version != 1 || ref.imageID != 10 || ref.colorID != 20 || ref.checksum != 0x12345678 || ref.flags != pictDefFlagTransparent {
		t.Fatalf("idref = %#v", ref)
	}
	if ref.lightingID != 30 || ref.plane != -7 || ref.numFrames != 3 || ref.numAnims != 2 || ref.animFrameTable[0] != 5 || ref.animFrameTable[1] != -1 {
		t.Fatalf("idref optional fields = %#v", ref)
	}
	light, ok := images.Lighting(40)
	if !ok || light.Color != [4]byte{1, 2, 3, 4} || light.Radius != 9 || light.Plane != -2 {
		t.Fatalf("lighting = %#v, %t", light, ok)
	}
	gotItem, ok := images.Item(50)
	if !ok || gotItem.Flags != 0xaabbccdd || gotItem.Slot != -3 || gotItem.RightHandPictID != 101 || gotItem.LeftHandPictID != 102 || gotItem.WornPictID != 103 || gotItem.Name != "healer" {
		t.Fatalf("item = %#v, %t", gotItem, ok)
	}
}

func TestLoadBytesRejectsTruncatedEntryTable(t *testing.T) {
	data := make([]byte, 12)
	binary.BigEndian.PutUint16(data[0:2], 0xffff)
	binary.BigEndian.PutUint32(data[2:6], 1)
	if _, err := LoadBytes(data); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("error = %v, want unexpected EOF", err)
	}
}

func testCLImagesFile(resources []testImageResource) []byte {
	const headerSize = 12
	tableSize := len(resources) * 16
	totalSize := headerSize + tableSize
	for _, resource := range resources {
		totalSize += len(resource.data)
	}
	data := make([]byte, totalSize)
	binary.BigEndian.PutUint16(data[0:2], 0xffff)
	binary.BigEndian.PutUint32(data[2:6], uint32(len(resources)))
	binary.BigEndian.PutUint16(data[10:12], 2)

	resourceOffset := headerSize + tableSize
	for index, resource := range resources {
		tableOffset := headerSize + index*16
		binary.BigEndian.PutUint32(data[tableOffset:tableOffset+4], uint32(resourceOffset))
		binary.BigEndian.PutUint32(data[tableOffset+4:tableOffset+8], uint32(len(resource.data)))
		binary.BigEndian.PutUint32(data[tableOffset+8:tableOffset+12], resource.typeID)
		binary.BigEndian.PutUint32(data[tableOffset+12:tableOffset+16], resource.id)
		copy(data[resourceOffset:], resource.data)
		resourceOffset += len(resource.data)
	}
	return data
}
