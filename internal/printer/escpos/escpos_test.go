package escpos

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/timboli111/PrintCat/internal/document"
	"github.com/timboli111/PrintCat/internal/printer"
	"github.com/timboli111/PrintCat/internal/render"
	"github.com/timboli111/PrintCat/internal/render/basic"
)

func umToDots(um document.Unit, dpi int) int {
	v := float64(um) / 1000.0 / 25.4 * float64(dpi)
	return int(v)
}

func TestProtocolRegistration(t *testing.T) {
	encoder := &Encoder{}
	if encoder.Protocol() != printer.ESCPOS {
		t.Errorf("expected protocol ESCPOS, got %v", encoder.Protocol())
	}
}

func TestEncodeEmptyDocument(t *testing.T) {
	encoder := &Encoder{}
	doc := document.New("test", "Test", document.Size{Width: 80000, Height: 100000})
	profile := printer.PrinterProfile{DPI: 203}
	renderer := &basic.Renderer{}

	data, err := encoder.Encode(context.Background(), doc, renderer, profile, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty payload")
	}
	if data[0] != 0x1B || data[1] != 0x40 {
		t.Errorf("expected ESC @, got %x %x", data[0], data[1])
	}
}

func TestEncodeWithText(t *testing.T) {
	encoder := &Encoder{}
	doc := document.New("test", "Test", document.Size{Width: 80000, Height: 100000})
	elem := document.NewTextElement("t1", "Hello", document.Rect{
		Position: document.Point{X: 10000, Y: 10000},
		Size:     document.Size{Width: 40000, Height: 10000},
	}, 12000)
	doc.Elements = append(doc.Elements, elem)

	profile := printer.PrinterProfile{DPI: 203}
	renderer := &basic.Renderer{}

	data, err := encoder.Encode(context.Background(), doc, renderer, profile, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for i := 0; i < len(data)-2; i++ {
		if data[i] == 0x1D && data[i+1] == 0x76 && data[i+2] == 0x30 {
			found = true
			break
		}
	}
	if !found {
		t.Error("raster command (GS v 0) not found")
	}
}

func extractRasterWidthBytes(t *testing.T, data []byte) int {
	t.Helper()
	for i := 0; i+7 < len(data); i++ {
		if data[i] == 0x1D && data[i+1] == 0x76 && data[i+2] == 0x30 {
			xL := int(data[i+4])
			xH := int(data[i+5])
			return xL | (xH << 8)
		}
	}
	t.Fatal("GS v 0 header not found")
	return 0
}

func extractRasterHeader(t *testing.T, data []byte) (offset, widthBytes, height int) {
	t.Helper()
	for i := 0; i+7 < len(data); i++ {
		if data[i] == 0x1D && data[i+1] == 0x76 && data[i+2] == 0x30 {
			widthBytes = int(data[i+4]) | (int(data[i+5]) << 8)
			height = int(data[i+6]) | (int(data[i+7]) << 8)
			return i, widthBytes, height
		}
	}
	t.Fatal("GS v 0 header not found")
	return 0, 0, 0
}

func TestMediaWidthCropsRaster(t *testing.T) {
	encoder := &Encoder{}
	doc := document.New("test", "Test", document.Size{Width: 80000, Height: 100000})
	profile := printer.PrinterProfile{DPI: 203, MediaWidth: 48000}
	renderer := &basic.Renderer{}

	data, err := encoder.Encode(context.Background(), doc, renderer, profile, nil)
	if err != nil {
		t.Fatalf("Encode must not error when document exceeds MediaWidth: %v", err)
	}

	gotWidthBytes := extractRasterWidthBytes(t, data)
	wantWidthDots := umToDots(document.Unit(profile.MediaWidth), profile.DPI)
	wantWidthBytes := (wantWidthDots + 7) / 8
	if gotWidthBytes != wantWidthBytes {
		t.Errorf("widthBytes = %d, want %d", gotWidthBytes, wantWidthBytes)
	}
	docWidthDots := umToDots(doc.PageSize.Width, profile.DPI)
	if wantWidthDots >= docWidthDots {
		t.Fatalf("setup error: media %d >= doc %d", wantWidthDots, docWidthDots)
	}
}

func TestMediaWidthDoesNotAffectNarrowerDocument(t *testing.T) {
	encoder := &Encoder{}
	doc := document.New("test", "Test", document.Size{Width: 40000, Height: 100000})
	profile := printer.PrinterProfile{DPI: 203, MediaWidth: 48000}
	renderer := &basic.Renderer{}

	data, err := encoder.Encode(context.Background(), doc, renderer, profile, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	gotWidthBytes := extractRasterWidthBytes(t, data)
	wantWidthDots := umToDots(doc.PageSize.Width, profile.DPI)
	if gotWidthBytes != (wantWidthDots+7)/8 {
		t.Errorf("widthBytes = %d, want %d", gotWidthBytes, (wantWidthDots+7)/8)
	}
}

func TestMediaWidthZeroKeepsLegacyBehavior(t *testing.T) {
	encoder := &Encoder{}
	doc := document.New("test", "Test", document.Size{Width: 80000, Height: 100000})
	profile := printer.PrinterProfile{DPI: 203, MediaWidth: 0}
	renderer := &basic.Renderer{}

	data, err := encoder.Encode(context.Background(), doc, renderer, profile, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	gotWidthBytes := extractRasterWidthBytes(t, data)
	wantWidthDots := umToDots(doc.PageSize.Width, profile.DPI)
	if gotWidthBytes != (wantWidthDots+7)/8 {
		t.Errorf("widthBytes = %d, want %d", gotWidthBytes, (wantWidthDots+7)/8)
	}
}

func TestMediaWidthEqualToDocumentWidth(t *testing.T) {
	encoder := &Encoder{}
	doc := document.New("test", "Test", document.Size{Width: 48000, Height: 100000})
	profile := printer.PrinterProfile{DPI: 203, MediaWidth: 48000}
	renderer := &basic.Renderer{}

	data, err := encoder.Encode(context.Background(), doc, renderer, profile, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	gotWidthBytes := extractRasterWidthBytes(t, data)
	wantWidthDots := umToDots(doc.PageSize.Width, profile.DPI)
	if gotWidthBytes != (wantWidthDots+7)/8 {
		t.Errorf("widthBytes = %d, want %d", gotWidthBytes, (wantWidthDots+7)/8)
	}
}

// TestDensityDefaultIsUsed verifies that Density == 0 falls back to 160.
// The observation strategy: a full-black source raster (pixels = 0) is
// always black regardless of threshold. So instead we craft a pixel with a
// gray value strictly between two thresholds to distinguish 160 from 180.
// We use a mock renderer that returns gray = 170 for a single pixel.
func TestDensityDefaultIsUsed(t *testing.T) {
	encoder := &Encoder{}
	doc := document.New("test", "Test", document.Size{Width: 254, Height: 254})
	profile := printer.PrinterProfile{DPI: 203}
	mock := &singlePixelMock{gray: 170}
	data, err := encoder.Encode(context.Background(), doc, mock, profile, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// With threshold 160, gray 170 -> white (170 >= 160).
	if !pixelIsWhite(t, data) {
		t.Error("Density <= 0 should fall back to 160, gray 170 must be white")
	}
}

// TestDensityConfigurableAboveDefault verifies that when Density is set
// higher than the fallback, a gray that would be white at 160 becomes
// black at 200.
func TestDensityConfigurableAboveDefault(t *testing.T) {
	encoder := &Encoder{}
	doc := document.New("test", "Test", document.Size{Width: 254, Height: 254})
	profile := printer.PrinterProfile{DPI: 203, Density: 200}
	mock := &singlePixelMock{gray: 170}
	data, err := encoder.Encode(context.Background(), doc, mock, profile, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// With threshold 200, gray 170 -> black (170 < 200).
	if !pixelIsBlack(t, data) {
		t.Error("Density=200 must binarize gray 170 to black")
	}
}

// TestDensityBelowDefaultTurnsBlackToWhite verifies the opposite direction:
// density 120 rejects gray 130.
func TestDensityBelowDefaultTurnsBlackToWhite(t *testing.T) {
	encoder := &Encoder{}
	doc := document.New("test", "Test", document.Size{Width: 254, Height: 254})
	profile := printer.PrinterProfile{DPI: 203, Density: 120}
	mock := &singlePixelMock{gray: 130}
	data, err := encoder.Encode(context.Background(), doc, mock, profile, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 130 >= 120 -> white.
	if !pixelIsWhite(t, data) {
		t.Error("Density=120 must binarize gray 130 to white")
	}
}

// singlePixelMock returns a 2x2 raster filled with the given gray value.
type singlePixelMock struct {
	gray byte
}

func (m *singlePixelMock) Render(ctx context.Context, doc document.Document, target render.Target) (render.Raster, error) {
	return render.Raster{
		Width:  2,
		Height: 2,
		Pixels: []byte{
			m.gray, m.gray,
			m.gray, m.gray,
		},
	}, nil
}

// rasterPixelsAt parses the GS v 0 payload from data and returns the
// packed raster bytes plus the byte width. The GS v 0 format we emit is:
//
//	0x1D 0x76 0x30 0x00 xL xH yL yH <packed>
func rasterPixelsAt(t *testing.T, data []byte) ([]byte, int) {
	t.Helper()
	for i := 0; i+7 < len(data); i++ {
		if data[i] == 0x1D && data[i+1] == 0x76 && data[i+2] == 0x30 {
			widthBytes := int(data[i+4]) | (int(data[i+5]) << 8)
			return data[i+8:], widthBytes
		}
	}
	t.Fatal("GS v 0 not found")
	return nil, 0
}

// pixelIsBlack reports whether the top-left pixel of a 1x1 raster is
// emitted as a black dot (MSB of first byte set).
func pixelIsBlack(t *testing.T, data []byte) bool {
	t.Helper()
	packed, _ := rasterPixelsAt(t, data)
	if len(packed) < 1 {
		t.Fatal("raster has no bytes")
	}
	return packed[0]&0x80 != 0
}

func pixelIsWhite(t *testing.T, data []byte) bool {
	return !pixelIsBlack(t, data)
}

func TestCutOption(t *testing.T) {
	encoder := &Encoder{}
	doc := document.New("test", "Test", document.Size{Width: 80000, Height: 100000})
	profile := printer.PrinterProfile{DPI: 203, SupportsCutter: true}
	renderer := &basic.Renderer{}

	data, err := encoder.Encode(context.Background(), doc, renderer, profile, map[string]string{"cut": "true"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data[len(data)-3] != 0x1D || data[len(data)-2] != 0x56 || data[len(data)-1] != 0x00 {
		t.Error("cut command not found at end")
	}

	profile2 := printer.PrinterProfile{DPI: 203, SupportsCutter: false}
	data2, err := encoder.Encode(context.Background(), doc, renderer, profile2, map[string]string{"cut": "true"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bytes.Contains(data2, []byte{0x1D, 0x56, 0x00}) {
		t.Error("cut command found when SupportsCutter false")
	}
}

func TestFeedOption(t *testing.T) {
	encoder := &Encoder{}
	doc := document.New("test", "Test", document.Size{Width: 80000, Height: 100000})
	profile := printer.PrinterProfile{DPI: 203}
	renderer := &basic.Renderer{}

	data, err := encoder.Encode(context.Background(), doc, renderer, profile, map[string]string{"feed": "5"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Contains(data, []byte{0x1B, 0x64, 0x05}) {
		t.Error("feed command ESC d 5 not found")
	}
}

func TestInvalidDPI(t *testing.T) {
	encoder := &Encoder{}
	doc := document.New("test", "Test", document.Size{Width: 80000, Height: 100000})
	profile := printer.PrinterProfile{DPI: 0}
	renderer := &basic.Renderer{}

	_, err := encoder.Encode(context.Background(), doc, renderer, profile, nil)
	if err == nil {
		t.Error("expected error for invalid DPI")
	}
}

func TestGSv0Parameters(t *testing.T) {
	encoder := &Encoder{}
	doc := document.New("test", "Test", document.Size{Width: 80000, Height: 100000})
	profile := printer.PrinterProfile{DPI: 203}
	renderer := &basic.Renderer{}

	data, err := encoder.Encode(context.Background(), doc, renderer, profile, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	idx := -1
	for i := 0; i < len(data)-2; i++ {
		if data[i] == 0x1D && data[i+1] == 0x76 && data[i+2] == 0x30 {
			idx = i
			break
		}
	}
	if idx == -1 {
		t.Fatal("GS v 0 command not found")
	}
	if len(data) < idx+8 {
		t.Fatal("command too short")
	}
	if data[idx+3] != 0x00 {
		t.Errorf("expected m=0, got %x", data[idx+3])
	}
	widthBytes := (umToDots(doc.PageSize.Width, profile.DPI) + 7) / 8
	heightDots := umToDots(doc.PageSize.Height, profile.DPI)
	if int(data[idx+4]) != (widthBytes&0xFF) || int(data[idx+5]) != ((widthBytes>>8)&0xFF) {
		t.Errorf("xL/xH mismatch: expected %d/%d", widthBytes&0xFF, (widthBytes>>8)&0xFF)
	}
	if int(data[idx+6]) != (heightDots&0xFF) || int(data[idx+7]) != ((heightDots>>8)&0xFF) {
		t.Errorf("yL/yH mismatch: expected %d/%d", heightDots&0xFF, (heightDots>>8)&0xFF)
	}
}

func TestZeroWidthDocument(t *testing.T) {
	encoder := &Encoder{}
	doc := document.New("test", "Test", document.Size{Width: 1, Height: 100000})
	profile := printer.PrinterProfile{DPI: 203}
	renderer := &basic.Renderer{}

	_, err := encoder.Encode(context.Background(), doc, renderer, profile, nil)
	if err == nil {
		t.Error("expected error for zero-width document")
	}
}

type mockRenderer struct {
	w, h int
	pix  []byte
	err  error
}

func (m *mockRenderer) Render(ctx context.Context, doc document.Document, target render.Target) (render.Raster, error) {
	if m.err != nil {
		return render.Raster{}, m.err
	}
	return render.Raster{Width: m.w, Height: m.h, Pixels: m.pix}, nil
}

func TestRasterDimensionMismatch(t *testing.T) {
	encoder := &Encoder{}
	doc := document.New("test", "Test", document.Size{Width: 80000, Height: 100000})
	profile := printer.PrinterProfile{DPI: 203}
	mock := &mockRenderer{w: 100, h: 200, pix: make([]byte, 100*200)}
	_, err := encoder.Encode(context.Background(), doc, mock, profile, nil)
	if err == nil {
		t.Error("expected error for raster dimension mismatch")
	}
}

func TestBitPackingPattern(t *testing.T) {
	encoder := &Encoder{}
	doc := document.New("test", "Test", document.Size{Width: 1001, Height: 126})
	profile := printer.PrinterProfile{DPI: 203}
	pix := []byte{0, 255, 0, 255, 0, 255, 0, 255}
	mock := &mockRenderer{w: 8, h: 1, pix: pix}
	data, err := encoder.Encode(context.Background(), doc, mock, profile, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	idx := -1
	for i := 0; i+7 < len(data); i++ {
		if data[i] == 0x1D && data[i+1] == 0x76 && data[i+2] == 0x30 {
			idx = i
			break
		}
	}
	if idx == -1 {
		t.Fatal("GS v 0 command not found")
	}
	if data[idx+8] != 0xAA {
		t.Errorf("bit-packing mismatch: expected 0xAA, got 0x%02x", data[idx+8])
	}
}

func TestSmartTrimDisabledPreservesLegacyBytes(t *testing.T) {
	doc := document.New("test", "Test", document.Size{Width: 800, Height: 400})
	pixels := bytes.Repeat([]byte{255}, 8*4)
	pixels[0] = 0
	data, err := (&Encoder{}).Encode(context.Background(), doc, &mockRenderer{w: 8, h: 4, pix: pixels}, printer.PrinterProfile{DPI: 254}, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{
		0x1B, 0x40,
		0x1D, 0x76, 0x30, 0x00, 0x01, 0x00, 0x04, 0x00,
		0x80, 0x00, 0x00, 0x00,
	}
	if !bytes.Equal(data, want) {
		t.Fatalf("disabled Smart Trim changed legacy output:\n got % x\nwant % x", data, want)
	}
}

func TestSmartTrimHeaderMatchesPackedRasterLength(t *testing.T) {
	doc := document.New("test", "Test", document.Size{Width: 800, Height: 1000}) // 8x10 dots at 254 DPI.
	pixels := bytes.Repeat([]byte{255}, 8*10)
	pixels[1*8+1] = 0
	pixels[4*8+2] = 0
	pixels[4*8+3] = 0
	profile := printer.PrinterProfile{
		DPI:                     254,
		SmartTrimEnabled:        true,
		SmartTrimBottomMarginUm: 100, // One raster row at 254 DPI.
	}
	data, err := (&Encoder{}).Encode(context.Background(), doc, &mockRenderer{w: 8, h: 10, pix: pixels}, profile, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, widthBytes, height := extractRasterHeader(t, data)
	if widthBytes != 1 || height != 6 {
		t.Fatalf("header widthBytes=%d height=%d, want 1x6", widthBytes, height)
	}
	packedLength := widthBytes * height
	if got := len(data) - 10; got != packedLength {
		t.Fatalf("bytes after raster header=%d, header declares %d", got, packedLength)
	}
}

func TestSmartTrimUsesSafeDefaultMargin(t *testing.T) {
	doc := document.New("test", "Test", document.Size{Width: 800, Height: 4000}) // 8x40 dots at 254 DPI.
	pixels := bytes.Repeat([]byte{255}, 8*40)
	pixels[1*8+1] = 0
	pixels[1*8+2] = 0
	data, err := (&Encoder{}).Encode(context.Background(), doc, &mockRenderer{w: 8, h: 40, pix: pixels}, printer.PrinterProfile{
		DPI:              254,
		SmartTrimEnabled: true,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, _, height := extractRasterHeader(t, data)
	if height != 27 { // 2 content rows + ceil(2.5 mm * 254 DPI) = 25 margin rows.
		t.Fatalf("header height=%d, want 27", height)
	}
}

func TestSmartTrimProcessesMultipageContentHeightsIndependently(t *testing.T) {
	encoder := &Encoder{}
	profile := printer.PrinterProfile{DPI: 254, SmartTrimEnabled: true, SmartTrimBottomMarginUm: 100}
	pageCases := []struct {
		name       string
		height     int
		lastInkRow int
		wantHeight int
	}{
		{name: "page one", height: 10, lastInkRow: 1, wantHeight: 3},
		{name: "page two", height: 12, lastInkRow: 4, wantHeight: 6},
	}
	for _, tt := range pageCases {
		t.Run(tt.name, func(t *testing.T) {
			pixels := bytes.Repeat([]byte{255}, 8*tt.height)
			pixels[tt.lastInkRow*8+1] = 0
			pixels[tt.lastInkRow*8+2] = 0
			doc := document.New("page", tt.name, document.Size{Width: 800, Height: document.Unit(tt.height * 100)})
			data, err := encoder.Encode(context.Background(), doc, &mockRenderer{w: 8, h: tt.height, pix: pixels}, profile, nil)
			if err != nil {
				t.Fatal(err)
			}
			_, _, gotHeight := extractRasterHeader(t, data)
			if gotHeight != tt.wantHeight {
				t.Fatalf("header height=%d, want %d", gotHeight, tt.wantHeight)
			}
		})
	}
}

func TestSmartTrimRejectsMarginsOutsideProfileBounds(t *testing.T) {
	doc := document.New("test", "Test", document.Size{Width: 800, Height: 400})
	for _, margin := range []document.Unit{-1, printer.MaxSmartTrimBottomMarginUm + 1} {
		_, err := (&Encoder{}).Encode(context.Background(), doc, &mockRenderer{w: 8, h: 4, pix: bytes.Repeat([]byte{255}, 8*4)}, printer.PrinterProfile{
			DPI:                     254,
			SmartTrimEnabled:        true,
			SmartTrimBottomMarginUm: margin,
		}, nil)
		if err == nil {
			t.Errorf("Encode accepted out-of-range margin %d µm", margin)
		}
	}
}

func TestSmartTrimAcceptsMaximumMargin(t *testing.T) {
	rows, err := smartTrimMarginRows(printer.MaxSmartTrimBottomMarginUm, 203, 1000)
	if err != nil {
		t.Fatalf("maximum Smart Trim margin was rejected: %v", err)
	}
	if rows != 800 {
		t.Fatalf("100 mm at 203 DPI = %d rows, want 800", rows)
	}
}

func TestRasterHeaderDimensionsMustFitUint16(t *testing.T) {
	const heightDots = 1 << 16
	doc := document.New("oversized", "Oversized raster", document.Size{
		Width:  800,
		Height: document.Unit(heightDots * 100),
	})
	_, err := (&Encoder{}).Encode(
		context.Background(),
		doc,
		&mockRenderer{w: 8, h: heightDots, pix: bytes.Repeat([]byte{255}, 8*heightDots)},
		printer.PrinterProfile{DPI: 254},
		nil,
	)
	if err == nil {
		t.Fatal("expected error for raster height exceeding the 16-bit ESC/POS header")
	}
}

func TestSmartTrimCompositesTransparentPNGOverWhite(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 8, 12))
	img.SetNRGBA(1, 2, color.NRGBA{A: 255})
	img.SetNRGBA(2, 2, color.NRGBA{A: 255})
	img.SetNRGBA(1, 4, color.NRGBA{A: 255})
	img.SetNRGBA(2, 4, color.NRGBA{A: 255})
	var pngData bytes.Buffer
	if err := png.Encode(&pngData, img); err != nil {
		t.Fatal(err)
	}
	doc := document.New("test", "Transparent PNG", document.Size{Width: 800, Height: 1200})
	doc.Elements = append(doc.Elements, document.NewImageElement("image", pngData.Bytes(), "image/png", document.Rect{
		Size: document.Size{Width: doc.PageSize.Width, Height: doc.PageSize.Height},
	}))
	renderer := &basic.Renderer{}

	trimmed, err := (&Encoder{}).Encode(context.Background(), doc, renderer, printer.PrinterProfile{
		DPI:                     254,
		SmartTrimEnabled:        true,
		SmartTrimBottomMarginUm: 100,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, _, height := extractRasterHeader(t, trimmed)
	if height != 6 {
		t.Fatalf("transparent PNG trimmed height=%d, want 6", height)
	}

	legacy, err := (&Encoder{}).Encode(context.Background(), doc, renderer, printer.PrinterProfile{DPI: 254}, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, _, legacyHeight := extractRasterHeader(t, legacy)
	if legacyHeight != 12 {
		t.Fatalf("Smart Trim disabled height=%d, want legacy height 12", legacyHeight)
	}
}

func TestSmartTrimDoesNotClipSmallTextNearBottom(t *testing.T) {
	doc := document.New("small-text", "Small text", document.Size{Width: 20_000, Height: 50_000})
	doc.Elements = append(doc.Elements, document.NewTextElement("small", "i", document.Rect{
		Position: document.Point{X: 1_000, Y: 42_000},
		Size:     document.Size{Width: 18_000, Height: 8_000},
	}, 1_800))
	renderer := &basic.Renderer{}
	raster, err := renderer.Render(context.Background(), doc, render.Target{DPI: 254, Monochrome: true})
	if err != nil {
		t.Fatal(err)
	}
	lastInkRow := -1
	for y := 0; y < raster.Height; y++ {
		for x := 0; x < raster.Width; x++ {
			if raster.Pixels[y*raster.Width+x] < defaultBinarizeThreshold {
				lastInkRow = y
				break
			}
		}
	}
	if lastInkRow < 0 {
		t.Fatal("small text did not produce printable pixels")
	}

	data, err := (&Encoder{}).Encode(context.Background(), doc, renderer, printer.PrinterProfile{
		DPI:                     254,
		SmartTrimEnabled:        true,
		SmartTrimBottomMarginUm: 100,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	metrics := printer.AnalyzePayload(printer.ESCPOS, data)
	if !metrics.HasRasterHeader || metrics.RasterHeightDots < lastInkRow+1 {
		t.Fatalf("Smart Trim clipped small text: last ink row=%d header=%+v", lastInkRow, metrics)
	}
}
