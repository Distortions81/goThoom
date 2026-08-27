package main

import (
	_ "embed"
	"encoding/json"
	"image"
	_ "image/png"
	"net/http"
	"strings"

	"gothoom/eui"

	"github.com/pkg/browser"
)

//go:embed data/about.txt
var aboutText string

var aboutWin *eui.WindowData
var aboutList *eui.ItemData
var aboutLines []string

var patreonList *eui.ItemData

const patreonsURL = "https://m45sci.xyz/u/dist/goThoom/patreons.json"
const websiteURL = "https://gothoom.m45sci.xyz/"

func initAboutUI() {
	if aboutWin != nil {
		return
	}
	aboutWin, aboutList, _ = makeTextWindow("About", eui.HZoneCenter, eui.VZoneMiddleTop, false)
	aboutWin.AutoSize = true

	flow := aboutList.Parent

	linkBtn, linkEvents := eui.NewButton()
	linkBtn.Text = "goThoom Site"
	linkBtn.Size.Y = 20
	linkBtn.Fixed = true
	linkEvents.Handle = func(ev eui.UIEvent) {
		if ev.Type == eui.EventClick {
			browser.OpenURL(websiteURL)
		}
	}

	flow.PrependItem(linkBtn)

	aboutLines = strings.Split(strings.ReplaceAll(aboutText, "\r\n", "\n"), "\n")
	aboutWin.OnResize = func() { updateTextWindow(aboutWin, aboutList, nil, aboutLines, 15, "", monoFaceSource, false) }

	go loadPatreons()
}

func openAboutWindow(anchor *eui.ItemData) {
	if aboutWin == nil {
		return
	}

	if aboutWin.Open {
		aboutWin.Close()
		return
	}

	updateTextWindow(aboutWin, aboutList, nil, aboutLines, 15, "", monoFaceSource, false)
	if anchor != nil {
		aboutWin.MarkOpenNear(anchor)
	} else {
		aboutWin.MarkOpen()
	}
	if aboutWin.OnResize != nil {
		aboutWin.OnResize()
	}
	aboutWin.Refresh()
}

type patreonEntry struct {
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
}

type patreonFile struct {
	Patreons []patreonEntry `json:"patreons"`
}

func loadPatreons() {
	resp, err := http.Get(patreonsURL)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return
	}
	var pf patreonFile
	if err := json.NewDecoder(resp.Body).Decode(&pf); err != nil {
		return
	}
	for _, p := range pf.Patreons {
		url := p.Avatar
		if url == "" {
			continue
		}
		imgResp, err := http.Get(url)
		if err != nil {
			continue
		}
		img, _, err := image.Decode(imgResp.Body)
		imgResp.Body.Close()
		if err != nil {
			continue
		}
		w := img.Bounds().Dx()
		h := img.Bounds().Dy()
		imgItem, backing := eui.NewImageItem(w, h)
		backing.Deallocate()
		imgItem.Image = newManagedImageFromImage(img)
		imgItem.Size = eui.Point{X: float32(w), Y: float32(h)}
		imgItem.Fixed = true
		nameItem, _ := eui.NewText()
		nameItem.Text = p.Name
		nameItem.FontSize = 14
		nameItem.Fixed = true
		nameItem.Size.Y = 16
		patItem := &eui.ItemData{ItemType: eui.ITEM_FLOW, FlowType: eui.FLOW_VERTICAL, Fixed: true}
		patItem.AddItem(imgItem)
		patItem.AddItem(nameItem)
		patItem.Size.X = float32(w)
		patItem.Size.Y = float32(h) + nameItem.Size.Y
		patreonList.AddItem(patItem)
	}
	if aboutWin != nil {
		aboutWin.Refresh()
	}
}
