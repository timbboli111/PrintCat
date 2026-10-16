package trim

import (
	"testing"

	"github.com/timboli111/PrintCat/internal/render"
)

func whiteRaster(width, height int) render.Raster {
	pixels := make([]byte, width*height)
	for i := range pixels {
		pixels[i] = 255
	}
	return render.Raster{Width: width, Height: height, Pixels: pixels}
}

func setPixel(raster *render.Raster, x, y int, gray byte) {
	raster.Pixels[y*raster.Width+x] = gray
}

func TestBottomKeepsBlankRasterUnchanged(t *testing.T) {
	raster := whiteRaster(8, 20)
	got, changed, err := Bottom(raster, 160, 2)
	if err != nil {
		t.Fatal(err)
	}
	if changed || got.Height != raster.Height || len(got.Pixels) != len(raster.Pixels) {
		t.Fatalf("blank raster changed: changed=%t, height=%d", changed, got.Height)
	}
}

func TestBottomTrimsLongWhiteTailAndKeepsMargin(t *testing.T) {
	raster := whiteRaster(8, 30)
	setPixel(&raster, 1, 2, 0)
	setPixel(&raster, 2, 2, 0)
	setPixel(&raster, 6, 5, 0)
	setPixel(&raster, 7, 5, 0)

	got, changed, err := Bottom(raster, 160, 4)
	if err != nil {
		t.Fatal(err)
	}
	if !changed || got.Height != 10 {
		t.Fatalf("trim result changed=%t height=%d, want true and 10", changed, got.Height)
	}
	if len(got.Pixels) != got.Width*got.Height {
		t.Fatalf("pixel count %d does not match %dx%d raster", len(got.Pixels), got.Width, got.Height)
	}
	if raster.Height != 30 {
		t.Fatal("input raster was mutated")
	}
}

func TestBottomUsesPrintThresholdForAntialiasing(t *testing.T) {
	raster := whiteRaster(5, 12)
	setPixel(&raster, 0, 3, 159) // Printed as ink at threshold 160.
	setPixel(&raster, 1, 3, 120)
	setPixel(&raster, 0, 9, 200) // Not printed as ink at threshold 160.
	setPixel(&raster, 1, 9, 220)

	got, changed, err := Bottom(raster, 160, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !changed || got.Height != 5 {
		t.Fatalf("antialias trim changed=%t height=%d, want true and 5", changed, got.Height)
	}
}

func TestBottomKeepsThinHorizontalLine(t *testing.T) {
	raster := whiteRaster(12, 20)
	for x := 2; x < 10; x++ {
		setPixel(&raster, x, 6, 100)
	}

	got, changed, err := Bottom(raster, 160, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !changed || got.Height != 9 {
		t.Fatalf("thin-line trim changed=%t height=%d, want true and 9", changed, got.Height)
	}
}

func TestBottomKeepsSyntheticBarcode(t *testing.T) {
	raster := whiteRaster(20, 24)
	// Vertical bars with varying widths, each spanning several rows.
	for y := 3; y <= 14; y++ {
		for _, x := range []int{1, 2, 6, 10, 11, 12, 18} {
			setPixel(&raster, x, y, 0)
		}
	}

	got, changed, err := Bottom(raster, 160, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !changed || got.Height != 17 {
		t.Fatalf("barcode trim changed=%t height=%d, want true and 17", changed, got.Height)
	}
}

func TestBottomIgnoresIsolatedNoiseAfterContent(t *testing.T) {
	raster := whiteRaster(10, 40)
	setPixel(&raster, 1, 5, 0)
	setPixel(&raster, 2, 5, 0)
	setPixel(&raster, 7, 7, 0) // One isolated dot already covered by the safety margin.

	got, changed, err := Bottom(raster, 160, 3)
	if err != nil {
		t.Fatal(err)
	}
	if !changed || got.Height != 9 {
		t.Fatalf("noise trim changed=%t height=%d, want true and 9", changed, got.Height)
	}
}

func TestBottomFallsBackForIsolatedMarkBeyondMargin(t *testing.T) {
	raster := whiteRaster(10, 40)
	setPixel(&raster, 1, 5, 0)
	setPixel(&raster, 2, 5, 0)
	setPixel(&raster, 7, 36, 0)

	got, changed, err := Bottom(raster, 160, 3)
	if err != nil {
		t.Fatal(err)
	}
	if changed || got.Height != raster.Height {
		t.Fatalf("ambiguous distant pixel must preserve full raster, changed=%t height=%d", changed, got.Height)
	}
}

func TestBottomPreservesContentAtBottomEdge(t *testing.T) {
	raster := whiteRaster(8, 12)
	setPixel(&raster, 2, 11, 0)
	setPixel(&raster, 3, 11, 0)

	got, changed, err := Bottom(raster, 160, 2)
	if err != nil {
		t.Fatal(err)
	}
	if changed || got.Height != raster.Height {
		t.Fatalf("bottom-edge content must retain full raster, changed=%t height=%d", changed, got.Height)
	}
}

func TestBottomLeavesAmbiguousSparsePageUnchanged(t *testing.T) {
	raster := whiteRaster(8, 20)
	setPixel(&raster, 2, 3, 0)
	setPixel(&raster, 5, 12, 0)

	got, changed, err := Bottom(raster, 160, 1)
	if err != nil {
		t.Fatal(err)
	}
	if changed || got.Height != raster.Height {
		t.Fatalf("ambiguous sparse raster changed: changed=%t height=%d", changed, got.Height)
	}
}

func TestBottomKeepsContinuousSinglePixelStroke(t *testing.T) {
	raster := whiteRaster(5, 18)
	for y := 4; y <= 8; y++ {
		setPixel(&raster, 2, y, 80)
	}

	got, changed, err := Bottom(raster, 160, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !changed || got.Height != 11 {
		t.Fatalf("single-pixel stroke changed=%t height=%d, want true and 11", changed, got.Height)
	}
}

func TestBottomRejectsMalformedAndOverflowingRaster(t *testing.T) {
	tests := []struct {
		name   string
		raster render.Raster
	}{
		{name: "zero dimensions", raster: render.Raster{}},
		{name: "length mismatch", raster: render.Raster{Width: 2, Height: 2, Pixels: []byte{255}}},
		{name: "overflow", raster: render.Raster{Width: int(^uint(0) >> 1), Height: 2}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, err := Bottom(tt.raster, 160, 0); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
	if _, _, err := Bottom(whiteRaster(2, 2), 160, -1); err == nil {
		t.Fatal("expected negative margin error")
	}
}
