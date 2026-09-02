package tspl

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/timboli111/PrintCat/internal/document"
	"github.com/timboli111/PrintCat/internal/printer"
	"github.com/timboli111/PrintCat/internal/render"
)

type Encoder struct{}

func (e *Encoder) Protocol() printer.Protocol {
	return printer.TSPL
}

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

	copies := 1
	if copiesStr, ok := options["copies"]; ok {
		if n, err := strconv.Atoi(copiesStr); err == nil && n > 0 {
			copies = n
		}
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("SIZE %d,%d", widthDots, heightDots))
	lines = append(lines, "GAP 0,0")
	lines = append(lines, "CLS")

	for _, el := range doc.Elements {
		if !el.Visible {
			continue
		}
		x := int(float64(el.Bounds.Position.X) / 1000.0 / 25.4 * float64(profile.DPI))
		y := int(float64(el.Bounds.Position.Y) / 1000.0 / 25.4 * float64(profile.DPI))
		w := int(float64(el.Bounds.Size.Width) / 1000.0 / 25.4 * float64(profile.DPI))
		h := int(float64(el.Bounds.Size.Height) / 1000.0 / 25.4 * float64(profile.DPI))
		yTSPL := heightDots - y - h

		switch el.Type {
		case document.TextElement:
			td, ok := el.Data.(document.TextData)
			if !ok {
				continue
			}
			lines = append(lines, textCommand(x, yTSPL, td.Content, td.FontSize, profile.DPI))
		case document.ImageElement:
			idata, ok := el.Data.(document.ImageData)
			if !ok {
				continue
			}
			cmd, err := bitmapCommand(ctx, renderer, x, yTSPL, w, h, el.Bounds.Size.Width, el.Bounds.Size.Height, idata, profile.DPI)
			if err != nil {
				return nil, fmt.Errorf("bitmap command failed: %w", err)
			}
			lines = append(lines, cmd)
		case document.LineElement:
			lines = append(lines, fmt.Sprintf("LINE %d,%d,%d,%d,1", x, yTSPL, x+w, yTSPL+h))
		case document.RectangleElement:
			lines = append(lines, fmt.Sprintf("BOX %d,%d,%d,%d,1", x, yTSPL, x+w, yTSPL+h))
		}
	}

	lines = append(lines, fmt.Sprintf("PRINT %d,1", copies))
	payload := strings.Join(lines, "\n") + "\n"
	return []byte(payload), nil
}

func textCommand(x, y int, content string, fontSize document.Unit, dpi int) string {
	heightDots := int(float64(fontSize) / 1000.0 / 25.4 * float64(dpi))
	font := "3"
	if heightDots > 24 {
		font = "4"
	}
	if heightDots > 32 {
		font = "5"
	}
	text := strings.ReplaceAll(content, ",", " ")
	return fmt.Sprintf("TEXT %d,%d,%s,0,1,1,%s", x, y, font, text)
}

func bitmapCommand(ctx context.Context, renderer render.Renderer, x, y, w, h int, widthUm, heightUm document.Unit, idata document.ImageData, dpi int) (string, error) {
	if w <= 0 || h <= 0 {
		return "", fmt.Errorf("invalid bitmap dimensions: %dx%d dots", w, h)
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
		return "", fmt.Errorf("render image failed: %w", err)
	}
	if raster.Width != w || raster.Height != h {
		return "", fmt.Errorf("raster size mismatch: expected %dx%d, got %dx%d", w, h, raster.Width, raster.Height)
	}
	if len(raster.Pixels) != w*h {
		return "", fmt.Errorf("raster pixel count mismatch: expected %d, got %d", w*h, len(raster.Pixels))
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
	hexData := ""
	for _, b := range packed {
		hexData += fmt.Sprintf("%02X", b)
	}
	return fmt.Sprintf("BITMAP %d,%d,%d,%d,0,%s", x, y, w, h, hexData), nil
}
