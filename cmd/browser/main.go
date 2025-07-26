package main

import (
	"bytes"
	"doc_generator/pkg/parsers"
	"doc_generator/pkg/store"
	goImg "image"
	"image/color"
	"io/fs"
	"log"
	"math"
	"math/rand/v2"
	"os"
	"path"
	"path/filepath"
	"slices"

	"github.com/ebitenui/ebitenui"
	"github.com/ebitenui/ebitenui/image"
	"github.com/ebitenui/ebitenui/widget"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/exp/constraints"
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

type Vector[T any] struct {
	X T
	Y T
}

type Number interface {
	constraints.Float | constraints.Integer
}

func Sub[T Number](a, b Vector[T]) Vector[T] {
	return Vector[T]{
		X: a.X - b.X,
		Y: a.Y - b.Y,
	}
}

func Add[T Number](a, b Vector[T]) Vector[T] {
	return Vector[T]{
		X: a.X + b.X,
		Y: a.Y + b.Y,
	}
}

func Len[T Number](a Vector[T]) float64 {
	x := float64(a.X)
	y := float64(a.Y)
	return math.Sqrt((x * x) + (y * y))
}

func Normalized[T constraints.Float](a Vector[T]) Vector[T] {
	len := Len(a)
	return Vector[T]{
		X: a.X / T(len),
		Y: a.Y / T(len),
	}
}

func INormalized[T constraints.Integer](a Vector[T]) Vector[float64] {
	len := Len(a)
	return Vector[float64]{
		X: float64(a.X) / float64(len),
		Y: float64(a.Y) / float64(len),
	}
}

type IVec = Vector[int32]
type FVec = Vector[float32]

type Edge struct {
	From int
	To   int
}

type Graph[T any] struct {
	Vertices []T
	Links    []Edge
}

type ImportVertex struct {
	Position FVec
	FileName string
}

type ImportView struct {
	Graph Graph[ImportVertex]
}

func (iv *ImportView) Draw(screen *ebiten.Image) {
	nineSlice := DefaultNineSlice(colornames.Darkslategray)
	face := DefaultFont()
	for _, file := range iv.Graph.Vertices {
		width, height := text.Measure(file.FileName, face, 48)
		nineSlice.Draw(screen, int(width)+10, int(height)+10, func(opts *ebiten.DrawImageOptions) {
			opts.GeoM.Translate(float64(file.Position.X), float64(file.Position.Y))
		})
		opts := &text.DrawOptions{}
		opts.GeoM.Translate(float64(file.Position.X)+5, float64(file.Position.Y)+5)
		text.Draw(screen, file.FileName, face, opts)
	}

	for _, link := range iv.Graph.Links {
		fromPos := iv.Graph.Vertices[link.From].Position
		toPos := iv.Graph.Vertices[link.To].Position
		vector.StrokeLine(screen, fromPos.X, fromPos.Y, toPos.X, toPos.Y, 1.0, colornames.Darkgreen, true)
	}
}

func (iv *ImportView) Update() {
	for idxA, a := range iv.Graph.Vertices {
		for idxB, b := range iv.Graph.Vertices {
			if idxA == idxB {
				continue
			}

			areLinked := -1 != slices.IndexFunc(iv.Graph.Links, func(v Edge) bool {
				return (v.From == idxA && v.To == idxB) ||
					(v.From == idxB && v.To == idxA)
			})

			fromPos := a.Position
			if fromPos.X == b.Position.X && fromPos.Y == b.Position.Y {
				fromPos = Add(b.Position, FVec{rand.Float32() - 0.5, rand.Float32() - 0.5})
			}
			diff := Sub(fromPos, b.Position)

			distGoal := 300
			if areLinked {
				distGoal = 100
			}

			normalized := Normalized(diff)
			if Len(diff) > float64(distGoal) {
				iv.Graph.Vertices[idxA].Position = Sub(a.Position, normalized)
			} else {
				iv.Graph.Vertices[idxA].Position = Add(a.Position, normalized)
			}
		}
	}
}

type Game struct {
	ui *ebitenui.UI

	source store.Source

	importView ImportView
}

func (g *Game) Update() error {
	g.ui.Update()
	g.importView.Update()
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	g.ui.Draw(screen)

	g.importView.Draw(screen)

	//nineSlice := DefaultNineSlice(colornames.Darkslategray)

	//x, y := 10, 0
	//face := DefaultFont()
	//for _, file := range g.source.Files {
	//	width, height := text.Measure(file.Name, face, 48)
	//	nineSlice.Draw(screen, int(width)+10, int(height)+10, func(opts *ebiten.DrawImageOptions) {
	//		opts.GeoM.Translate(float64(x), float64(y))
	//	})
	//	opts := &text.DrawOptions{}
	//	opts.GeoM.Translate(float64(x)+5, float64(y)+5)
	//	text.Draw(screen, file.Name, face, opts)
	//	y += int(height) + 10
	//}

	ebitenutil.DebugPrint(screen, "Hello, World!")
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return 1920, 1080
}

func main() {

	inputPath := os.Args[1]

	source := store.Source{}

	filepath.WalkDir(inputPath, func(fPath string, d fs.DirEntry, err error) error {

		fileType := path.Ext(fPath)

		if fileType == ".cpp" || fileType == ".hpp" || fileType == ".h" || fileType == ".c" {
			file, err := os.ReadFile(fPath)
			if err != nil {
				panic(err)
			}

			parser := &parsers.CPlusPlus{FileName: fPath, File: file}
			parser.Parse(&source)
		}

		return nil
	})

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

	iv := ImportView{}
	for _, file := range source.Files {
		iv.Graph.Vertices = append(iv.Graph.Vertices, ImportVertex{
			Position: FVec{(1920 / 2) + (rand.Float32() - 0.5), (1080 / 2) + (rand.Float32() - 0.5)},
			FileName: file.Name,
		})
	}
	for _, file := range source.Files {
		from := slices.IndexFunc(iv.Graph.Vertices, func(v ImportVertex) bool {
			return v.FileName == file.Name
		})
		for _, imported := range file.FileImports {
			to := slices.IndexFunc(iv.Graph.Vertices, func(v ImportVertex) bool {
				return path.Base(v.FileName) == path.Base(imported)
			})
			if to != -1 {
				iv.Graph.Links = append(iv.Graph.Links, Edge{from, to})
			}
		}
	}

	if err := ebiten.RunGame(&Game{
		source:     source,
		ui:         &ebitenui.UI{Container: root},
		importView: iv,
	}); err != nil {
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
