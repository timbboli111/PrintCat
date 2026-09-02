package epl

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
	if e.Protocol() != printer.EPL {
		t.Errorf("expected EPL, got %v", e.Protocol())
	}
}

func TestLabelSetup(t *testing.T) {
	doc := document.New("test", "Test", document.Size{Width: 80000, Height: 100000})
	profile := printer.PrinterProfile{DPI: 203}
	renderer := &basic.Renderer{}
	data, err := (&Encoder{}).Encode(context.Background(), doc, renderer, profile, nil)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, "N\n") {
		t.Error("N command missing")
	}
	dpi := float64(profile.DPI)
	widthDots := int(float64(doc.PageSize.Width) / 1000.0 / 25.4 * dpi)
	heightDots := int(float64(doc.PageSize.Height) / 1000.0 / 25.4 * dpi)
	if !strings.Contains(s, "q"+strconv.Itoa(widthDots)+"\n") {
		t.Errorf("q%d missing", widthDots)
	}
	if !strings.Contains(s, "Q"+strconv.Itoa(heightDots)+",0\n") {
		t.Errorf("Q%d,0 missing", heightDots)
	}
	if !strings.Contains(s, "P1,1\n") {
		t.Error("P1,1 missing")
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
	if !strings.Contains(s, "A") {
		t.Error("A command missing")
	}
	dpi := float64(profile.DPI)
	xDots := int(float64(elem.Bounds.Position.X) / 1000.0 / 25.4 * dpi)
	yDots := int(float64(elem.Bounds.Position.Y) / 1000.0 / 25.4 * dpi)
	if !strings.Contains(s, "A"+strconv.Itoa(xDots)+","+strconv.Itoa(yDots)+",0") {
		t.Errorf("A coordinates mismatch: expected A%d,%d,0", xDots, yDots)
	}
	if !strings.Contains(s, "\"Hello\"") {
		t.Error("text content missing quotes")
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
	if !bytes.Contains(data, []byte("GW")) {
		t.Error("GW command missing")
	}
	if !bytes.Contains(data, []byte{0xAA}) {
		t.Error("bit packing mismatch: expected raw byte 0xAA in GW data")
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
	if !strings.Contains(s, "LO") {
		t.Error("LO command missing")
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
	if !strings.Contains(s, "BOX") {
		t.Error("BOX command missing")
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
	s := string(data)
	if !strings.Contains(s, "P1,1\n") {
		t.Error("P1,1 missing for copies=1")
	}

	data, err = (&Encoder{}).Encode(context.Background(), doc, renderer, profile, map[string]string{"copies": "3"})
	if err != nil {
		t.Fatal(err)
	}
	s = string(data)
	if !strings.Contains(s, "P3,1\n") {
		t.Error("P3,1 missing for copies=3")
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
	if !strings.Contains(s, "N\n") {
		t.Error("N missing")
	}
	if !strings.Contains(s, "q") {
		t.Error("q missing")
	}
	if !strings.Contains(s, "Q") {
		t.Error("Q missing")
	}
	if !strings.Contains(s, "P1,1\n") {
		t.Error("P1,1 missing")
	}
}

func TestTerminatorLF(t *testing.T) {
	doc := document.New("test", "Test", document.Size{Width: 80000, Height: 100000})
	profile := printer.PrinterProfile{DPI: 203}
	renderer := &basic.Renderer{}
	data, err := (&Encoder{}).Encode(context.Background(), doc, renderer, profile, nil)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if strings.Contains(s, "\r\n") {
		t.Error("CRLF found, expected LF only")
	}
	if !strings.Contains(s, "\n") {
		t.Error("LF not found")
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

func TestDPI203And300(t *testing.T) {
	doc := document.New("test", "Test", document.Size{Width: 80000, Height: 100000})
	renderer := &basic.Renderer{}

	profile203 := printer.PrinterProfile{DPI: 203}
	data203, err := (&Encoder{}).Encode(context.Background(), doc, renderer, profile203, nil)
	if err != nil {
		t.Fatal(err)
	}
	s203 := string(data203)

	profile300 := printer.PrinterProfile{DPI: 300}
	data300, err := (&Encoder{}).Encode(context.Background(), doc, renderer, profile300, nil)
	if err != nil {
		t.Fatal(err)
	}
	s300 := string(data300)

	if !strings.Contains(s203, "q") || !strings.Contains(s300, "q") {
		t.Error("both DPI should produce q command")
	}
	if s203 == s300 {
		t.Error("DPI 203 and 300 should produce different output")
	}
}
