package basic

import (
	"bytes"
	"context"
	"image"
	"image/color"
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

func TestImageRenderingPreservesAspectRatio(t *testing.T) {
	r := &Renderer{}
	doc := document.New("test", "test", document.Size{Width: 40_000, Height: 40_000})

	img := image.NewGray(image.Rect(0, 0, 20, 10))
	for i := range img.Pix {
		img.Pix[i] = 0
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	doc.Elements = append(doc.Elements, document.NewImageElement("img1", buf.Bytes(), "image/png", document.Rect{
		Size: document.Size{Width: 20_000, Height: 20_000},
	}))

	ras, err := r.Render(context.Background(), doc, render.Target{DPI: 254})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	// 20 mm becomes 200 pixels at 254 DPI. A 2:1 source occupies 200x100
	// pixels, centred vertically in its square allocation.
	if ras.Pixels[10*ras.Width+10] != 255 {
		t.Error("top letterbox area is not white")
	}
	if ras.Pixels[100*ras.Width+10] != 0 {
		t.Error("contained image area is not black")
	}
}

func TestTransparentImageCanCompositeOverWhite(t *testing.T) {
	r := &Renderer{}
	doc := document.New("test", "test", document.Size{Width: 20_000, Height: 20_000})
	src := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	src.SetNRGBA(7, 7, color.NRGBA{R: 0, G: 0, B: 0, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		t.Fatal(err)
	}
	doc.Elements = append(doc.Elements, document.NewImageElement("alpha", buf.Bytes(), "image/png", document.Rect{
		Position: document.Point{X: 2_000, Y: 2_000},
		Size:     document.Size{Width: 8_000, Height: 8_000},
	}))

	legacyRaster, err := r.Render(context.Background(), doc, render.Target{DPI: 254})
	if err != nil {
		t.Fatal(err)
	}
	compositedRaster, err := r.Render(context.Background(), doc, render.Target{DPI: 254, CompositeAlphaOnWhite: true})
	if err != nil {
		t.Fatal(err)
	}
	transparentSample := (20+10)*legacyRaster.Width + (20 + 10)
	if legacyRaster.Pixels[transparentSample] == 255 {
		t.Fatal("setup error: legacy transparent image sample unexpectedly white")
	}
	if compositedRaster.Pixels[transparentSample] != 255 {
		t.Errorf("transparent source pixel = %d, want white (255)", compositedRaster.Pixels[transparentSample])
	}
	blackSample := (20+75)*compositedRaster.Width + (20 + 75)
	if compositedRaster.Pixels[blackSample] >= 160 {
		t.Errorf("opaque black source pixel = %d, want darker than print threshold", compositedRaster.Pixels[blackSample])
	}
}

// TestImageRenderingAppliesRotation verifies that the print rasterizer
// honours ImageData.Rotation. The source is a landscape 4:1 image whose
// left half is black and right half is white. With rotation 0 the black
// half fills the left of the contained area; with rotation 90 the black
// half must appear at the top.
func TestImageRenderingAppliesRotation(t *testing.T) {
	r := &Renderer{}
	doc := document.New("test", "test", document.Size{Width: 40_000, Height: 40_000})

	src := image.NewGray(image.Rect(0, 0, 4, 1))
	src.Pix[0] = 0
	src.Pix[1] = 0
	src.Pix[2] = 255
	src.Pix[3] = 255
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		t.Fatal(err)
	}

	doc.Elements = []document.Element{document.NewImageElement(
		"img0", buf.Bytes(), "image/png",
		document.Rect{Size: document.Size{Width: 20_000, Height: 20_000}},
	)}
	ras0, err := r.Render(context.Background(), doc, render.Target{DPI: 254})
	if err != nil {
		t.Fatalf("Render rotation=0 error = %v", err)
	}
	// Contained rect: 200x50 centred vertically. Left half black.
	if ras0.Pixels[100*ras0.Width+40] != 0 {
		t.Errorf("rotation=0 left half expected black, got %d",
			ras0.Pixels[100*ras0.Width+40])
	}
	if ras0.Pixels[100*ras0.Width+160] != 255 {
		t.Errorf("rotation=0 right half expected white, got %d",
			ras0.Pixels[100*ras0.Width+160])
	}

	rotated := document.ImageData{
		Data:     buf.Bytes(),
		MimeType: "image/png",
		Rotation: 90,
	}
	doc.Elements = []document.Element{{
		ID:      "img90",
		Type:    document.ImageElement,
		Bounds:  document.Rect{Size: document.Size{Width: 20_000, Height: 20_000}},
		Visible: true,
		Data:    rotated,
	}}
	ras90, err := r.Render(context.Background(), doc, render.Target{DPI: 254})
	if err != nil {
		t.Fatalf("Render rotation=90 error = %v", err)
	}
	// Contained rect now 50x200 centred horizontally. The original black
	// half becomes the top half of the raster.
	if ras90.Pixels[40*ras90.Width+100] != 0 {
		t.Errorf("rotation=90 top expected black, got %d",
			ras90.Pixels[40*ras90.Width+100])
	}
	if ras90.Pixels[160*ras90.Width+100] != 255 {
		t.Errorf("rotation=90 bottom expected white, got %d",
			ras90.Pixels[160*ras90.Width+100])
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
