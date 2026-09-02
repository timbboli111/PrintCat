package zpl

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"strconv"
	"strings"
	"testing"

	"github.com/timboli111/PrintCat/internal/document"
	"github.com/timboli111/PrintCat/internal/printer"
	"github.com/timboli111/PrintCat/internal/render/basic"
)

func TestProtocol(t *testing.T) {
	e := &Encoder{}
	if e.Protocol() != printer.ZPL {
		t.Errorf("expected ZPL, got %v", e.Protocol())
	}
}

func TestLabelSize(t *testing.T) {
	doc := document.New("test", "Test", document.Size{Width: 80000, Height: 100000})
	profile := printer.PrinterProfile{DPI: 203}
	renderer := &basic.Renderer{}
	data, err := (&Encoder{}).Encode(context.Background(), doc, renderer, profile, nil)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, "^XA") {
		t.Error("^XA missing")
	}
	widthUm := float64(80000)
	heightUm := float64(100000)
	dpi := float64(profile.DPI)
	widthDots := int(widthUm / 1000.0 / 25.4 * dpi)
	heightDots := int(heightUm / 1000.0 / 25.4 * dpi)
	if !strings.Contains(s, "^PW"+strconv.Itoa(widthDots)) {
		t.Errorf("^PW%d missing", widthDots)
	}
	if !strings.Contains(s, "^LL"+strconv.Itoa(heightDots)) {
		t.Errorf("^LL%d missing", heightDots)
	}
	if !strings.Contains(s, "^XZ") {
		t.Error("^XZ missing")
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
	if !strings.Contains(s, "^FO") {
		t.Error("^FO missing")
	}
	if !strings.Contains(s, "^A0") {
		t.Error("^A0 missing")
	}
	if !strings.Contains(s, "^FDHello^FS") {
		t.Error("text content missing")
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
	if !strings.Contains(s, "^GFA") {
		t.Error("^GFA missing")
	}
	if !strings.Contains(s, "AA") {
		t.Error("bit packing mismatch: expected AA in hex data")
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

	renderer := &basic.Renderer{}
	profile := printer.PrinterProfile{DPI: 203}
	_, err := (&Encoder{}).Encode(context.Background(), doc, renderer, profile, nil)
	if err == nil {
		t.Error("expected error for image out of bounds")
	}
}

func TestLineElement(t *testing.T) {
	doc := document.New("test", "Test", document.Size{Width: 80000, Height: 100000})
	line := document.Element{
		ID:      "l1",
		Type:    document.LineElement,
		Bounds:  document.Rect{Position: document.Point{X: 10000, Y: 10000}, Size: document.Size{Width: 40000, Height: 0}},
		Visible: true,
	}
	doc.Elements = append(doc.Elements, line)

	profile := printer.PrinterProfile{DPI: 203}
	renderer := &basic.Renderer{}
	data, err := (&Encoder{}).Encode(context.Background(), doc, renderer, profile, nil)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, "^GB") {
		t.Error("^GB missing for line")
	}
}

func TestRectangleElement(t *testing.T) {
	doc := document.New("test", "Test", document.Size{Width: 80000, Height: 100000})
	rect := document.Element{
		ID:      "r1",
		Type:    document.RectangleElement,
		Bounds:  document.Rect{Position: document.Point{X: 10000, Y: 10000}, Size: document.Size{Width: 40000, Height: 20000}},
		Visible: true,
	}
	doc.Elements = append(doc.Elements, rect)

	profile := printer.PrinterProfile{DPI: 203}
	renderer := &basic.Renderer{}
	data, err := (&Encoder{}).Encode(context.Background(), doc, renderer, profile, nil)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, "^GB") {
		t.Error("^GB missing for rectangle")
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
	if strings.Contains(string(data), "^PQ") {
		t.Error("^PQ emitted for copies=1")
	}

	data, err = (&Encoder{}).Encode(context.Background(), doc, renderer, profile, map[string]string{"copies": "3"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "^PQ3") {
		t.Error("^PQ3 not found for copies=3")
	}

	_, err = (&Encoder{}).Encode(context.Background(), doc, renderer, profile, map[string]string{"copies": "0"})
	if err == nil {
		t.Error("expected error for copies=0")
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

func TestInvalidDocument(t *testing.T) {
	doc := document.New("", "Test", document.Size{Width: 80000, Height: 100000})
	profile := printer.PrinterProfile{DPI: 203}
	renderer := &basic.Renderer{}
	_, err := (&Encoder{}).Encode(context.Background(), doc, renderer, profile, nil)
	if err == nil {
		t.Error("expected error for invalid document")
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
	s := string(data)
	if !strings.Contains(s, "^XA") || !strings.Contains(s, "^XZ") {
		t.Error("minimal label missing")
	}
}
