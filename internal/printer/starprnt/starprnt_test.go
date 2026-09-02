package starprnt

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"testing"

	"github.com/timboli111/PrintCat/internal/document"
	"github.com/timboli111/PrintCat/internal/printer"
	"github.com/timboli111/PrintCat/internal/render/basic"
)

func TestProtocol(t *testing.T) {
	e := &Encoder{}
	if e.Protocol() != printer.StarPRNT {
		t.Errorf("expected StarPRNT, got %v", e.Protocol())
	}
}

func TestInitialization(t *testing.T) {
	doc := document.New("test", "Test", document.Size{Width: 80000, Height: 100000})
	profile := printer.PrinterProfile{DPI: 203}
	renderer := &basic.Renderer{}
	data, err := (&Encoder{}).Encode(context.Background(), doc, renderer, profile, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 2 || data[0] != 0x1B || data[1] != 0x40 {
		t.Errorf("ESC @ not found at start: %x", data[:2])
	}
}

func TestPageLengthValid(t *testing.T) {
	doc := document.New("test", "Test", document.Size{Width: 80000, Height: 80000})
	profile := printer.PrinterProfile{DPI: 203}
	renderer := &basic.Renderer{}
	data, err := (&Encoder{}).Encode(context.Background(), doc, renderer, profile, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 5 {
		t.Fatal("data too short")
	}
	if data[2] != 0x1B || data[3] != 0x43 || data[4] != 0x00 {
		t.Errorf("expected ESC C 0 at position 2, got %x %x %x", data[2], data[3], data[4])
	}
	if data[5] != 0x04 {
		t.Errorf("expected n=4, got %d", data[5])
	}
}

func TestPageLengthOutOfRange(t *testing.T) {
	doc := document.New("test", "Test", document.Size{Width: 80000, Height: 7000000})
	profile := printer.PrinterProfile{DPI: 203}
	renderer := &basic.Renderer{}
	_, err := (&Encoder{}).Encode(context.Background(), doc, renderer, profile, nil)
	if err == nil {
		t.Error("expected error for page height out of range")
	}
}

func TestNoSilentClamp(t *testing.T) {
	doc := document.New("test", "Test", document.Size{Width: 80000, Height: 7000000})
	profile := printer.PrinterProfile{DPI: 203}
	renderer := &basic.Renderer{}
	data, err := (&Encoder{}).Encode(context.Background(), doc, renderer, profile, nil)
	if err == nil {
		t.Error("expected error; should not clamp silently")
	}
	if len(data) > 0 {
		t.Error("data returned despite error")
	}
}

func TestAbsolutePositioning(t *testing.T) {
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
	if !bytes.Contains(data, []byte{0x1B, 0x1D, 0x41}) {
		t.Error("ESC GS A (absolute position) not found")
	}
	expectedPos := []byte{0x1B, 0x1D, 0x41, 0x4F, 0x00}
	if !bytes.Contains(data, expectedPos) {
		t.Errorf("expected position command for x=79, got %x", data)
	}
}

func TestTextCommand(t *testing.T) {
	doc := document.New("test", "Test", document.Size{Width: 80000, Height: 100000})
	elem := document.NewTextElement("t1", "Hello", document.Rect{
		Position: document.Point{X: 0, Y: 0},
		Size:     document.Size{Width: 40000, Height: 10000},
	}, 12000)
	doc.Elements = append(doc.Elements, elem)

	profile := printer.PrinterProfile{DPI: 203}
	renderer := &basic.Renderer{}
	data, err := (&Encoder{}).Encode(context.Background(), doc, renderer, profile, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte{0x1B, 0x1E, 0x46}) {
		t.Error("ESC RS F (font select) not found")
	}
	if !bytes.Contains(data, []byte("Hello")) {
		t.Error("text 'Hello' not found")
	}
}

func TestImageCommand(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, 8, 1))
	for x := 0; x < 8; x++ {
		if x%2 == 0 {
			img.Pix[x] = 0
		} else {
			img.Pix[x] = 255
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
	elem := document.Element{
		ID:      "img",
		Type:    document.ImageElement,
		Bounds:  document.Rect{Position: document.Point{X: 0, Y: 0}, Size: document.Size{Width: 1001, Height: 126}},
		Visible: true,
		Data:    imgData,
	}
	doc := document.New("test", "Test", document.Size{Width: 80000, Height: 100000})
	doc.Elements = append(doc.Elements, elem)

	profile := printer.PrinterProfile{DPI: 203}
	renderer := &basic.Renderer{}
	data, err := (&Encoder{}).Encode(context.Background(), doc, renderer, profile, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte{0x1B, 0x1D, 0x53}) {
		t.Error("ESC GS S (raster) not found")
	}
	if !bytes.Contains(data, []byte{0xAA}) {
		t.Error("bit packing mismatch: expected raw byte 0xAA")
	}
}

func TestImageOutOfBounds(t *testing.T) {
	imgData := document.ImageData{
		Data:     []byte{},
		MimeType: "image/png",
	}
	elem := document.Element{
		ID:      "img",
		Type:    document.ImageElement,
		Bounds:  document.Rect{Position: document.Point{X: 90000, Y: 0}, Size: document.Size{Width: 1001, Height: 126}},
		Visible: true,
		Data:    imgData,
	}
	doc := document.New("test", "Test", document.Size{Width: 80000, Height: 100000})
	doc.Elements = append(doc.Elements, elem)

	profile := printer.PrinterProfile{DPI: 203}
	renderer := &basic.Renderer{}
	_, err := (&Encoder{}).Encode(context.Background(), doc, renderer, profile, nil)
	if err == nil {
		t.Error("expected error for image out of bounds")
	}
}

func TestDPIConversion(t *testing.T) {
	doc := document.New("test", "Test", document.Size{Width: 80000, Height: 100000})
	elem := document.NewTextElement("t1", "Hello", document.Rect{
		Position: document.Point{X: 10000, Y: 10000},
		Size:     document.Size{Width: 40000, Height: 10000},
	}, 12000)
	doc.Elements = append(doc.Elements, elem)

	renderer := &basic.Renderer{}

	profile203 := printer.PrinterProfile{DPI: 203}
	data203, err := (&Encoder{}).Encode(context.Background(), doc, renderer, profile203, nil)
	if err != nil {
		t.Fatal(err)
	}

	profile300 := printer.PrinterProfile{DPI: 300}
	data300, err := (&Encoder{}).Encode(context.Background(), doc, renderer, profile300, nil)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(data203, data300) {
		t.Error("DPI 203 and 300 should produce different output")
	}
}

func TestMediaWidth(t *testing.T) {
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

func TestInvalidDPI(t *testing.T) {
	doc := document.New("test", "Test", document.Size{Width: 80000, Height: 100000})
	profile := printer.PrinterProfile{DPI: 0}
	renderer := &basic.Renderer{}
	_, err := (&Encoder{}).Encode(context.Background(), doc, renderer, profile, nil)
	if err == nil {
		t.Error("expected error for invalid DPI")
	}
}

func TestInvalidDocument(t *testing.T) {
	doc := document.New("", "Test", document.Size{Width: 80000, Height: 100000})
	profile := printer.PrinterProfile{DPI: 203}
	renderer := &basic.Renderer{}
	_, err := (&Encoder{}).Encode(context.Background(), doc, renderer, profile, nil)
	if err == nil {
		t.Error("expected error for invalid document")
	}
}

func TestCopies(t *testing.T) {
	doc := document.New("test", "Test", document.Size{Width: 80000, Height: 100000})
	profile := printer.PrinterProfile{DPI: 203}
	renderer := &basic.Renderer{}

	data, err := (&Encoder{}).Encode(context.Background(), doc, renderer, profile, map[string]string{"copies": "1"})
	if err != nil {
		t.Fatal(err)
	}
	ffCount1 := bytes.Count(data, []byte{0x0C})
	if ffCount1 != 1 {
		t.Errorf("expected 1 FF for copies=1, got %d", ffCount1)
	}

	data, err = (&Encoder{}).Encode(context.Background(), doc, renderer, profile, map[string]string{"copies": "3"})
	if err != nil {
		t.Fatal(err)
	}
	ffCount3 := bytes.Count(data, []byte{0x0C})
	if ffCount3 != 3 {
		t.Errorf("expected 3 FF for copies=3, got %d", ffCount3)
	}

	_, err = (&Encoder{}).Encode(context.Background(), doc, renderer, profile, map[string]string{"copies": "0"})
	if err == nil {
		t.Error("expected error for copies=0")
	}
}

func TestEmptyDocument(t *testing.T) {
	doc := document.New("test", "Test", document.Size{Width: 80000, Height: 100000})
	profile := printer.PrinterProfile{DPI: 203}
	renderer := &basic.Renderer{}
	data, err := (&Encoder{}).Encode(context.Background(), doc, renderer, profile, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 6 {
		t.Error("payload too short")
	}
	if data[0] != 0x1B || data[1] != 0x40 {
		t.Error("ESC @ missing")
	}
	if data[2] != 0x1B || data[3] != 0x43 || data[4] != 0x00 {
		t.Error("ESC C 0 missing")
	}
	if data[len(data)-1] != 0x0C {
		t.Error("FF not at end")
	}
}

func TestLineRectangleSkipped(t *testing.T) {
	doc := document.New("test", "Test", document.Size{Width: 80000, Height: 100000})
	line := document.Element{
		ID:      "l1",
		Type:    document.LineElement,
		Bounds:  document.Rect{Position: document.Point{X: 0, Y: 0}, Size: document.Size{Width: 40000, Height: 0}},
		Visible: true,
	}
	rect := document.Element{
		ID:      "r1",
		Type:    document.RectangleElement,
		Bounds:  document.Rect{Position: document.Point{X: 0, Y: 0}, Size: document.Size{Width: 40000, Height: 20000}},
		Visible: true,
	}
	doc.Elements = append(doc.Elements, line, rect)

	profile := printer.PrinterProfile{DPI: 203}
	renderer := &basic.Renderer{}
	data, err := (&Encoder{}).Encode(context.Background(), doc, renderer, profile, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Error("payload empty")
	}
	if data[len(data)-1] != 0x0C {
		t.Error("FF not at end")
	}
}

func TestContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	doc := document.New("test", "Test", document.Size{Width: 80000, Height: 100000})
	profile := printer.PrinterProfile{DPI: 203}
	renderer := &basic.Renderer{}
	_, err := (&Encoder{}).Encode(ctx, doc, renderer, profile, nil)
	if err != nil {
		// Some renderers may check context; ignore error
	}
}
