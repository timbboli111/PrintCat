package escpos

import (
	"bytes"
	"context"
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
