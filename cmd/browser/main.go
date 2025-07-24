package main

import (
	"bytes"
	"fmt"
	goImg "image"
	"image/color"
	"log"
	"math"

	"github.com/ebitenui/ebitenui"
	"github.com/ebitenui/ebitenui/image"
	"github.com/ebitenui/ebitenui/widget"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/colornames"
	"golang.org/x/image/font/gofont/goregular"
)

var (
	whiteImage    = ebiten.NewImage(3, 3)
	whiteSubImage = whiteImage.SubImage(goImg.Rect(1, 1, 2, 2)).(*ebiten.Image)
)

func init() {
	whiteImage.Fill(color.White)
}

type Game struct {
	ui *ebitenui.UI
}

func (g *Game) Update() error {
	g.ui.Update()
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	g.ui.Draw(screen)

	ebitenutil.DebugPrint(screen, "Hello, World!")
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return 1080, 720
}

func main() {
	ebiten.SetWindowSize(1080, 720)
	ebiten.SetWindowTitle("Code Browser")

	button := widget.NewButton(
		widget.ButtonOpts.TextLabel("Test Button"),
		widget.ButtonOpts.TextFace(DefaultFont()),
		widget.ButtonOpts.TextColor(&widget.ButtonTextColor{
			Idle:    colornames.Green,
			Hover:   colornames.Green,
			Pressed: Mix(colornames.Gainsboro, colornames.Black, 0.4),
		}),
		widget.ButtonOpts.WidgetOpts(
			widget.WidgetOpts.LayoutData(widget.AnchorLayoutData{
				VerticalPosition:   widget.AnchorLayoutPositionCenter,
				HorizontalPosition: widget.AnchorLayoutPositionCenter,
			}),
			widget.WidgetOpts.MinSize(180, 48),
		),
		widget.ButtonOpts.Image(&widget.ButtonImage{
			Idle:         DefaultNineSlice(colornames.Darkslategray),
			Hover:        DefaultNineSlice(Mix(colornames.Darkslategray, colornames.Mediumseagreen, 0.4)),
			Disabled:     DefaultNineSlice(Mix(colornames.Darkslategray, colornames.Gainsboro, 0.8)),
			Pressed:      PressedNineSlice(Mix(colornames.Darkslategray, colornames.Black, 0.4)),
			PressedHover: PressedNineSlice(Mix(colornames.Darkslategray, colornames.Black, 0.4)),
		}),
	)

	root := widget.NewContainer(
		widget.ContainerOpts.BackgroundImage(
			image.NewNineSliceColor(colornames.Gainsboro),
		),
		widget.ContainerOpts.Layout(widget.NewAnchorLayout()),
	)
	root.AddChild(button)

	if err := ebiten.RunGame(&Game{ui: &ebitenui.UI{Container: root}}); err != nil {
		log.Fatal(err)
	}
}

func DefaultFont() text.Face {
	s, err := text.NewGoTextFaceSource(bytes.NewReader(goregular.TTF))
	if err != nil {
		panic(err)
	}
	return &text.GoTextFace{
		Source: s,
		Size:   20,
	}
}

func DefaultNineSlice(base color.Color) *image.NineSlice {
	var size float32 = 64
	var tiles float32 = 16
	var radius float32 = 8
	tile := size / tiles
	facet := Mix(base, colornames.Gainsboro, 0.2)
	img := ebiten.NewImage(int(size), int(size))
	path := RoundedRectPath(0, tile, size, size-tile, radius, radius, radius, radius)
	DrawFilledPath(img, path, base)
	path = RoundedRectPath(0, tile, size, size-tile*2, radius, radius, radius, radius)
	DrawFilledPath(img, path, facet)
	path = RoundedRectPath(tile, tile*2, size-tile*2, size-tile*4, radius, radius, radius, radius)
	DrawFilledPath(img, path, base)
	return image.NewNineSliceBorder(img, int(tile*4))
}

func PressedNineSlice(base color.Color) *image.NineSlice {
	var size float32 = 64
	var tiles float32 = 16
	var radius float32 = 8
	tile := size / tiles
	facet := Mix(base, colornames.Gainsboro, 0.2)
	img := ebiten.NewImage(int(size), int(size))
	path := RoundedRectPath(0, 0, size, size, radius, radius, radius, radius)
	DrawFilledPath(img, path, facet)
	path = RoundedRectPath(tile, tile, size-tile*2, size-tile*2, radius, radius, radius, radius)
	DrawFilledPath(img, path, base)
	return image.NewNineSliceBorder(img, int(tile*4))
}

func DrawFilledPath(img *ebiten.Image, path *vector.Path, clr color.Color) {
	vertices, indices := make([]ebiten.Vertex, 0, 64), make([]uint16, 0, 64)
	vertices, indices = path.AppendVerticesAndIndicesForFilling(vertices[:0], indices[:0])

	r, g, b, _ := clr.RGBA()
	for i := range vertices {
		vertices[i].SrcX = 1
		vertices[i].SrcY = 1
		vertices[i].ColorR = float32(r) / float32(0xffff)
		vertices[i].ColorG = float32(g) / float32(0xffff)
		vertices[i].ColorB = float32(b) / float32(0xffff)
		vertices[i].ColorA = 1
	}
	fmt.Println(vertices)

	op := &ebiten.DrawTrianglesOptions{}
	op.AntiAlias = true
	op.FillRule = ebiten.FillRuleNonZero
	img.DrawTriangles(vertices, indices, whiteSubImage, op)
}

func Mix(a, b color.Color, percent float64) color.Color {
	rgba := func(c color.Color) (r, g, b, a uint8) {
		r16, g16, b16, a16 := c.RGBA()
		return uint8(r16 >> 8), uint8(g16 >> 8), uint8(b16 >> 8), uint8(a16 >> 8)
	}
	lerp := func(x, y uint8) uint8 {
		return uint8(math.Round(float64(x) + percent*(float64(y)-float64(x))))
	}
	r1, g1, b1, a1 := rgba(a)
	r2, g2, b2, a2 := rgba(b)
	return color.RGBA{
		R: lerp(r1, r2),
		G: lerp(g1, g2),
		B: lerp(b1, b2),
		A: lerp(a1, a2),
	}
}

func RoundedRectPath(x, y, w, h, tl, tr, br, bl float32) *vector.Path {
	path := &vector.Path{}
	path.Arc(x+w-tr, y+tr, tr, 3*math.Pi/2, 0, vector.Clockwise)
	path.LineTo(x+w, y+h-br)
	path.Arc(x+w-br, y+h-br, br, 0, math.Pi/2, vector.Clockwise)
	path.LineTo(x+bl, y+h)
	path.Arc(x+bl, y+h-bl, bl, math.Pi/2, math.Pi, vector.Clockwise)
	path.LineTo(x, y+tl)
	path.Arc(x+tl, y+tl, tl, math.Pi, 3*math.Pi/2, vector.Clockwise)
	path.Close()
	return path
}
