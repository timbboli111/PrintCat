// Package trim provides conservative, protocol-neutral raster trimming.
package trim

import (
	"fmt"

	"github.com/timboli111/PrintCat/internal/render"
)

const minimumInkPixelsInRow = 2

// Bottom removes only blank rows below the last confidently detected content
// row. Pixels darker than threshold are treated as ink, matching the encoder's
// binarization rule. One-pixel-wide content is accepted when it continues for
// at least two adjacent rows. Isolated one-pixel rows are treated as
// ambiguous: they are safe if the retained margin already includes them;
// otherwise the full raster is kept. Pages containing only such sparse
// pixels are also left unchanged.
//
// marginRows is the number of blank rows to retain below the detected content.
// The returned raster reuses the input pixel storage and does not mutate it.
func Bottom(raster render.Raster, threshold, marginRows int) (render.Raster, bool, error) {
	if raster.Width <= 0 || raster.Height <= 0 {
		return render.Raster{}, false, fmt.Errorf("invalid raster dimensions: %dx%d", raster.Width, raster.Height)
	}
	if marginRows < 0 {
		return render.Raster{}, false, fmt.Errorf("bottom margin must not be negative: %d", marginRows)
	}
	pixelCount, ok := checkedProduct(raster.Width, raster.Height)
	if !ok {
		return render.Raster{}, false, fmt.Errorf("raster dimensions overflow: %dx%d", raster.Width, raster.Height)
	}
	if len(raster.Pixels) != pixelCount {
		return render.Raster{}, false, fmt.Errorf("raster pixel length mismatch: expected %d, got %d", pixelCount, len(raster.Pixels))
	}

	lastContentRow := -1
	lastSinglePixelRow := -1
	hasInk := false
	singlePixelRunStart := -1
	for y := 0; y < raster.Height; y++ {
		rowStart := y * raster.Width
		inkPixels := 0
		for x := 0; x < raster.Width; x++ {
			if int(raster.Pixels[rowStart+x]) < threshold {
				inkPixels++
				hasInk = true
			}
		}

		switch {
		case inkPixels >= minimumInkPixelsInRow:
			lastContentRow = y
			singlePixelRunStart = -1
		case inkPixels == 1:
			lastSinglePixelRow = y
			if singlePixelRunStart < 0 {
				singlePixelRunStart = y
			}
			if y-singlePixelRunStart+1 >= 2 {
				lastContentRow = y
			}
		default:
			singlePixelRunStart = -1
		}
	}

	// Blank pages and pages containing only sparse, isolated pixels do not
	// provide enough evidence to safely shorten the print area.
	if !hasInk || lastContentRow < 0 {
		return raster, false, nil
	}

	contentHeight := lastContentRow + 1
	if marginRows >= raster.Height-contentHeight {
		return raster, false, nil
	}
	trimmedHeight := contentHeight + marginRows
	if trimmedHeight >= raster.Height {
		return raster, false, nil
	}
	// A lone pixel below the detected content is ambiguous: it could be
	// noise or a valid tiny mark. Trim only if the retained margin still
	// includes that pixel; otherwise keep the full page.
	if lastSinglePixelRow >= trimmedHeight {
		return raster, false, nil
	}
	trimmed := raster
	trimmed.Height = trimmedHeight
	trimmed.Pixels = raster.Pixels[:raster.Width*trimmedHeight]
	return trimmed, true, nil
}

func checkedProduct(a, b int) (int, bool) {
	maxInt := int(^uint(0) >> 1)
	if a <= 0 || b <= 0 || a > maxInt/b {
		return 0, false
	}
	return a * b, true
}
