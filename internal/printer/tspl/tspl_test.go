package tspl

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"testing"

	"github.com/timboli111/PrintCat/internal/document"
	"github.com/timboli111/PrintCat/internal/printer"
	"github.com/timboli111/PrintCat/internal/render/basic"
)

func TestProtocol(t *testing.T) {
	e := &Encoder{}
	if e.Protocol() != printer.TSPL {
		t.Errorf("expected TSPL, got %v", e.Protocol())
	}
}

func TestSizeCommand(t *testing.T) {
	doc := document.New("test", "Test", document.Size{Width: 80000, Height: 100000})
	profile := printer.PrinterProfile{DPI: 203}
	renderer := &basic.Renderer{}
	data, err := (&Encoder{}).Encode(context.Background(), doc, renderer, profile, nil)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !contains(s, "SIZE") {
		t.Error("SIZE command missing")
	}
	widthUm := float64(80000)
	heightUm := float64(100000)
	dpi := float64(profile.DPI)
	widthDots := int(widthUm / 1000.0 / 25.4 * dpi)
	heightDots := int(heightUm / 1000.0 / 25.4 * dpi)
	expectedSize := fmt.Sprintf("SIZE %d,%d", widthDots, heightDots)
	if !contains(s, expectedSize) {
		t.Errorf("SIZE expected %s, got %s", expectedSize, s)
	}
	if !contains(s, "GAP 0,0") {
		t.Error("GAP 0,0 missing")
	}
	if !contains(s, "CLS") {
		t.Error("CLS missing")
	}
	if !contains(s, "PRINT") {
		t.Error("PRINT missing")
	}
}

func TestTextCommand(t *testing.T) {
	doc := document.New("test", "Test", document.Size{Width: 80000, Height: 100000})
	elem := document.NewTextElement("t1", "Hello", document.Rect{
		Position: document.Point{X: 10000, Y: 10000},
		Size:     document.Size{Width: 40000, Height: 10000},
	}, 12000)
	doc.Elements = append(doc.Elements, elem)

	profile := printer.PrinterProfile{DPI: 203}
	renderer := &basic.Renderer{}
	data, err := (&Encoder{}).Encode(context.Background(), doc, renderer, profile, nil)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !contains(s, "TEXT") {
		t.Error("TEXT command missing")
	}
}

func TestBitmapCommand(t *testing.T) {
	// Create a 8x1 PNG image with pattern: black, white, black, white, ...
	img := image.NewGray(image.Rect(0, 0, 8, 1))
	for x := 0; x < 8; x++ {
		if x%2 == 0 {
			img.Pix[x] = 0 // black
		} else {
			img.Pix[x] = 255 // white
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	imgData := document.ImageData{
		Data:     buf.Bytes(),
		MimeType: "image/png",
	}
	// 1001 µm ≈ 8 dots, 126 µm ≈ 1 dot at 203 DPI
	elem := document.Element{
		ID:      "img",
		Type:    document.ImageElement,
		Bounds:  document.Rect{Position: document.Point{X: 10000, Y: 10000}, Size: document.Size{Width: 1001, Height: 126}},
		Visible: true,
		Data:    imgData,
	}
	doc := document.New("test", "Test", document.Size{Width: 80000, Height: 100000})
	doc.Elements = append(doc.Elements, elem)

	renderer := &basic.Renderer{}
	profile := printer.PrinterProfile{DPI: 203}
	data, err := (&Encoder{}).Encode(context.Background(), doc, renderer, profile, nil)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !contains(s, "BITMAP") {
		t.Error("BITMAP command missing")
	}
	// Alternating pattern should pack to 0xAA
	if !contains(s, "AA") {
		t.Error("bit packing mismatch: expected AA in hex data")
	}
}

func TestCopiesOption(t *testing.T) {
	doc := document.New("test", "Test", document.Size{Width: 80000, Height: 100000})
	profile := printer.PrinterProfile{DPI: 203}
	renderer := &basic.Renderer{}
	options := map[string]string{"copies": "3"}
	data, err := (&Encoder{}).Encode(context.Background(), doc, renderer, profile, options)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !contains(s, "PRINT 3,1") {
		t.Error("PRINT with copies=3 not found")
	}
}

func TestInvalidDPI(t *testing.T) {
	doc := document.New("test", "Test", document.Size{Width: 80000, Height: 100000})
	profile := printer.PrinterProfile{DPI: 0}
	renderer := &basic.Renderer{}
	_, err := (&Encoder{}).Encode(context.Background(), doc, renderer, profile, nil)
	if err == nil {
		t.Error("expected error for invalid DPI")
	}
}

func TestMediaWidthValidation(t *testing.T) {
	doc := document.New("test", "Test", document.Size{Width: 80000, Height: 100000})
	profile := printer.PrinterProfile{
		DPI:        203,
		MediaWidth: 50000,
	}
	renderer := &basic.Renderer{}
	_, err := (&Encoder{}).Encode(context.Background(), doc, renderer, profile, nil)
	if err == nil {
		t.Error("expected error for document exceeding media width")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0) || (len(s) > len(sub) && (s[:len(sub)] == sub || s[len(s)-len(sub):] == sub || (len(s) > len(sub)+1 && contains(s[1:], sub))))
}
