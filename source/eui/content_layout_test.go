package eui

import (
	"math"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

func TestButtonWidthIncludesCaptionIconAndHelpAtEveryScale(t *testing.T) {
	if err := Init(); err != nil {
		t.Fatal(err)
	}
	oldScale := UIScale()
	t.Cleanup(func() { SetUIScale(oldScale) })
	for _, scale := range []float32{1, 1.5, 2} {
		SetUIScale(scale)
		win := NewWindow()
		win.ShowTooltipIndicators = true
		button, _ := NewButton()
		button.Text = "Reload Macros"
		button.Size = Point{X: 44, Y: 24}
		button.FontSize = 12
		button.Image = ebiten.NewImage(24, 24)
		button.SetTooltip("Reload enabled macros.")
		win.AddItem(button)
		width, _ := text.Measure(button.Text, itemFace(button, 12*scale+2), 0)
		want := float32(math.Ceil(width + float64(45*scale)))
		if button.GetSize().X < want {
			t.Fatalf("caption/icon/help clipped at %gx: %g < %g", scale, button.GetSize().X, want)
		}
		large := button.GetSize().X
		button.Text = "Go"
		button.Image = nil
		button.SetTooltip("")
		if button.GetSize().X >= large || button.Size.X != 44 {
			t.Fatal("content sizing permanently changed requested width")
		}
	}
}

func TestButtonWidthIncludesPredictionAfterPrimaryCaption(t *testing.T) {
	if err := Init(); err != nil {
		t.Fatal(err)
	}
	button, _ := NewButton()
	button.Text = "/give\nCoins."
	button.FontSize = 11
	withoutPrediction := button.buttonContentWidth()
	button.Prediction = " <person> <amount>"
	if withPrediction := button.buttonContentWidth(); withPrediction <= withoutPrediction {
		t.Fatalf("button prediction did not increase caption width: %.1f <= %.1f", withPrediction, withoutPrediction)
	}
}

func TestConstrainedFlowKeepsDeclaredViewportSize(t *testing.T) {
	if err := Init(); err != nil {
		t.Fatal(err)
	}
	oldScale := UIScale()
	t.Cleanup(func() { SetUIScale(oldScale) })
	SetUIScale(2)

	row := NewRow()
	row.Fixed = true
	row.ConstrainToSize = true
	row.Size = Point{X: 100, Y: 20}
	button, _ := NewButton()
	button.Text = "Wide child"
	button.Size = Point{X: 100, Y: 20}
	button.Position = Point{X: 4, Y: 4}
	row.AddItem(button)

	if got := row.GetSize(); got != (Point{X: 200, Y: 40}) {
		t.Fatalf("constrained flow grew to %+v, want its declared 200x40 pixel viewport", got)
	}
}

func TestConstrainedButtonKeepsDeclaredWidth(t *testing.T) {
	if err := Init(); err != nil {
		t.Fatal(err)
	}
	oldScale := UIScale()
	t.Cleanup(func() { SetUIScale(oldScale) })
	SetUIScale(1)

	button, _ := NewButton()
	button.Text = "A caption wider than the available tab"
	button.Size = Point{X: 40, Y: 20}
	if button.GetSize().X <= 40 {
		t.Fatal("test caption does not exercise content width expansion")
	}
	button.ConstrainToSize = true
	if got := button.GetSize().X; got != 40 {
		t.Fatalf("constrained button width = %.2f, want 40", got)
	}
}

func TestDiagramButtonCaptionFitsAndRecoversAfterResize(t *testing.T) {
	if err := Init(); err != nil {
		t.Fatal(err)
	}
	oldScale := UIScale()
	t.Cleanup(func() { SetUIScale(oldScale) })
	button, _ := NewButton()
	button.Text = "Chat +\nConsole"
	button.ConstrainToSize = true
	for _, scale := range []float32{1, 1.5, 2} {
		SetUIScale(scale)
		button.Size = Point{X: 32, Y: 24}
		button.FitButtonCaption(12)
		if button.FontSize >= 12 || button.buttonContentWidth() > button.Size.X*scale {
			t.Fatal("small diagram caption does not fit")
		}
		button.Size = Point{X: 140, Y: 100}
		button.FitButtonCaption(12)
		if button.FontSize != 12 {
			t.Fatal("enlarging diagram did not restore the caption size")
		}
	}
}

func TestWindowBodyReservesMeasuredFooterAndPositionGaps(t *testing.T) {
	if err := Init(); err != nil {
		t.Fatal(err)
	}
	oldScale := UIScale()
	oldW, oldH := ScreenSize()
	t.Cleanup(func() { SetUIScale(oldScale); SetScreenSize(oldW, oldH) })
	SetScreenSize(1920, 1080)
	for _, scale := range []float32{1, 1.5, 2} {
		SetUIScale(scale)
		win := NewWindow()
		win.Size = Point{X: 600, Y: 450}
		win.NoScroll = true
		root := NewColumn()
		root.Fixed = true
		intro := NewLabel("Intro")
		intro.Size = Point{X: 560, Y: 35}
		intro.Position.Y = 7
		body := NewColumn()
		body.Fixed = true
		body.Scrollable = true
		body.Position.Y = 5
		for i := 0; i < 30; i++ {
			line := NewLabel("Scrollable content")
			line.Size = Point{X: 400, Y: 24}
			body.AddItem(line)
		}
		save, _ := NewButton()
		save.Text = "Save"
		save.Size = Point{X: 44, Y: 28}
		save.Position.Y = 6
		footer := NewRow(save)
		footer.Position.Y = 9
		root.AddItem(intro)
		root.AddItem(body)
		root.AddItem(footer)
		win.AddItem(root)
		win.AddWindow(false)
		win.MarkOpen()
		for _, height := range []float32{450, 300, 500} {
			win.Size.Y = height
			LayoutWindowBody(win, root, body)
			win.Refresh()
			canvas := ebiten.NewImage(1920, 1080)
			Draw(canvas)
			bounds := win.getMainRect()
			if save.DrawRect.Y1 <= save.DrawRect.Y0 || save.DrawRect.Y1 > win.GetPos().Y+bounds.Y1+1 {
				t.Fatalf("footer escaped client area at %gx / %gh", scale, height)
			}
			if body.DrawRect.Y1 > save.DrawRect.Y0 {
				t.Fatal("scroll body overlaps footer")
			}
			if body.Size.Y >= root.Size.Y {
				t.Fatal("fixed rows did not reserve body space")
			}
			canvas.Deallocate()
		}
		win.RemoveWindow()
	}
}

func TestNestedFlowPositionIsAppliedOnce(t *testing.T) {
	if err := Init(); err != nil {
		t.Fatal(err)
	}
	oldW, oldH := ScreenSize()
	SetScreenSize(1000, 700)
	t.Cleanup(func() { SetScreenSize(oldW, oldH) })
	win := NewWindow()
	win.Size = Point{X: 700, Y: 450}
	root := NewRow()
	root.Position = Point{X: 11, Y: 13}
	column := NewColumn()
	column.Position = Point{X: 17, Y: 19}
	row := NewRow()
	row.Position = Point{X: 23, Y: 29}
	button, _ := NewButton()
	button.Text = "Nested"
	button.Size = Point{X: 100, Y: 28}
	button.Position = Point{X: 3, Y: 5}
	row.AddItem(button)
	column.AddItem(row)
	root.AddItem(column)
	win.AddItem(root)
	win.AddWindow(false)
	win.MarkOpen()
	defer win.RemoveWindow()
	canvas := ebiten.NewImage(1000, 700)
	Draw(canvas)
	defer canvas.Deallocate()
	pad := (win.Padding + win.BorderPad) * win.scale()
	wantX := win.GetPos().X + pad + (11+17+23+3)*UIScale()
	wantY := win.GetPos().Y + win.GetTitleSize() + pad + (13+19+29+5)*UIScale()
	if math.Abs(float64(button.DrawRect.X0-wantX)) > 1 || math.Abs(float64(button.DrawRect.Y0-wantY)) > 1 {
		t.Fatalf("nested position = %v, want (%g,%g)", button.DrawRect, wantX, wantY)
	}
}

func TestNestedFlowResizePropagatesBeforeFollowingSibling(t *testing.T) {
	if err := Init(); err != nil {
		t.Fatal(err)
	}
	oldScale := UIScale()
	t.Cleanup(func() { SetUIScale(oldScale) })
	for _, scale := range []float32{1, 1.25, 2} {
		SetUIScale(scale)
		inner := NewColumn()
		section := NewColumn(NewLabel("Text Sizes"), NewRow(inner))
		following := NewColumn(NewLabel("Input Assistance"))
		root := NewColumn(section, following)
		// Populate the nested column after its ancestors have been measured.
		for range 4 {
			inner.AddItem(&ItemData{ItemType: ITEM_TEXT, Size: Point{X: 200, Y: 48}})
		}
		root.resizeFlow(Point{X: 700 * scale, Y: 700 * scale})
		want := section.GetSize().Y + following.GetSize().Y
		if root.GetSize().Y < want {
			t.Fatalf("nested flow clipped following sibling at %gx: height %g, need %g", scale, root.GetSize().Y, want)
		}
		before := root.GetSize()
		root.resizeFlow(Point{X: 700 * scale, Y: 700 * scale})
		if root.GetSize() != before {
			t.Fatalf("layout needed a second pass at %gx: %v -> %v", scale, before, root.GetSize())
		}
	}
}
