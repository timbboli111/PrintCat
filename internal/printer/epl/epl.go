package epl

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/timboli111/PrintCat/internal/document"
	"github.com/timboli111/PrintCat/internal/printer"
	"github.com/timboli111/PrintCat/internal/render"
)

const lf = "\n"

type Encoder struct{}

func (e *Encoder) Protocol() printer.Protocol {
	return printer.EPL
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
		n, err := strconv.Atoi(copiesStr)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("invalid copies: %s", copiesStr)
		}
		copies = n
	}

	// Build EPL2 session
	var parts [][]byte

	// N - Clear image buffer
	parts = append(parts, []byte("N"+lf))

	// q{width} - Label width
	parts = append(parts, []byte(fmt.Sprintf("q%d"+lf, widthDots)))

	// Q{height},0 - Label length, gap=0 (continuous media)
	parts = append(parts, []byte(fmt.Sprintf("Q%d,0"+lf, heightDots)))

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
		case document.LineElement:
			parts = append(parts, []byte(fmt.Sprintf("LO%d,%d,%d,%d"+lf, x, y, x+w, y+h)))
		case document.RectangleElement:
			parts = append(parts, []byte(fmt.Sprintf("BOX%d,%d,%d,%d,1"+lf, x, y, x+w, y+h)))
		default:
			continue
		}
	}

	// P{copies},1 - Print
	parts = append(parts, []byte(fmt.Sprintf("P%d,1"+lf, copies)))

	// Concatenate all parts
	result := make([]byte, 0)
	for _, p := range parts {
		result = append(result, p...)
	}
	return result, nil
}

// EPL Text: A{x},{y},0,3,1,1,N,"data"
// Font 3 = 24x24 dots
func textCommand(x, y int, content string, fontSize document.Unit, dpi int) []byte {
	sizeDots := int(float64(fontSize) / 1000.0 / 25.4 * float64(dpi))
	font := "3" // default 24x24
	if sizeDots <= 16 {
		font = "2" // 16x16
	} else if sizeDots <= 24 {
		font = "3" // 24x24
	} else {
		font = "4" // 32x32
	}
	// Escape quotes in content
	text := strings.ReplaceAll(content, `"`, `""`)
	return []byte(fmt.Sprintf("A%d,%d,0,%s,1,1,N,\"%s\""+lf, x, y, font, text))
}

// EPL GW: GW{x},{y},{widthBytes},{height},{RAW_BINARY}
// Data is raw binary, MSB-first, black=1, white=0.
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

	// Command header: GW{x},{y},{widthBytes},{h},
	// then raw binary data, then LF
	header := fmt.Sprintf("GW%d,%d,%d,%d,", x, y, widthBytes, h)
	cmd := make([]byte, 0, len(header)+len(packed)+1)
	cmd = append(cmd, header...)
	cmd = append(cmd, packed...)
	cmd = append(cmd, lf...)
	return cmd, nil
}
