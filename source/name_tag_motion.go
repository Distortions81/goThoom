package main

import "github.com/hajimehoshi/ebiten/v2"

// DrawImage adjusts destination vertices to roughly thirds of a pixel. Explicit
// triangles retain the interpolated positions for continuously moving labels.
func drawSmoothNameTag(dst, label *ebiten.Image, x, y float64, alpha float32) {
	bounds := label.Bounds()
	left, top := float32(x), float32(y)
	right, bottom := left+float32(bounds.Dx()), top+float32(bounds.Dy())
	vertices := [...]ebiten.Vertex{
		{DstX: left, DstY: top, SrcX: float32(bounds.Min.X), SrcY: float32(bounds.Min.Y)},
		{DstX: right, DstY: top, SrcX: float32(bounds.Max.X), SrcY: float32(bounds.Min.Y)},
		{DstX: left, DstY: bottom, SrcX: float32(bounds.Min.X), SrcY: float32(bounds.Max.Y)},
		{DstX: right, DstY: bottom, SrcX: float32(bounds.Max.X), SrcY: float32(bounds.Max.Y)},
	}
	for i := range vertices {
		vertices[i].ColorR, vertices[i].ColorG, vertices[i].ColorB = 1, 1, 1
		vertices[i].ColorA = alpha
	}
	indices := [...]uint16{0, 1, 2, 1, 2, 3}
	dst.DrawTriangles(vertices[:], indices[:], label, &ebiten.DrawTrianglesOptions{
		Filter: ebiten.FilterLinear, DisableMipmaps: true, Address: ebiten.AddressClampToZero,
	})
}
