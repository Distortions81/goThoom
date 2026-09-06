// assetpreview exports local CL_Images previews for manual asset classification.
package main

import (
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"sort"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
	"gothoom/climg"
)

type asset struct {
	ID           uint32 `json:"id"`
	Width        int    `json:"sheet_width"`
	Height       int    `json:"sheet_height"`
	Frames       int    `json:"animation_frames"`
	Plane        int    `json:"plane"`
	Flags        uint32 `json:"flags"`
	CustomColors bool   `json:"custom_colors"`
}

func main() {
	archive := flag.String("images", "", "path to installed CL_Images")
	out := flag.String("out", "", "new output directory (must not already exist)")
	min := flag.Uint("min", 0, "first image ID")
	max := flag.Uint("max", 99, "last image ID")
	flag.Parse()
	if *archive == "" || *out == "" || *min > *max || uint64(*max) > math.MaxUint32 {
		flag.Usage()
		os.Exit(2)
	}
	if err := export(*archive, *out, uint32(*min), uint32(*max)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func export(archive, out string, min, max uint32) error {
	data, err := os.ReadFile(archive)
	if err != nil {
		return err
	}
	images, err := climg.LoadBytes(data)
	if err != nil {
		return err
	}
	// Never overwrite earlier exports or hand-written classifications.
	if err := os.Mkdir(out, 0755); err != nil {
		return err
	}
	ids := images.IDs()
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	var assets []asset
	var sheet *image.RGBA
	page := 0
	for _, id := range ids {
		if id < min || id > max {
			continue
		}
		pixels := images.DecodeRGBA(id, nil, false)
		if pixels == nil {
			return fmt.Errorf("decode image %d", id)
		}
		if err := writePNG(filepath.Join(out, fmt.Sprintf("image-%04d.png", id)), pixels); err != nil {
			return err
		}
		w, h := images.Size(id)
		a := asset{id, w, h, images.NumFrames(id), images.Plane(id), images.Flags(id), images.HasCustomColors(id)}
		cell := len(assets) % 20
		if cell == 0 {
			sheet = image.NewRGBA(image.Rect(0, 0, 960, 1000))
			draw.Draw(sheet, sheet.Bounds(), image.NewUniform(color.RGBA{38, 42, 48, 255}), image.Point{}, draw.Src)
			page++
		}
		x, y := cell%4*240, cell/4*200
		label(sheet, x+8, y+16, fmt.Sprintf("%d | %dx%d | plane %d", id, w, h, a.Plane))
		label(sheet, x+8, y+32, fmt.Sprintf("frames %d | palette %v", a.Frames, a.CustomColors))
		preview := pixels.Bounds().Inset(1) // DecodeRGBA adds a one-pixel border.
		// Mobile-like sheets have 16 columns and 3 rows, sometimes a palette
		// footer. This is only a preview heuristic, not a semantic type label.
		if mobileLike(w, h) {
			thumb(sheet, pixels, preview, image.Rect(x+8, y+40, x+232, y+90))
			preview.Max = preview.Min.Add(image.Pt(w/16, w/16))
			thumb(sheet, pixels, preview, image.Rect(x+65, y+94, x+175, y+194))
		} else {
			if a.Frames > 1 {
				preview.Max.Y = preview.Min.Y + h/a.Frames
			}
			thumb(sheet, pixels, preview, image.Rect(x+8, y+40, x+232, y+194))
		}
		assets = append(assets, a)
		if cell == 19 {
			if err := writePNG(filepath.Join(out, fmt.Sprintf("contact-%02d.png", page)), sheet); err != nil {
				return err
			}
		}
	}
	if len(assets)%20 != 0 {
		if err := writePNG(filepath.Join(out, fmt.Sprintf("contact-%02d.png", page)), sheet); err != nil {
			return err
		}
	}
	manifest := struct {
		ArchiveSHA256 string  `json:"archive_sha256"`
		Assets        []asset `json:"assets"`
	}{fmt.Sprintf("%x", sha256.Sum256(data)), assets}
	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(out, "metadata.json"), append(encoded, '\n'), 0644); err != nil {
		return err
	}
	fmt.Printf("Exported %d images and %d contact sheets to %s\n", len(assets), page, out)
	return nil
}

func mobileLike(w, h int) bool { return w >= 256 && w%16 == 0 && (h == w/16*3 || h == w/16*3+1) }

func label(dst *image.RGBA, x, y int, text string) {
	d := font.Drawer{Dst: dst, Src: image.White, Face: basicfont.Face7x13, Dot: fixed.P(x, y)}
	d.DrawString(text)
}

func thumb(dst *image.RGBA, src image.Image, crop, box image.Rectangle) {
	if crop.Empty() {
		return
	}
	scale := math.Min(float64(box.Dx())/float64(crop.Dx()), float64(box.Dy())/float64(crop.Dy()))
	w, h := int(float64(crop.Dx())*scale), int(float64(crop.Dy())*scale)
	x0, y0 := box.Min.X+(box.Dx()-w)/2, box.Min.Y+(box.Dy()-h)/2
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			gray := uint8(100)
			if (x/8+y/8)%2 == 0 {
				gray = 145
			}
			dst.SetRGBA(x0+x, y0+y, color.RGBA{gray, gray, gray, 255})
		}
	}
	// Nearest-neighbor sampling preserves the original pixel-art structure.
	scaled := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			scaled.Set(x, y, src.At(crop.Min.X+int(float64(x)/scale), crop.Min.Y+int(float64(y)/scale)))
		}
	}
	draw.Draw(dst, image.Rect(x0, y0, x0+w, y0+h), scaled, image.Point{}, draw.Over)
}

func writePNG(path string, img image.Image) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}
	err = png.Encode(f, img)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	return err
}
