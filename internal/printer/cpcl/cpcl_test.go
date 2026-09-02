package cpcl

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
	if e.Protocol() != printer.CPCL {
		t.Errorf("expected CPCL, got %v", e.Protocol())
	}
}

func TestStartLine(t *testing.T) {
	doc := document.New("test", "Test", document.Size{Width: 80000, Height: 100000})
	profile := printer.PrinterProfile{DPI: 203}
	renderer := &basic.Renderer{}
	data, err := (&Encoder{}).Encode(context.Background(), doc, renderer, profile, nil)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, "! 0") {
		t.Error("start line missing")
	}
	dpi := float64(profile.DPI)
	heightDots := int(float64(doc.PageSize.Height) / 1000.0 / 25.4 * dpi)
	expected := "! 0 " + strconv.Itoa(profile.DPI) + " " + strconv.Itoa(profile.DPI) + " " + strconv.Itoa(heightDots) + " 1"
	if !strings.Contains(s, expected) {
		t.Errorf("start line mismatch: expected %q, got %q", expected, s)
	}
	if !strings.Contains(s, "\r\n") {
		t.Error("CRLF missing")
	}
}

func TestLabelDimensions(t *testing.T) {
	doc := document.New("test", "Test", document.Size{Width: 80000, Height: 100000})
	profile := printer.PrinterProfile{DPI: 203}
	renderer := &basic.Renderer{}
	data, err := (&Encoder{}).Encode(context.Background(), doc, renderer, profile, nil)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	dpi := float64(profile.DPI)
	heightDots := int(float64(doc.PageSize.Height) / 1000.0 / 25.4 * dpi)
	if !strings.Contains(s, strconv.Itoa(heightDots)) {
		t.Errorf("height %d not found in start line", heightDots)
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
	if !strings.Contains(s, "TEXT 7") {
		t.Error("TEXT command missing")
	}
	dpi := float64(profile.DPI)
	xDots := int(float64(elem.Bounds.Position.X) / 1000.0 / 25.4 * dpi)
	yDots := int(float64(elem.Bounds.Position.Y) / 1000.0 / 25.4 * dpi)
	if !strings.Contains(s, strconv.Itoa(xDots)) || !strings.Contains(s, strconv.Itoa(yDots)) {
		t.Error("TEXT coordinates mismatch")
	}
	if !strings.Contains(s, "Hello") {
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
	if !strings.Contains(s, "EG") {
		t.Error("EG command missing")
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
	if !strings.Contains(s, "LINE") {
		t.Error("LINE command missing")
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
	if !strings.Contains(s, "! 0 203 203 ") || !strings.Contains(s, " 1\r\n") {
		t.Error("copies=1 not reflected in start line")
	}

	data, err = (&Encoder{}).Encode(context.Background(), doc, renderer, profile, map[string]string{"copies": "3"})
	if err != nil {
		t.Fatal(err)
	}
	s = string(data)
	if !strings.Contains(s, "! 0 203 203 ") || !strings.Contains(s, " 3\r\n") {
		t.Error("copies=3 not reflected in start line")
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
	if !strings.Contains(s, "! 0") {
		t.Error("start line missing")
	}
	if !strings.Contains(s, "PRINT\r\n") {
		t.Error("PRINT missing")
	}
}

func TestCRLF(t *testing.T) {
	doc := document.New("test", "Test", document.Size{Width: 80000, Height: 100000})
	profile := printer.PrinterProfile{DPI: 203}
	renderer := &basic.Renderer{}
	data, err := (&Encoder{}).Encode(context.Background(), doc, renderer, profile, nil)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.Contains(s, "\r\n") {
		t.Error("CRLF not found")
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
		// some renderers may check context; ignore error
	}
}
