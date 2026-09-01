package basic

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"testing"

	"github.com/timboli111/PrintCat/internal/document"
	"github.com/timboli111/PrintCat/internal/render"
)

func TestEmptyDocument(t *testing.T) {
	r := &Renderer{}
	doc := document.New("test", "test", document.Size{Width: 80000, Height: 100000})
	ras, err := r.Render(context.Background(), doc, render.Target{DPI: 96})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedW, expectedH := expectedPixels(80000, 100000, 96)
	if ras.Width != expectedW || ras.Height != expectedH {
		t.Errorf("dimensions: got %dx%d, want %dx%d", ras.Width, ras.Height, expectedW, expectedH)
	}
	if len(ras.Pixels) != ras.Width*ras.Height {
		t.Errorf("len(Pixels)=%d, want %d", len(ras.Pixels), ras.Width*ras.Height)
	}
	for _, p := range ras.Pixels {
		if p != 255 {
			t.Errorf("non-white pixel found in empty document")
		}
	}
}

func TestTextRendering(t *testing.T) {
	r := &Renderer{}
	doc := document.New("test", "test", document.Size{Width: 80000, Height: 100000})
	elem := document.NewTextElement("t1", "Hello", document.Rect{
		Position: document.Point{X: 10000, Y: 10000},
		Size:     document.Size{Width: 40000, Height: 10000},
	}, 12000)
	doc.Elements = append(doc.Elements, elem)

	ras, err := r.Render(context.Background(), doc, render.Target{DPI: 96})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	allWhite := true
	for _, p := range ras.Pixels {
		if p < 255 {
			allWhite = false
			break
		}
	}
	if allWhite {
		t.Error("text rendering produced no black pixels")
	}
}

func TestImageRendering(t *testing.T) {
	r := &Renderer{}
	doc := document.New("test", "test", document.Size{Width: 80000, Height: 100000})

	img := image.NewGray(image.Rect(0, 0, 10, 10))
	for i := range img.Pix {
		img.Pix[i] = 0
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
		ID:      "img1",
		Type:    document.ImageElement,
		Bounds:  document.Rect{Position: document.Point{X: 10000, Y: 10000}, Size: document.Size{Width: 20000, Height: 20000}},
		ZIndex:  0,
		Visible: true,
		Data:    imgData,
	}
	doc.Elements = append(doc.Elements, elem)

	ras, err := r.Render(context.Background(), doc, render.Target{DPI: 96})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	pxMinX, pxMinY := elementPixels(10000, 10000, 96)
	pxMaxX, pxMaxY := elementPixels(30000, 30000, 96)
	if pxMinX >= ras.Width || pxMinY >= ras.Height || pxMaxX <= 0 || pxMaxY <= 0 {
		t.Fatal("image bounds outside raster")
	}
	foundBlack := false
	for y := pxMinY; y < pxMaxY && y < ras.Height; y++ {
		for x := pxMinX; x < pxMaxX && x < ras.Width; x++ {
			if ras.Pixels[y*ras.Width+x] < 128 {
				foundBlack = true
				break
			}
		}
		if foundBlack {
			break
		}
	}
	if !foundBlack {
		t.Error("image did not produce black pixels")
	}
}

func TestInvalidImageData(t *testing.T) {
	r := &Renderer{}
	doc := document.New("test", "test", document.Size{Width: 80000, Height: 100000})
	elem := document.Element{
		ID:      "badimg",
		Type:    document.ImageElement,
		Bounds:  document.Rect{Position: document.Point{X: 10000, Y: 10000}, Size: document.Size{Width: 10000, Height: 10000}},
		Visible: true,
		Data:    document.ImageData{Data: []byte("invalid"), MimeType: "image/png"},
	}
	doc.Elements = append(doc.Elements, elem)

	_, err := r.Render(context.Background(), doc, render.Target{DPI: 96})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInvalidTextData(t *testing.T) {
	r := &Renderer{}
	doc := document.New("test", "test", document.Size{Width: 80000, Height: 100000})
	elem := document.Element{
		ID:      "badtext",
		Type:    document.TextElement,
		Bounds:  document.Rect{Position: document.Point{X: 10000, Y: 10000}, Size: document.Size{Width: 10000, Height: 10000}},
		Visible: true,
		Data:    "not TextData",
	}
	doc.Elements = append(doc.Elements, elem)

	_, err := r.Render(context.Background(), doc, render.Target{DPI: 96})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDifferentDPI(t *testing.T) {
	r := &Renderer{}
	doc := document.New("test", "test", document.Size{Width: 80000, Height: 100000})
	ras96, err := r.Render(context.Background(), doc, render.Target{DPI: 96})
	if err != nil {
		t.Fatal(err)
	}
	ras203, err := r.Render(context.Background(), doc, render.Target{DPI: 203})
	if err != nil {
		t.Fatal(err)
	}
	if ras96.Width >= ras203.Width || ras96.Height >= ras203.Height {
		t.Errorf("higher DPI should produce larger raster: 96=%dx%d, 203=%dx%d", ras96.Width, ras96.Height, ras203.Width, ras203.Height)
	}
}

func TestInvalidDPI(t *testing.T) {
	r := &Renderer{}
	doc := document.New("test", "test", document.Size{Width: 80000, Height: 100000})
	_, err := r.Render(context.Background(), doc, render.Target{DPI: 0})
	if err == nil {
		t.Error("expected error for DPI=0")
	}
	_, err = r.Render(context.Background(), doc, render.Target{DPI: -10})
	if err == nil {
		t.Error("expected error for negative DPI")
	}
}

func TestInvalidDocument(t *testing.T) {
	r := &Renderer{}
	doc := document.New("", "test", document.Size{Width: 80000, Height: 100000})
	_, err := r.Render(context.Background(), doc, render.Target{DPI: 96})
	if err == nil {
		t.Error("expected error for invalid document")
	}
}

// helper functions to match renderer calculations
func expectedPixels(width, height document.Unit, dpi int) (int, int) {
	w := float64(width) / 1000.0 / 25.4 * float64(dpi)
	h := float64(height) / 1000.0 / 25.4 * float64(dpi)
	return int(w), int(h)
}

func elementPixels(x, y document.Unit, dpi int) (int, int) {
	px := float64(x) / 1000.0 / 25.4 * float64(dpi)
	py := float64(y) / 1000.0 / 25.4 * float64(dpi)
	return int(px), int(py)
}
