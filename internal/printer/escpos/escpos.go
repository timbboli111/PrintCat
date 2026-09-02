package escpos

import (
	"context"
	"fmt"
	"strconv"

	"github.com/timboli111/PrintCat/internal/document"
	"github.com/timboli111/PrintCat/internal/printer"
	"github.com/timboli111/PrintCat/internal/render"
)

type Encoder struct{}

func (e *Encoder) Protocol() printer.Protocol {
	return printer.ESCPOS
}

func (e *Encoder) Encode(ctx context.Context, doc document.Document, renderer render.Renderer, profile printer.PrinterProfile, options map[string]string) ([]byte, error) {
	if err := doc.Validate(); err != nil {
		return nil, fmt.Errorf("invalid document: %w", err)
	}
	if profile.DPI <= 0 {
		return nil, fmt.Errorf("invalid DPI: %d", profile.DPI)
	}

	docWidthDots := int(float64(doc.PageSize.Width) / 1000.0 / 25.4 * float64(profile.DPI))
	docHeightDots := int(float64(doc.PageSize.Height) / 1000.0 / 25.4 * float64(profile.DPI))

	if docWidthDots <= 0 || docHeightDots <= 0 {
		return nil, fmt.Errorf("document dimensions too small: %dx%d dots", docWidthDots, docHeightDots)
	}

	if profile.MediaWidth > 0 {
		mediaWidthDots := int(float64(profile.MediaWidth) / 1000.0 / 25.4 * float64(profile.DPI))
		if docWidthDots > mediaWidthDots {
			return nil, fmt.Errorf("document width (%d dots) exceeds media width (%d dots)", docWidthDots, mediaWidthDots)
		}
	}

	target := render.Target{
		DPI:        profile.DPI,
		Monochrome: true,
	}
	raster, err := renderer.Render(ctx, doc, target)
	if err != nil {
		return nil, fmt.Errorf("render failed: %w", err)
	}

	if raster.Width != docWidthDots || raster.Height != docHeightDots {
		return nil, fmt.Errorf("raster dimensions mismatch: expected %dx%d, got %dx%d", docWidthDots, docHeightDots, raster.Width, raster.Height)
	}
	if len(raster.Pixels) != raster.Width*raster.Height {
		return nil, fmt.Errorf("raster pixel length mismatch: expected %d, got %d", raster.Width*raster.Height, len(raster.Pixels))
	}

	widthBytes := (docWidthDots + 7) / 8
	packed := make([]byte, widthBytes*docHeightDots)
	for y := 0; y < docHeightDots; y++ {
		for x := 0; x < docWidthDots; x++ {
			idx := y*docWidthDots + x
			bit := 0
			if raster.Pixels[idx] < 128 {
				bit = 1
			}
			byteIdx := y*widthBytes + x/8
			bitPos := 7 - (x % 8)
			if bit == 1 {
				packed[byteIdx] |= 1 << bitPos
			}
		}
	}

	var cmd []byte
	cmd = append(cmd, 0x1B, 0x40)

	xL := byte(widthBytes & 0xFF)
	xH := byte((widthBytes >> 8) & 0xFF)
	yL := byte(docHeightDots & 0xFF)
	yH := byte((docHeightDots >> 8) & 0xFF)
	cmd = append(cmd, 0x1D, 0x76, 0x30, 0x00, xL, xH, yL, yH)
	cmd = append(cmd, packed...)

	feedLines := 0
	if feedStr, ok := options["feed"]; ok {
		if n, err := strconv.Atoi(feedStr); err == nil && n > 0 {
			feedLines = n
		}
	}
	if feedLines > 0 {
		cmd = append(cmd, 0x1B, 0x64, byte(feedLines))
	}

	if cut, ok := options["cut"]; ok && cut == "true" && profile.SupportsCutter {
		cmd = append(cmd, 0x1D, 0x56, 0x00)
	}

	return cmd, nil
}
