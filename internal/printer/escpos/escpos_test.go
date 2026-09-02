package escpos

import (
	"context"
	"testing"

	"github.com/timboli111/PrintCat/internal/document"
	"github.com/timboli111/PrintCat/internal/printer"
	"github.com/timboli111/PrintCat/internal/render"
	"github.com/timboli111/PrintCat/internal/render/basic"
)

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
	if len(data) == 0 {
		t.Error("expected non-empty payload")
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

func TestMediaWidthValidation(t *testing.T) {
	encoder := &Encoder{}
	doc := document.New("test", "Test", document.Size{Width: 80000, Height: 100000})
	profile := printer.PrinterProfile{
		DPI:        203,
		MediaWidth: 50000,
	}
	renderer := &basic.Renderer{}

	_, err := encoder.Encode(context.Background(), doc, renderer, profile, nil)
	if err == nil {
		t.Error("expected error for document exceeding media width")
	}
}

func TestCutOption(t *testing.T) {
	encoder := &Encoder{}
	doc := document.New("test", "Test", document.Size{Width: 80000, Height: 100000})
	profile := printer.PrinterProfile{
		DPI:            203,
		SupportsCutter: true,
	}
	renderer := &basic.Renderer{}

	data, err := encoder.Encode(context.Background(), doc, renderer, profile, map[string]string{"cut": "true"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data[len(data)-3] != 0x1D || data[len(data)-2] != 0x56 || data[len(data)-1] != 0x00 {
		t.Error("cut command not found at end")
	}

	profile2 := printer.PrinterProfile{
		DPI:            203,
		SupportsCutter: false,
	}
	data2, err := encoder.Encode(context.Background(), doc, renderer, profile2, map[string]string{"cut": "true"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i := 0; i < len(data2)-2; i++ {
		if data2[i] == 0x1D && data2[i+1] == 0x56 && data2[i+2] == 0x00 {
			t.Error("cut command found when SupportsCutter false")
		}
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
	found := false
	for i := 0; i < len(data)-2; i++ {
		if data[i] == 0x1B && data[i+1] == 0x64 && data[i+2] == 0x05 {
			found = true
			break
		}
	}
	if !found {
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
	m := data[idx+3]
	xL := data[idx+4]
	xH := data[idx+5]
	yL := data[idx+6]
	yH := data[idx+7]
	if m != 0x00 {
		t.Errorf("expected m=0, got %x", m)
	}
	widthBytes := (int(float64(doc.PageSize.Width)/1000.0/25.4*float64(profile.DPI)) + 7) / 8
	heightDots := int(float64(doc.PageSize.Height) / 1000.0 / 25.4 * float64(profile.DPI))
	if int(xL) != (widthBytes&0xFF) || int(xH) != ((widthBytes>>8)&0xFF) {
		t.Errorf("xL/xH mismatch: expected %d/%d, got %d/%d", widthBytes&0xFF, (widthBytes>>8)&0xFF, xL, xH)
	}
	if int(yL) != (heightDots&0xFF) || int(yH) != ((heightDots>>8)&0xFF) {
		t.Errorf("yL/yH mismatch: expected %d/%d, got %d/%d", heightDots&0xFF, (heightDots>>8)&0xFF, yL, yH)
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
	return render.Raster{
		Width:  m.w,
		Height: m.h,
		Pixels: m.pix,
	}, nil
}

func TestRasterDimensionMismatch(t *testing.T) {
	encoder := &Encoder{}
	doc := document.New("test", "Test", document.Size{Width: 80000, Height: 100000})
	profile := printer.PrinterProfile{DPI: 203}
	mock := &mockRenderer{
		w:   100,
		h:   200,
		pix: make([]byte, 100*200),
	}
	_, err := encoder.Encode(context.Background(), doc, mock, profile, nil)
	if err == nil {
		t.Error("expected error for raster dimension mismatch")
	}
}

// ESC/POS GS v 0 format: 1D 76 30 m xL xH yL yH d1...dk
// Bit packing: MSB first, bit 7 = leftmost pixel
func TestBitPackingPattern(t *testing.T) {
	encoder := &Encoder{}
	doc := document.New("test", "Test", document.Size{Width: 1001, Height: 126})
	profile := printer.PrinterProfile{DPI: 203}

	pix := []byte{0, 255, 0, 255, 0, 255, 0, 255}
	mock := &mockRenderer{
		w:   8,
		h:   1,
		pix: pix,
	}
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
	if len(data) < idx+9 {
		t.Fatal("raster data too short")
	}
	packedByte := data[idx+8]
	expected := byte(0xAA)
	if packedByte != expected {
		t.Errorf("bit-packing mismatch: expected 0x%02x, got 0x%02x", expected, packedByte)
	}
}
