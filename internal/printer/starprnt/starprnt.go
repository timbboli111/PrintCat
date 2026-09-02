package starprnt

import (
	"context"
	"fmt"
	"math"
	"strconv"

	"github.com/timboli111/PrintCat/internal/document"
	"github.com/timboli111/PrintCat/internal/printer"
	"github.com/timboli111/PrintCat/internal/render"
)

type Encoder struct{}

func (e *Encoder) Protocol() printer.Protocol {
	return printer.StarPRNT
}

const (
	ESC = 0x1B
	GS  = 0x1D
	RS  = 0x1E
	FF  = 0x0C
)

func (e *Encoder) Encode(ctx context.Context, doc document.Document, renderer render.Renderer, profile printer.PrinterProfile, options map[string]string) ([]byte, error) {
	if err := doc.Validate(); err != nil {
		return nil, fmt.Errorf("invalid document: %w", err)
	}
	if profile.DPI <= 0 {
		return nil, fmt.Errorf("invalid DPI: %d", profile.DPI)
	}

	widthDots := int(float64(doc.PageSize.Width) / 1000.0 / 25.4 * float64(profile.DPI))
	heightDots := int(float64(doc.PageSize.Height) / 1000.0 / 25.4 * float64(profile.DPI))
	if widthDots <= 0 || heightDots <= 0 {
		return nil, fmt.Errorf("invalid label dimensions: %dx%d dots", widthDots, heightDots)
	}
	if profile.MediaWidth > 0 {
		mediaDots := int(float64(profile.MediaWidth) / 1000.0 / 25.4 * float64(profile.DPI))
		if widthDots > mediaDots {
			return nil, fmt.Errorf("document width (%d dots) exceeds media width (%d dots)", widthDots, mediaDots)
		}
	}

	heightMm := float64(doc.PageSize.Height) / 1000.0
	n := int(math.Ceil(heightMm / 24.0))
	if n < 1 || n > 255 {
		return nil, fmt.Errorf("page height %.2f mm cannot be represented by ESC C 0 n (n=%d, must be 1-255)", heightMm, n)
	}

	copies := 1
	if copiesStr, ok := options["copies"]; ok {
		c, err := strconv.Atoi(copiesStr)
		if err != nil || c <= 0 {
			return nil, fmt.Errorf("invalid copies: %s", copiesStr)
		}
		copies = c
	}

	var parts [][]byte

	parts = append(parts, []byte{ESC, 0x40})
	parts = append(parts, []byte{ESC, 0x43, 0x00, byte(n)})

	for _, el := range doc.Elements {
		if !el.Visible {
			continue
		}
		if el.Bounds.Position.X < 0 || el.Bounds.Position.Y < 0 {
			return nil, fmt.Errorf("element %q has negative position", el.ID)
		}
		if el.Bounds.Position.X+el.Bounds.Size.Width > doc.PageSize.Width {
			return nil, fmt.Errorf("element %q exceeds page width", el.ID)
		}
		if el.Bounds.Position.Y+el.Bounds.Size.Height > doc.PageSize.Height {
			return nil, fmt.Errorf("element %q exceeds page height", el.ID)
		}

		x := int(float64(el.Bounds.Position.X) / 1000.0 / 25.4 * float64(profile.DPI))
		y := int(float64(el.Bounds.Position.Y) / 1000.0 / 25.4 * float64(profile.DPI))
		w := int(float64(el.Bounds.Size.Width) / 1000.0 / 25.4 * float64(profile.DPI))
		h := int(float64(el.Bounds.Size.Height) / 1000.0 / 25.4 * float64(profile.DPI))

		switch el.Type {
		case document.TextElement:
			td, ok := el.Data.(document.TextData)
			if !ok {
				continue
			}
			parts = append(parts, textCommand(x, y, td.Content, td.FontSize, profile.DPI))
		case document.ImageElement:
			idata, ok := el.Data.(document.ImageData)
			if !ok {
				continue
			}
			cmd, err := imageCommand(ctx, renderer, x, y, w, h, el.Bounds.Size.Width, el.Bounds.Size.Height, idata, profile.DPI)
			if err != nil {
				return nil, fmt.Errorf("image command failed: %w", err)
			}
			parts = append(parts, cmd)
		case document.LineElement, document.RectangleElement:
			continue
		default:
			continue
		}
	}

	for i := 0; i < copies; i++ {
		parts = append(parts, []byte{FF})
	}

	result := make([]byte, 0)
	for _, p := range parts {
		result = append(result, p...)
	}
	return result, nil
}

func absPos(pos int) []byte {
	n1 := byte(pos & 0xFF)
	n2 := byte((pos >> 8) & 0xFF)
	return []byte{ESC, GS, 0x41, n1, n2}
}

func fontSelect(font int) []byte {
	return []byte{ESC, RS, 0x46, byte(font)}
}

func textCommand(x, y int, content string, fontSize document.Unit, dpi int) []byte {
	var parts [][]byte
	parts = append(parts, absPos(x), absPos(y))
	parts = append(parts, fontSelect(0))
	parts = append(parts, []byte(content))
	parts = append(parts, []byte{'\n'})

	result := make([]byte, 0)
	for _, p := range parts {
		result = append(result, p...)
	}
	return result
}

func imageCommand(ctx context.Context, renderer render.Renderer, x, y, w, h int, widthUm, heightUm document.Unit, idata document.ImageData, dpi int) ([]byte, error) {
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("invalid image dimensions: %dx%d dots", w, h)
	}

	tmpDoc := document.New("img", "image", document.Size{Width: widthUm, Height: heightUm})
	elem := document.Element{
		ID:      "img",
		Type:    document.ImageElement,
		Bounds:  document.Rect{Position: document.Point{X: 0, Y: 0}, Size: document.Size{Width: widthUm, Height: heightUm}},
		Visible: true,
		Data:    idata,
	}
	tmpDoc.Elements = append(tmpDoc.Elements, elem)

	target := render.Target{DPI: dpi, Monochrome: true}
	raster, err := renderer.Render(ctx, tmpDoc, target)
	if err != nil {
		return nil, fmt.Errorf("render image failed: %w", err)
	}
	if raster.Width != w || raster.Height != h {
		return nil, fmt.Errorf("raster size mismatch: expected %dx%d, got %dx%d", w, h, raster.Width, raster.Height)
	}
	if len(raster.Pixels) != w*h {
		return nil, fmt.Errorf("raster pixel count mismatch")
	}

	widthBytes := (w + 7) / 8
	packed := make([]byte, widthBytes*h)
	for yPos := 0; yPos < h; yPos++ {
		for xPos := 0; xPos < w; xPos++ {
			idx := yPos*w + xPos
			bit := 0
			if raster.Pixels[idx] < 128 {
				bit = 1
			}
			byteIdx := yPos*widthBytes + xPos/8
			bitPos := 7 - (xPos % 8)
			if bit == 1 {
				packed[byteIdx] |= 1 << bitPos
			}
		}
	}

	xL := byte(widthBytes & 0xFF)
	xH := byte((widthBytes >> 8) & 0xFF)
	yL := byte(h & 0xFF)
	yH := byte((h >> 8) & 0xFF)

	var parts [][]byte
	parts = append(parts, absPos(x), absPos(y))
	parts = append(parts, []byte{ESC, GS, 0x53, 0x01, xL, xH, yL, yH, 0x00})
	parts = append(parts, packed)

	result := make([]byte, 0)
	for _, p := range parts {
		result = append(result, p...)
	}
	return result, nil
}
