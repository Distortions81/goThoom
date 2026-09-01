package main

import "github.com/hajimehoshi/ebiten/v2"

var frameBlendIndices = []uint32{0, 1, 2, 1, 2, 3}

type frameBlendDrawOptions struct {
	Left, Top      float64
	ScaleX, ScaleY float64
	Fade           float32
	Red            float32
	Green          float32
	Blue           float32
	Alpha          float32
	Linear         bool
}

// premultipliedDrawColor converts a straight-alpha tint into the format used
// by Ebitengine's sprite ColorScale and our frame-blend shader output.
func premultipliedDrawColor(red, green, blue, alpha float32) (float32, float32, float32, float32) {
	return red * alpha, green * alpha, blue * alpha, alpha
}

func drawFrameBlend(destination, previous, current *ebiten.Image, options frameBlendDrawOptions) bool {
	if destination == nil || previous == nil || current == nil || frameBlendShader == nil {
		return false
	}
	previousBounds := previous.Bounds()
	currentBounds := current.Bounds()
	width := max(previousBounds.Dx(), currentBounds.Dx())
	height := max(previousBounds.Dy(), currentBounds.Dy())
	if width < 1 || height < 1 {
		return false
	}
	previousOffsetX := float32((width - previousBounds.Dx()) / 2)
	previousOffsetY := float32((height - previousBounds.Dy()) / 2)
	currentOffsetX := float32((width - currentBounds.Dx()) / 2)
	currentOffsetY := float32((height - currentBounds.Dy()) / 2)
	custom := [4]float32{
		options.Fade,
		previousOffsetX - currentOffsetX,
		previousOffsetY - currentOffsetY,
		0,
	}
	if options.Linear {
		custom[3] = 1
	}

	left := float32(options.Left)
	top := float32(options.Top)
	right := float32(options.Left + float64(width)*options.ScaleX)
	bottom := float32(options.Top + float64(height)*options.ScaleY)
	sourceLeft := float32(previousBounds.Min.X) - previousOffsetX
	sourceTop := float32(previousBounds.Min.Y) - previousOffsetY
	sourceRight := sourceLeft + float32(width)
	sourceBottom := sourceTop + float32(height)
	vertices := [...]ebiten.Vertex{
		{DstX: left, DstY: top, SrcX: sourceLeft, SrcY: sourceTop},
		{DstX: right, DstY: top, SrcX: sourceRight, SrcY: sourceTop},
		{DstX: left, DstY: bottom, SrcX: sourceLeft, SrcY: sourceBottom},
		{DstX: right, DstY: bottom, SrcX: sourceRight, SrcY: sourceBottom},
	}
	red, green, blue, alpha := premultipliedDrawColor(options.Red, options.Green, options.Blue, options.Alpha)
	for index := range vertices {
		vertices[index].ColorR = red
		vertices[index].ColorG = green
		vertices[index].ColorB = blue
		vertices[index].ColorA = alpha
		vertices[index].Custom0 = custom[0]
		vertices[index].Custom1 = custom[1]
		vertices[index].Custom2 = custom[2]
		vertices[index].Custom3 = custom[3]
	}
	shaderOptions := &ebiten.DrawTrianglesShaderOptions{}
	shaderOptions.Images[0] = previous
	shaderOptions.Images[1] = current
	destination.DrawTrianglesShader32(vertices[:], frameBlendIndices, frameBlendShader, shaderOptions)
	return true
}
