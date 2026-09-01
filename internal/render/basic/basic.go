package basic

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"sync"

	"golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	"github.com/timboli111/PrintCat/internal/document"
	"github.com/timboli111/PrintCat/internal/render"
)

type Renderer struct {
	once sync.Once
	font *opentype.Font
	err  error
}

func (r *Renderer) Render(ctx context.Context, doc document.Document, target render.Target) (render.Raster, error) {
	if err := doc.Validate(); err != nil {
		return render.Raster{}, fmt.Errorf("invalid document: %w", err)
	}
	if target.DPI <= 0 {
		return render.Raster{}, fmt.Errorf("target DPI must be positive, got %d", target.DPI)
	}

	widthPx := int(float64(doc.PageSize.Width) / 1000.0 / 25.4 * float64(target.DPI))
	heightPx := int(float64(doc.PageSize.Height) / 1000.0 / 25.4 * float64(target.DPI))
	if widthPx <= 0 || heightPx <= 0 {
		return render.Raster{}, fmt.Errorf("invalid raster dimensions: %dx%d", widthPx, heightPx)
	}

	img := image.NewGray(image.Rect(0, 0, widthPx, heightPx))
	for i := range img.Pix {
		img.Pix[i] = 255
	}

	if err := r.initFont(); err != nil {
		return render.Raster{}, fmt.Errorf("failed to load font: %w", err)
	}

	for _, el := range doc.Elements {
		if !el.Visible {
			continue
		}
		switch el.Type {
		case document.TextElement:
			td, ok := el.Data.(document.TextData)
			if !ok {
				continue
			}
			r.renderText(img, target.DPI, el.Bounds, td)
		case document.ImageElement:
			idata, ok := el.Data.(document.ImageData)
			if !ok {
				continue
			}
			r.renderImage(img, target.DPI, el.Bounds, idata)
		default:
			continue
		}
	}

	return render.Raster{
		Width:  widthPx,
		Height: heightPx,
		Pixels: img.Pix,
	}, nil
}

func (r *Renderer) initFont() error {
	r.once.Do(func() {
		f, err := opentype.Parse(goregular.TTF)
		if err != nil {
			r.err = err
			return
		}
		r.font = f
	})
	return r.err
}

func (r *Renderer) renderText(img *image.Gray, dpi int, bounds document.Rect, td document.TextData) {
	pxMinX := int(float64(bounds.Position.X) / 1000.0 / 25.4 * float64(dpi))
	pxMinY := int(float64(bounds.Position.Y) / 1000.0 / 25.4 * float64(dpi))
	pxMaxX := int(float64(bounds.Position.X+bounds.Size.Width) / 1000.0 / 25.4 * float64(dpi))
	pxMaxY := int(float64(bounds.Position.Y+bounds.Size.Height) / 1000.0 / 25.4 * float64(dpi))
	if pxMinX >= img.Bounds().Dx() || pxMinY >= img.Bounds().Dy() || pxMaxX <= 0 || pxMaxY <= 0 {
		return
	}

	pxMinX = max(pxMinX, 0)
	pxMinY = max(pxMinY, 0)
	pxMaxX = min(pxMaxX, img.Bounds().Dx())
	pxMaxY = min(pxMaxY, img.Bounds().Dy())

	points := float64(td.FontSize) / 1000.0 / 25.4 * 72.0
	if points < 1 {
		points = 1
	}

	face, err := opentype.NewFace(r.font, &opentype.FaceOptions{
		Size:    points,
		DPI:     float64(dpi),
		Hinting: font.HintingFull,
	})
	if err != nil {
		return
	}
	defer face.Close()

	metrics := face.Metrics()
	ascent := metrics.Ascent.Ceil()
	descent := metrics.Descent.Ceil()
	lineHeight := ascent + descent

	d := &font.Drawer{
		Dst:  img,
		Src:  image.Black,
		Face: face,
	}

	advance := d.MeasureString(td.Content)
	advancePx := advance.Ceil()
	if advancePx > pxMaxX-pxMinX {
		advancePx = pxMaxX - pxMinX
	}

	var x int
	switch td.Alignment {
	case "center":
		x = pxMinX + (pxMaxX-pxMinX-advancePx)/2
	case "right":
		x = pxMaxX - advancePx
	default:
		x = pxMinX
	}
	y := pxMinY + (pxMaxY-pxMinY-lineHeight)/2 + ascent
	if y < 0 {
		y = ascent
	}
	if y > img.Bounds().Dy() {
		y = img.Bounds().Dy() - descent
	}

	d.Dot = fixed.Point26_6{
		X: fixed.I(x),
		Y: fixed.I(y),
	}
	d.DrawString(td.Content)
}

func (r *Renderer) renderImage(img *image.Gray, dpi int, bounds document.Rect, idata document.ImageData) {
	src, _, err := image.Decode(bytes.NewReader(idata.Data))
	if err != nil {
		pxMinX := int(float64(bounds.Position.X) / 1000.0 / 25.4 * float64(dpi))
		pxMinY := int(float64(bounds.Position.Y) / 1000.0 / 25.4 * float64(dpi))
		pxMaxX := int(float64(bounds.Position.X+bounds.Size.Width) / 1000.0 / 25.4 * float64(dpi))
		pxMaxY := int(float64(bounds.Position.Y+bounds.Size.Height) / 1000.0 / 25.4 * float64(dpi))
		pxMinX = max(pxMinX, 0)
		pxMinY = max(pxMinY, 0)
		pxMaxX = min(pxMaxX, img.Bounds().Dx())
		pxMaxY = min(pxMaxY, img.Bounds().Dy())
		if pxMinX >= pxMaxX || pxMinY >= pxMaxY {
			return
		}
		rect := image.Rect(pxMinX, pxMinY, pxMaxX, pxMaxY)
		draw.Draw(img, rect, image.NewUniform(color.Gray{Y: 128}), image.Point{}, draw.Src)
		return
	}

	pxMinX := int(float64(bounds.Position.X) / 1000.0 / 25.4 * float64(dpi))
	pxMinY := int(float64(bounds.Position.Y) / 1000.0 / 25.4 * float64(dpi))
	pxMaxX := int(float64(bounds.Position.X+bounds.Size.Width) / 1000.0 / 25.4 * float64(dpi))
	pxMaxY := int(float64(bounds.Position.Y+bounds.Size.Height) / 1000.0 / 25.4 * float64(dpi))
	if pxMinX >= img.Bounds().Dx() || pxMinY >= img.Bounds().Dy() || pxMaxX <= 0 || pxMaxY <= 0 {
		return
	}
	pxMinX = max(pxMinX, 0)
	pxMinY = max(pxMinY, 0)
	pxMaxX = min(pxMaxX, img.Bounds().Dx())
	pxMaxY = min(pxMaxY, img.Bounds().Dy())
	dstRect := image.Rect(pxMinX, pxMinY, pxMaxX, pxMaxY)

	draw.ApproxBiLinear.Scale(img, dstRect, src, src.Bounds(), draw.Src, nil)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
