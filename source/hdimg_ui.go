package main

import (
	"fmt"
	"image"
	"log"
	"math"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"
	"gothoom/eui"
)

var (
	hdPicturePreviewWin     *eui.WindowData
	hdPicturePreviewRoot    *eui.ItemData
	hdPicturePreviewList    *eui.ItemData
	hdPicturePreviewStatus  *eui.ItemData
	hdPicturePreviewCards   []hdPicturePreviewCard
	hdPicturePreviewColumns int
	hdPicturePreviewZoomUI  *eui.ItemData
	hdPicturePreviewPackUI  *eui.ItemData
	hdSpritePackCB          *eui.ItemData
	hdPicturePreviewOpenBtn *eui.ItemData
	hdPicturePreviewIDs     []uint16
	hdPicturePreviewPackKey string
	hdPicturePreviewDirty   bool
)

const (
	hdPicturePreviewWindowWidth  float32 = 660
	hdPicturePreviewWindowHeight float32 = 700
	hdPicturePreviewCardWidth    float32 = 292
	hdPicturePreviewCardGap      float32 = 8
	hdPicturePreviewImageHeight  float32 = 184
	hdPicturePreviewFontSize     float32 = 12
)

type hdPicturePreviewMode uint8

const (
	hdPicturePreviewReplacement hdPicturePreviewMode = iota
	hdPicturePreviewOriginal
	hdPicturePreviewBoth
)

var (
	hdPicturePreviewDisplay = hdPicturePreviewReplacement
	hdPicturePreviewZoom    = previewGalleryZoomDefaultPercent / 100
)

type hdPicturePreviewCard struct {
	id                uint16
	source            hdPictureSource
	checkbox          *eui.ItemData
	image             *ebiten.Image
	replacement       *ebiten.Image
	replacementLoaded bool
}

func makeHDPicturePreviewWindow() {
	if hdPicturePreviewWin != nil {
		return
	}
	hdPicturePreviewWin = eui.NewWindow()
	hdPicturePreviewWin.Title = "HD Sprite Preview"
	hdPicturePreviewWin.Closable = true
	hdPicturePreviewWin.Resizable = true
	hdPicturePreviewWin.AutoSize = false
	hdPicturePreviewWin.Movable = true
	hdPicturePreviewWin.NoScroll = true
	hdPicturePreviewWin.NoCache = true
	hdPicturePreviewWin.Size = eui.Point{X: hdPicturePreviewWindowWidth, Y: hdPicturePreviewWindowHeight}
	hdPicturePreviewWin.ShowTooltipIndicators = true
	hdPicturePreviewWin.SetZone(eui.HZoneCenter, eui.VZoneMiddleTop)

	hdPicturePreviewRoot = eui.NewColumn()
	controls := eui.NewRow()

	packPicker, packEvents := eui.NewDropdown()
	hdPicturePreviewPackUI = packPicker
	packPicker.Label = "Sprite pack"
	packPicker.Size = eui.Point{X: 300, Y: settingsControlHeight}
	packPicker.FontSize = hdPicturePreviewFontSize
	packPicker.SetTooltip("Choose one ZIP to browse, or choose Loose files for PNGs stored directly in the hdimg folders.")
	packEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type != eui.EventDropdownSelected {
			return
		}
		selectedKey := ""
		imageCacheLifecycleMu.RLock()
		if ev.Index >= 0 && ev.Index < len(hdPicturePacks) {
			selectedKey = hdPicturePacks[ev.Index].key
		}
		imageCacheLifecycleMu.RUnlock()
		if selectedKey != "" {
			hdPicturePreviewPackKey = selectedKey
		}
		refreshHDPicturePreview()
	}
	controls.AddItem(packPicker)

	modePicker, modeEvents := eui.NewDropdown()
	modePicker.Label = "Display"
	modePicker.Options = []string{"HD replacements", "Originals", "Original + HD"}
	modePicker.Selected = int(hdPicturePreviewDisplay)
	modePicker.Size = eui.Point{X: 230, Y: settingsControlHeight}
	modePicker.FontSize = hdPicturePreviewFontSize
	modePicker.SetTooltip("Choose what every card shows. In the combined view, the original is on the left and HD replacement is on the right.")
	modeEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventDropdownSelected && ev.Index >= 0 && ev.Index < len(modePicker.Options) {
			hdPicturePreviewDisplay = hdPicturePreviewMode(ev.Index)
			updateHDPicturePreviewImages()
		}
	}
	controls.AddItem(modePicker)
	hdPicturePreviewRoot.AddItem(controls)

	actions := eui.NewRow()
	reloadButton, reloadEvents := eui.NewButton()
	reloadButton.Text = "Reload HD Sprites"
	setMaterialButtonIcon(reloadButton, "restart_alt")
	reloadButton.Size = eui.Point{X: 160, Y: settingsControlHeight}
	reloadButton.FontSize = hdPicturePreviewFontSize
	reloadButton.SetTooltip("Read PNGs and ZIPs in the game or user-data hdimg folder again.")
	reloadEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			reloadHDPictureDebug()
		}
	}
	actions.AddItem(reloadButton)

	zoomSlider, zoomEvents := eui.NewSlider()
	hdPicturePreviewZoomUI = zoomSlider
	zoomSlider.Label = "Zoom (%)"
	zoomSlider.MinValue = previewGalleryZoomMinPercent
	zoomSlider.MaxValue = previewGalleryZoomMaxPercent
	zoomSlider.Value = hdPicturePreviewZoom * 100
	zoomSlider.IntOnly = true
	zoomSlider.Size = eui.Point{X: 180, Y: settingsControlHeight}
	zoomSlider.FontSize = hdPicturePreviewFontSize
	zoomSlider.SetTooltip("Resize the preview cards. Smaller cards make more columns; the largest size fills the gallery width.")
	zoomEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventSliderChanged {
			hdPicturePreviewZoom = ev.Value / 100
			hdPicturePreviewColumns = 0
			layoutHDPicturePreviewWindow()
		}
	}
	actions.AddItem(zoomSlider)
	hdPicturePreviewRoot.AddItem(actions)

	hdPicturePreviewStatus, _ = eui.NewText()
	hdPicturePreviewStatus.FontSize = hdPicturePreviewFontSize
	hdPicturePreviewStatus.Size = eui.Point{X: 600, Y: 24}
	hdPicturePreviewRoot.AddItem(hdPicturePreviewStatus)

	hdPicturePreviewList = &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Fixed: true, Scrollable: true}
	hdPicturePreviewRoot.AddItem(hdPicturePreviewList)
	hdPicturePreviewWin.OnResize = layoutHDPicturePreviewWindow
	hdPicturePreviewWin.AddItem(hdPicturePreviewRoot)
	hdPicturePreviewWin.AddWindow(false)
	refreshHDPicturePreview()
}

func openHDPicturePreview() {
	makeHDPicturePreviewWindow()
	refreshHDPicturePreview()
	hdPicturePreviewWin.MarkOpen()
	layoutHDPicturePreviewWindow()
}

func layoutHDPicturePreviewWindow() {
	if hdPicturePreviewWin == nil || hdPicturePreviewRoot == nil || hdPicturePreviewList == nil {
		return
	}
	eui.LayoutWindowBody(hdPicturePreviewWin, hdPicturePreviewRoot, hdPicturePreviewList)
	columns := hdPicturePreviewColumnCount(hdPicturePreviewList.Size.X)
	if columns != hdPicturePreviewColumns || len(hdPicturePreviewCards) != len(hdPicturePreviewIDs) {
		rebuildHDPicturePreviewCards(columns)
		eui.LayoutWindowBody(hdPicturePreviewWin, hdPicturePreviewRoot, hdPicturePreviewList)
	}
	hdPicturePreviewWin.Refresh()
}

func hdPicturePreviewColumnCount(width float32) int {
	cardWidth, _ := hdPicturePreviewCardDimensions(width)
	return max(1, int((width+hdPicturePreviewCardGap)/(cardWidth+hdPicturePreviewCardGap)))
}

func hdPicturePreviewCardDimensions(availableWidth float32) (width, imageHeight float32) {
	zoom := hdPicturePreviewZoom
	if zoom <= 0 {
		zoom = 1
	}
	if zoom >= previewGalleryZoomMaxPercent/100 && availableWidth > 0 {
		usableWidth := max(float32(1), availableWidth-eui.ScrollbarWidth()/eui.UIScale())
		zoom = usableWidth / hdPicturePreviewCardWidth
	}
	return hdPicturePreviewCardWidth * zoom, hdPicturePreviewImageHeight * zoom
}

func rebuildHDPicturePreviewCards(columns int) {
	if hdPicturePreviewList == nil {
		return
	}
	if columns < 1 {
		columns = 1
	}
	scroll := hdPicturePreviewList.Scroll
	for _, card := range hdPicturePreviewCards {
		if card.image != nil {
			card.image.Deallocate()
		}
		if card.replacement != nil {
			card.replacement.Deallocate()
		}
	}
	hdPicturePreviewCards = nil
	hdPicturePreviewList.Contents = nil
	hdPicturePreviewColumns = columns
	cardWidth, imageHeight := hdPicturePreviewCardDimensions(hdPicturePreviewList.Size.X)

	for start := 0; start < len(hdPicturePreviewIDs); start += columns {
		row := eui.NewRow()
		for index := start; index < min(start+columns, len(hdPicturePreviewIDs)); index++ {
			id := hdPicturePreviewIDs[index]
			source, sourceFound := hdPicturePreviewSource(id)
			compatible := hdPictureCompatible(id)
			card := eui.NewColumn()
			card.Fixed = true
			card.Filled = true
			card.Color = eui.SubtleAlternateRowColor()
			card.Size = eui.Point{X: cardWidth, Y: imageHeight + 30}

			checkbox, events := eui.NewCheckbox()
			checkbox.Text = fmt.Sprintf("Picture %d", id)
			checkbox.Checked = compatible && hdPictureEnabled(id)
			checkbox.Disabled = !compatible
			checkbox.FontSize = hdPicturePreviewFontSize
			checkbox.Size = eui.Point{X: cardWidth, Y: 26}
			if compatible && sourceFound {
				checkbox.SetTooltip(fmt.Sprintf("Use this picture ID in the game. Preview source: %s", source.label))
			} else {
				checkbox.SetTooltip("This replacement cannot be enabled because the original picture has multiple frames.")
			}
			events.Handle = func(ev eui.UIEvent) {
				if ev.Type == eui.EventCheckboxChanged {
					setHDPictureEnabled(id, ev.Checked)
					invalidateHDPictureSelection()
					settingsDirty = true
				}
			}
			card.AddItem(checkbox)

			imageItem, backing := eui.NewImageFastItem(max(1, roundToInt(float64(cardWidth))), max(1, roundToInt(float64(imageHeight))))
			imageItem.Size = eui.Point{X: cardWidth, Y: imageHeight}
			card.AddItem(imageItem)
			row.AddItem(card)
			hdPicturePreviewCards = append(hdPicturePreviewCards, hdPicturePreviewCard{id: id, source: source, checkbox: checkbox, image: backing})
		}
		hdPicturePreviewList.AddItem(row)
	}
	hdPicturePreviewList.Scroll = scroll
	updateHDPicturePreviewImages()
}

func reloadHDPictureDebug() {
	location, count := reloadHDPictures()
	updateToolbarHands()
	refreshHDPicturePreview()
	consoleMessage(fmt.Sprintf("Reloaded %d HD sprites from %s.", count, location))
}

func refreshHDPicturePreview() {
	if hdPicturePreviewWin == nil {
		return
	}
	refreshHDPicturePreviewPackPicker()
	imageCacheLifecycleMu.RLock()
	pack, found := selectedHDPicturePreviewPackLocked()
	ids := make([]uint16, 0, len(pack.sources))
	for id := range pack.sources {
		ids = append(ids, id)
	}
	imageCacheLifecycleMu.RUnlock()
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	hdPicturePreviewIDs = ids
	if !found {
		hdPicturePreviewStatus.Text = "No HD sprite replacements found."
	} else {
		hdPicturePreviewStatus.Text = fmt.Sprintf("%d HD sprite replacements in %s", len(ids), pack.label)
	}
	hdPicturePreviewStatus.Dirty = true
	rebuildHDPicturePreviewCards(hdPicturePreviewColumnCount(hdPicturePreviewList.Size.X))
	layoutHDPicturePreviewWindow()
}

func refreshHDPicturePreviewPackPicker() {
	if hdPicturePreviewPackUI == nil {
		return
	}
	imageCacheLifecycleMu.RLock()
	options := make([]string, len(hdPicturePacks))
	packCount := len(hdPicturePacks)
	selected := -1
	selectedKey := hdPicturePreviewPackKey
	for index, pack := range hdPicturePacks {
		options[index] = pack.label
		if pack.key == selectedKey {
			selected = index
		}
	}
	if selected < 0 && len(hdPicturePacks) > 0 {
		selected = 0
		selectedKey = hdPicturePacks[0].key
	}
	imageCacheLifecycleMu.RUnlock()
	hdPicturePreviewPackKey = selectedKey
	if len(options) == 0 {
		options = []string{"No sprite packs found"}
		selected = 0
	}
	hdPicturePreviewPackUI.Options = options
	hdPicturePreviewPackUI.Selected = selected
	hdPicturePreviewPackUI.Disabled = packCount == 0
	hdPicturePreviewPackUI.Dirty = true
}

func selectedHDPicturePreviewPackLocked() (hdPicturePack, bool) {
	for _, pack := range hdPicturePacks {
		if pack.key == hdPicturePreviewPackKey {
			return pack, true
		}
	}
	return hdPicturePack{}, false
}

func hdPicturePreviewSource(id uint16) (hdPictureSource, bool) {
	imageCacheLifecycleMu.RLock()
	defer imageCacheLifecycleMu.RUnlock()
	pack, found := selectedHDPicturePreviewPackLocked()
	if !found {
		return hdPictureSource{}, false
	}
	source, found := pack.sources[id]
	return source, found
}

func updateHDPicturePreviewImages() {
	hdPicturePreviewDirty = true
	if hdPicturePreviewWin != nil {
		hdPicturePreviewWin.Refresh()
	}
}

// drawPendingHDPicturePreviewImages performs the texture draws immediately
// before EUI presents the window. In particular, the first gallery refresh
// happens while its window is still closed and cannot be presented yet.
func drawPendingHDPicturePreviewImages() {
	if !hdPicturePreviewDirty || hdPicturePreviewWin == nil || !hdPicturePreviewWin.IsOpen() {
		return
	}
	hdPicturePreviewDirty = false
	for index := range hdPicturePreviewCards {
		card := &hdPicturePreviewCards[index]
		if card.image == nil {
			continue
		}
		card.image.Clear()
		card.image.Fill(hdPicturePreviewWin.BackgroundColor())
		content := card.image.Bounds().Inset(8)
		switch hdPicturePreviewDisplay {
		case hdPicturePreviewOriginal:
			drawHDPicturePreviewImage(card.image, loadImageFrameOriginal(card.id, 0), content)
		case hdPicturePreviewBoth:
			const gap = 8
			leftWidth := max(1, (content.Dx()-gap)/2)
			left := content
			left.Max.X = left.Min.X + leftWidth
			right := content
			right.Min.X = left.Max.X + gap
			drawHDPicturePreviewImage(card.image, loadImageFrameOriginal(card.id, 0), left)
			drawHDPicturePreviewImage(card.image, card.hdReplacement(), right)
		default:
			drawHDPicturePreviewImage(card.image, card.hdReplacement(), content)
		}
	}
}

func (card *hdPicturePreviewCard) hdReplacement() *ebiten.Image {
	if card.replacementLoaded {
		return card.replacement
	}
	card.replacementLoaded = true
	img, err := decodeHDPictureSource(card.source)
	if err != nil {
		log.Printf("HD picture %d (%s): %v", card.id, card.source.label, err)
		return nil
	}
	card.replacement = img
	return img
}

func drawHDPicturePreviewImage(screen, picture *ebiten.Image, bounds image.Rectangle) {
	if screen == nil || picture == nil || bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		return
	}
	pictureBounds := picture.Bounds()
	if pictureBounds.Dx() <= 0 || pictureBounds.Dy() <= 0 {
		return
	}
	scale := math.Min(float64(bounds.Dx())/float64(pictureBounds.Dx()), float64(bounds.Dy())/float64(pictureBounds.Dy()))
	op := acquireDrawOpts()
	op.Filter = ebiten.FilterLinear
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(
		float64(bounds.Min.X)+(float64(bounds.Dx())-float64(pictureBounds.Dx())*scale)/2,
		float64(bounds.Min.Y)+(float64(bounds.Dy())-float64(pictureBounds.Dy())*scale)/2,
	)
	clip := screen.RecyclableSubImage(bounds)
	clip.DrawImage(picture, op)
	clip.Recycle()
	releaseDrawOpts(op)
}

func invalidateHDPictureSelection() {
	imageCacheLifecycleMu.Lock()
	imageMu.Lock()
	clearScaledArtworkCachesLocked()
	imageMu.Unlock()
	imageCacheLifecycleMu.Unlock()
	artworkCacheGeneration.Add(1)
	toolbarHandsRendered = false
	inventoryDirty = true
	playersDirty = true
	updateToolbarHands()
	if gameWin != nil {
		gameWin.Refresh()
	}
}
