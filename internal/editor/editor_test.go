package editor

import (
	"bytes"
	"github.com/timboli111/PrintCat/internal/document"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

func TestDocToViewAndBack(t *testing.T) {
	doc := document.New("test", "test", document.Size{Width: 80000, Height: 100000})
	ed := New(&doc)

	p := document.Point{X: 10000, Y: 20000}
	x, y := ed.DocToView(p)
	back := ed.ViewToDoc(x, y)
	if back.X != p.X || back.Y != p.Y {
		t.Errorf("roundtrip failed: got (%d,%d), want (%d,%d)", back.X, back.Y, p.X, p.Y)
	}
}

func TestAddText(t *testing.T) {
	doc := document.New("test", "test", document.Size{Width: 80000, Height: 100000})
	ed := New(&doc)
	ed.AddText("hello", document.Point{X: 0, Y: 0}, document.Size{Width: 10000, Height: 5000}, 4000)
	if len(ed.Doc.Elements) != 1 {
		t.Fatalf("expected 1 element, got %d", len(ed.Doc.Elements))
	}
	el := ed.Doc.Elements[0]
	if el.Type != document.TextElement {
		t.Errorf("expected TextElement, got %v", el.Type)
	}
	if td, ok := el.Data.(document.TextData); !ok || td.Content != "hello" {
		t.Errorf("text data not correct")
	}
}

func TestSelectAndDelete(t *testing.T) {
	doc := document.New("test", "test", document.Size{Width: 80000, Height: 100000})
	ed := New(&doc)
	ed.AddText("a", document.Point{X: 0, Y: 0}, document.Size{Width: 10000, Height: 5000}, 4000)
	ed.AddText("b", document.Point{X: 5000, Y: 0}, document.Size{Width: 10000, Height: 5000}, 4000)

	ed.SelectByPoint(document.Point{X: 2500, Y: 2500})
	if ed.SelID == "" {
		t.Error("expected selection")
	}
	ed.DeleteSelected()
	if len(ed.Doc.Elements) != 1 {
		t.Errorf("expected 1 element after delete, got %d", len(ed.Doc.Elements))
	}
}

func TestMoveSelected(t *testing.T) {
	doc := document.New("test", "test", document.Size{Width: 80000, Height: 100000})
	ed := New(&doc)
	ed.AddText("a", document.Point{X: 1000, Y: 2000}, document.Size{Width: 10000, Height: 5000}, 4000)
	ed.SelectByPoint(document.Point{X: 1000, Y: 2000})
	ed.MoveSelected(500, -300)
	if ed.Doc.Elements[0].Bounds.Position.X != 1500 || ed.Doc.Elements[0].Bounds.Position.Y != 1700 {
		t.Errorf("position after move incorrect")
	}
}

func TestSetPaperSizeInvalid(t *testing.T) {
	doc := document.New("test", "test", document.Size{Width: 80000, Height: 100000})
	ed := New(&doc)
	ed.SetPaperSize(0, 50000)
	if ed.Doc.PageSize.Width != 80000 || ed.Doc.PageSize.Height != 100000 {
		t.Errorf("paper size changed when invalid width")
	}
	ed.SetPaperSize(-1000, 50000)
	if ed.Doc.PageSize.Width != 80000 {
		t.Errorf("paper size changed when negative width")
	}
	ed.SetPaperSize(60000, 0)
	if ed.Doc.PageSize.Height != 100000 {
		t.Errorf("paper size changed when invalid height")
	}
	ed.SetPaperSize(60000, 120000)
	if ed.Doc.PageSize.Width != 60000 || ed.Doc.PageSize.Height != 120000 {
		t.Errorf("valid paper size not set")
	}
}

func TestAddImageFitToPagePreservesAspectRatioAndCentresImage(t *testing.T) {
	doc := document.New("test", "test", document.Size{Width: 80_000, Height: 200_000})
	ed := New(&doc)

	data := pngData(t, 400, 200)
	if err := ed.AddImageFitToPage(data, "image/png"); err != nil {
		t.Fatalf("AddImageFitToPage() error = %v", err)
	}

	if len(doc.Elements) != 1 {
		t.Fatalf("elements = %d, want 1", len(doc.Elements))
	}
	bounds := doc.Elements[0].Bounds
	if bounds.Size.Width != 70_000 || bounds.Size.Height != 35_000 {
		t.Errorf("image size = %dx%d, want 70000x35000", bounds.Size.Width, bounds.Size.Height)
	}
	if bounds.Position.X != 5_000 || bounds.Position.Y != 82_500 {
		t.Errorf("image position = (%d,%d), want (5000,82500)", bounds.Position.X, bounds.Position.Y)
	}
}

func TestAddImageFitToPageUsesHeightForTallImages(t *testing.T) {
	doc := document.New("test", "test", document.Size{Width: 80_000, Height: 100_000})
	ed := New(&doc)

	if err := ed.AddImageFitToPage(pngData(t, 100, 200), "image/png"); err != nil {
		t.Fatalf("AddImageFitToPage() error = %v", err)
	}
	bounds := doc.Elements[0].Bounds
	if bounds.Size.Width != 45_000 || bounds.Size.Height != 90_000 {
		t.Errorf("image size = %dx%d, want 45000x90000", bounds.Size.Width, bounds.Size.Height)
	}
	if bounds.Position.X != 17_500 || bounds.Position.Y != 5_000 {
		t.Errorf("image position = (%d,%d), want (17500,5000)", bounds.Position.X, bounds.Position.Y)
	}
}

func TestAddImageFitToPageRejectsInvalidData(t *testing.T) {
	doc := document.New("test", "test", document.Size{Width: 80_000, Height: 100_000})
	ed := New(&doc)
	if err := ed.AddImageFitToPage([]byte("not an image"), "image/png"); err == nil {
		t.Fatal("AddImageFitToPage() unexpectedly accepted invalid data")
	}
}

func TestDetectImageMimePng(t *testing.T) {
	data := pngData(t, 10, 10)
	mime, err := DetectImageMime(data)
	if err != nil {
		t.Fatalf("DetectImageMime(png) error = %v", err)
	}
	if mime != "image/png" {
		t.Errorf("mime = %q, want image/png", mime)
	}
}

func TestDetectImageMimeJpeg(t *testing.T) {
	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("jpeg.Encode: %v", err)
	}
	mime, err := DetectImageMime(buf.Bytes())
	if err != nil {
		t.Fatalf("DetectImageMime(jpeg) error = %v", err)
	}
	if mime != "image/jpeg" {
		t.Errorf("mime = %q, want image/jpeg", mime)
	}
}

func TestDetectImageMimeRejectsEmpty(t *testing.T) {
	if _, err := DetectImageMime(nil); err == nil {
		t.Error("DetectImageMime(nil) unexpectedly succeeded")
	}
	if _, err := DetectImageMime([]byte{}); err == nil {
		t.Error("DetectImageMime(empty) unexpectedly succeeded")
	}
}

func TestDetectImageMimeRejectsCorrupt(t *testing.T) {
	if _, err := DetectImageMime([]byte("not an image")); err == nil {
		t.Error("DetectImageMime(corrupt) unexpectedly succeeded")
	}
}

func TestViewToDocAndBackAtMultipleZooms(t *testing.T) {
	doc := document.New("test", "test", document.Size{Width: 80000, Height: 100000})
	ed := New(&doc)
	for _, zoom := range []float64{0.5, 1.0, 1.5, 3.0} {
		ed.SetZoom(zoom)
		p := document.Point{X: 10000, Y: 20000}
		x, y := ed.DocToView(p)
		back := ed.ViewToDoc(x, y)
		if back.X != p.X || back.Y != p.Y {
			t.Errorf("zoom=%.1f roundtrip failed: got (%d,%d), want (%d,%d)",
				zoom, back.X, back.Y, p.X, p.Y)
		}
	}
}

func TestMoveSelectedAtMultipleZooms(t *testing.T) {
	doc := document.New("test", "test", document.Size{Width: 80000, Height: 100000})
	ed := New(&doc)
	ed.AddText("a", document.Point{X: 1000, Y: 2000},
		document.Size{Width: 10000, Height: 5000}, 4000)
	ed.SelectByPoint(document.Point{X: 1000, Y: 2000})
	ed.MoveSelected(500, -300)
	got := ed.Doc.Elements[0].Bounds.Position
	if got.X != 1500 || got.Y != 1700 {
		t.Errorf("position after move = (%d,%d), want (1500,1700)", got.X, got.Y)
	}
	ed.SetZoom(2.0)
	ed.MoveSelected(100, 200)
	got = ed.Doc.Elements[0].Bounds.Position
	if got.X != 1600 || got.Y != 1900 {
		t.Errorf("position after zoomed move = (%d,%d), want (1600,1900)",
			got.X, got.Y)
	}
}

func TestSelectedElementReturnsPointer(t *testing.T) {
	doc := document.New("test", "test", document.Size{Width: 80000, Height: 100000})
	ed := New(&doc)
	ed.AddText("a", document.Point{X: 1000, Y: 2000},
		document.Size{Width: 10000, Height: 5000}, 4000)
	if ed.SelectedElement() != nil {
		t.Error("expected nil SelectedElement when SelID is empty")
	}
	ed.SelID = "text-1"
	el := ed.SelectedElement()
	if el == nil {
		t.Fatal("expected non-nil SelectedElement")
	}
	if el.ID != "text-1" {
		t.Errorf("SelectedElement returned %q, want text-1", el.ID)
	}
	ed.SelID = "nonexistent"
	if ed.SelectedElement() != nil {
		t.Error("expected nil for stale SelID")
	}
}

func TestRotateSelectedImageNoSelectionIsNoop(t *testing.T) {
	doc := document.New("test", "test", document.Size{Width: 80000, Height: 100000})
	ed := New(&doc)
	if err := ed.RotateSelectedImage(); err != nil {
		t.Errorf("RotateSelectedImage with no selection returned error: %v", err)
	}
}

func TestRotateSelectedImageOnTextReturnsError(t *testing.T) {
	doc := document.New("test", "test", document.Size{Width: 80000, Height: 100000})
	ed := New(&doc)
	ed.AddText("a", document.Point{X: 1000, Y: 2000},
		document.Size{Width: 10000, Height: 5000}, 4000)
	ed.SelID = "text-1"
	if err := ed.RotateSelectedImage(); err == nil {
		t.Error("RotateSelectedImage on text unexpectedly succeeded")
	}
}

func TestRotateSelectedImageAdvancesRotationMetadataOnly(t *testing.T) {
	doc := document.New("test", "test", document.Size{Width: 200_000, Height: 200_000})
	ed := New(&doc)
	ed.AddImage(pngDataColored(t, 20, 10), "image/png",
		document.Point{X: 50_000, Y: 60_000},
		document.Size{Width: 80_000, Height: 40_000})
	ed.SelID = "img-1"

	idataBefore := ed.Doc.Elements[0].Data.(document.ImageData)
	beforeByteSlice := idataBefore.Data
	beforeMime := idataBefore.MimeType

	if err := ed.RotateSelectedImage(); err != nil {
		t.Fatalf("RotateSelectedImage error = %v", err)
	}

	idataAfter := ed.Doc.Elements[0].Data.(document.ImageData)
	if idataAfter.Rotation != 90 {
		t.Errorf("Rotation = %d, want 90", idataAfter.Rotation)
	}
	if idataAfter.MimeType != beforeMime {
		t.Errorf("MimeType changed from %q to %q", beforeMime, idataAfter.MimeType)
	}
	if len(idataAfter.Data) != len(beforeByteSlice) {
		t.Errorf("Data length changed: %d -> %d",
			len(beforeByteSlice), len(idataAfter.Data))
	}
}

func TestRotateSelectedImageSwapsBoundsAndKeepsCenter(t *testing.T) {
	doc := document.New("test", "test", document.Size{Width: 200_000, Height: 200_000})
	ed := New(&doc)
	ed.AddImage(pngDataColored(t, 20, 10), "image/png",
		document.Point{X: 50_000, Y: 60_000},
		document.Size{Width: 80_000, Height: 40_000})
	ed.SelID = "img-1"

	beforeCX := ed.Doc.Elements[0].Bounds.Position.X + ed.Doc.Elements[0].Bounds.Size.Width/2
	beforeCY := ed.Doc.Elements[0].Bounds.Position.Y + ed.Doc.Elements[0].Bounds.Size.Height/2

	if err := ed.RotateSelectedImage(); err != nil {
		t.Fatalf("RotateSelectedImage error = %v", err)
	}

	el := ed.Doc.Elements[0]
	if el.Bounds.Size.Width != 40_000 || el.Bounds.Size.Height != 80_000 {
		t.Errorf("bounds after rotate = %dx%d, want 40000x80000",
			el.Bounds.Size.Width, el.Bounds.Size.Height)
	}
	afterCX := el.Bounds.Position.X + el.Bounds.Size.Width/2
	afterCY := el.Bounds.Position.Y + el.Bounds.Size.Height/2
	if afterCX != beforeCX || afterCY != beforeCY {
		t.Errorf("centre moved: before=(%d,%d) after=(%d,%d)",
			beforeCX, beforeCY, afterCX, afterCY)
	}
}

// TestRotateSelectedImageFourTimesOnLargePage: page is large enough that
// rotation never triggers a scale-down, so four rotations should return the
// element to its original position and size.
func TestRotateSelectedImageFourTimesOnLargePage(t *testing.T) {
	doc := document.New("test", "test", document.Size{Width: 200_000, Height: 200_000})
	ed := New(&doc)
	ed.AddImage(pngDataColored(t, 20, 10), "image/png",
		document.Point{X: 50_000, Y: 60_000},
		document.Size{Width: 80_000, Height: 40_000})
	ed.SelID = "img-1"

	for i := 0; i < 4; i++ {
		if err := ed.RotateSelectedImage(); err != nil {
			t.Fatalf("rotate %d error = %v", i+1, err)
		}
	}
	el := ed.Doc.Elements[0]
	if el.Bounds.Size.Width != 80_000 || el.Bounds.Size.Height != 40_000 {
		t.Errorf("bounds after 4 rotations = %dx%d, want 80000x40000",
			el.Bounds.Size.Width, el.Bounds.Size.Height)
	}
	if el.Bounds.Position.X != 50_000 || el.Bounds.Position.Y != 60_000 {
		t.Errorf("position after 4 rotations = (%d,%d), want (50000,60000)",
			el.Bounds.Position.X, el.Bounds.Position.Y)
	}
	idata := el.Data.(document.ImageData)
	if idata.Rotation != 0 {
		t.Errorf("Rotation after 4 rotations = %d, want 0", idata.Rotation)
	}
}

// TestRotateSelectedImageFourTimesOnConstrainedPage exercises the case that
// motivated the BaseSize metadata: a rotation to 90° or 270° forces a
// scale-down to fit the page. Without BaseSize, four rotations would leave
// the element permanently shrunk. With BaseSize, four rotations return the
// element to its original size and position.
func TestRotateSelectedImageFourTimesOnConstrainedPage(t *testing.T) {
	// Page 200x90 mm. Margin 5 mm. maxW=190_000, maxH=80_000.
	// Element: 160_000 x 40_000 at (20_000, 25_000), centre (100_000, 45_000).
	// Rotate to 90°: desired 40_000 x 160_000, scaled to 20_000 x 80_000.
	// Centre preserved: position (90_000, 5_000).
	// Rotate to 180°: back to 160_000 x 40_000 at (20_000, 25_000).
	doc := document.New("test", "test", document.Size{Width: 200_000, Height: 90_000})
	ed := New(&doc)
	ed.AddImage(pngDataColored(t, 40, 10), "image/png",
		document.Point{X: 20_000, Y: 25_000},
		document.Size{Width: 160_000, Height: 40_000})
	ed.SelID = "img-1"

	// Rotation 1: expect the constrained size.
	if err := ed.RotateSelectedImage(); err != nil {
		t.Fatalf("rotate 1 error = %v", err)
	}
	el := ed.Doc.Elements[0]
	if el.Bounds.Size.Width != 20_000 || el.Bounds.Size.Height != 80_000 {
		t.Errorf("after rotate 1: bounds = %dx%d, want 20000x80000",
			el.Bounds.Size.Width, el.Bounds.Size.Height)
	}

	// Rotation 2: element must be restored to its original size, not stay
	// shrunk. Position too must return to the original coordinates because
	// the centre is preserved through the cycle.
	if err := ed.RotateSelectedImage(); err != nil {
		t.Fatalf("rotate 2 error = %v", err)
	}
	el = ed.Doc.Elements[0]
	if el.Bounds.Size.Width != 160_000 || el.Bounds.Size.Height != 40_000 {
		t.Errorf("after rotate 2: bounds = %dx%d, want 160000x40000",
			el.Bounds.Size.Width, el.Bounds.Size.Height)
	}
	if el.Bounds.Position.X != 20_000 || el.Bounds.Position.Y != 25_000 {
		t.Errorf("after rotate 2: position = (%d,%d), want (20000,25000)",
			el.Bounds.Position.X, el.Bounds.Position.Y)
	}

	// Rotations 3 and 4: back to rotation 0 with the original size.
	if err := ed.RotateSelectedImage(); err != nil {
		t.Fatalf("rotate 3 error = %v", err)
	}
	if err := ed.RotateSelectedImage(); err != nil {
		t.Fatalf("rotate 4 error = %v", err)
	}
	el = ed.Doc.Elements[0]
	if el.Bounds.Size.Width != 160_000 || el.Bounds.Size.Height != 40_000 {
		t.Errorf("after 4 rotations: bounds = %dx%d, want 160000x40000",
			el.Bounds.Size.Width, el.Bounds.Size.Height)
	}
	if el.Bounds.Position.X != 20_000 || el.Bounds.Position.Y != 25_000 {
		t.Errorf("after 4 rotations: position = (%d,%d), want (20000,25000)",
			el.Bounds.Position.X, el.Bounds.Position.Y)
	}
	idata := el.Data.(document.ImageData)
	if idata.Rotation != 0 {
		t.Errorf("Rotation after 4 rotations = %d, want 0", idata.Rotation)
	}
	if idata.BaseSize.Width != 160_000 || idata.BaseSize.Height != 40_000 {
		t.Errorf("BaseSize after 4 rotations = %dx%d, want 160000x40000",
			idata.BaseSize.Width, idata.BaseSize.Height)
	}
}

// TestRotateSelectedImageShrinksToFitPage: one rotation on a page where the
// rotated dimensions exceed the available area. Verifies scale-down,
// aspect-ratio preservation, and that the element remains on the page.
func TestRotateSelectedImageShrinksToFitPage(t *testing.T) {
	doc := document.New("test", "test", document.Size{Width: 200_000, Height: 90_000})
	ed := New(&doc)
	ed.AddImage(pngDataColored(t, 40, 10), "image/png",
		document.Point{X: 5_000, Y: 5_000},
		document.Size{Width: 160_000, Height: 40_000})
	ed.SelID = "img-1"

	if err := ed.RotateSelectedImage(); err != nil {
		t.Fatalf("RotateSelectedImage error = %v", err)
	}
	el := ed.Doc.Elements[0]
	if el.Bounds.Size.Width != 20_000 || el.Bounds.Size.Height != 80_000 {
		t.Errorf("bounds after rotate = %dx%d, want 20000x80000",
			el.Bounds.Size.Width, el.Bounds.Size.Height)
	}
	if el.Bounds.Position.X < 0 || el.Bounds.Position.Y < 0 {
		t.Errorf("position = (%d,%d), must not be negative",
			el.Bounds.Position.X, el.Bounds.Position.Y)
	}
	if el.Bounds.Position.X+el.Bounds.Size.Width > doc.PageSize.Width {
		t.Errorf("element overflows page horizontally: X+W=%d > %d",
			el.Bounds.Position.X+el.Bounds.Size.Width, doc.PageSize.Width)
	}
	if el.Bounds.Position.Y+el.Bounds.Size.Height > doc.PageSize.Height {
		t.Errorf("element overflows page vertically: Y+H=%d > %d",
			el.Bounds.Position.Y+el.Bounds.Size.Height, doc.PageSize.Height)
	}
	if el.Bounds.Size.Width*160_000 != el.Bounds.Size.Height*40_000 {
		t.Errorf("aspect ratio not preserved: %d/%d vs %d/%d",
			el.Bounds.Size.Width, el.Bounds.Size.Height, 40_000, 160_000)
	}
}

// TestRotateSelectedImageLegacyBaseSize: a legacy ImageData (BaseSize zero)
// must still work. The first rotation must lazily populate BaseSize from the
// current bounds, and the resulting behaviour must match an element that
// always had BaseSize set.
func TestRotateSelectedImageLegacyBaseSize(t *testing.T) {
	doc := document.New("test", "test", document.Size{Width: 200_000, Height: 200_000})

	// Manually build a legacy-style element: no BaseSize.
	legacy := document.Element{
		ID:      "legacy",
		Type:    document.ImageElement,
		Bounds:  document.Rect{Position: document.Point{X: 50_000, Y: 60_000}, Size: document.Size{Width: 80_000, Height: 40_000}},
		Visible: true,
		Data: document.ImageData{
			Data:     pngDataColored(t, 20, 10),
			MimeType: "image/png",
		},
	}
	doc.Elements = append(doc.Elements, legacy)
	ed := New(&doc)
	ed.SelID = "legacy"

	if err := ed.RotateSelectedImage(); err != nil {
		t.Fatalf("RotateSelectedImage error = %v", err)
	}
	el := ed.Doc.Elements[0]
	idata := el.Data.(document.ImageData)
	if idata.Rotation != 90 {
		t.Errorf("Rotation = %d, want 90", idata.Rotation)
	}
	if idata.BaseSize.Width != 80_000 || idata.BaseSize.Height != 40_000 {
		t.Errorf("BaseSize after first rotate = %dx%d, want 80000x40000",
			idata.BaseSize.Width, idata.BaseSize.Height)
	}
	if el.Bounds.Size.Width != 40_000 || el.Bounds.Size.Height != 80_000 {
		t.Errorf("bounds after rotate = %dx%d, want 40000x80000",
			el.Bounds.Size.Width, el.Bounds.Size.Height)
	}
}

// TestRotateSelectedImageSnapsCorruptRotation: a corrupted Rotation value
// (e.g. from a hand-edited document) must be snapped to the nearest
// quarter-turn before being advanced, so bounds and pixel transform stay in
// sync.
func TestRotateSelectedImageSnapsCorruptRotation(t *testing.T) {
	doc := document.New("test", "test", document.Size{Width: 200_000, Height: 200_000})
	doc.Elements = append(doc.Elements, document.Element{
		ID:      "img-1",
		Type:    document.ImageElement,
		Bounds:  document.Rect{Position: document.Point{X: 50_000, Y: 60_000}, Size: document.Size{Width: 80_000, Height: 40_000}},
		Visible: true,
		Data: document.ImageData{
			Data:     pngDataColored(t, 20, 10),
			MimeType: "image/png",
			Rotation: 44, // close to 0, snaps to 0 then advances to 90
		},
	})
	ed := New(&doc)
	ed.SelID = "img-1"

	if err := ed.RotateSelectedImage(); err != nil {
		t.Fatalf("RotateSelectedImage error = %v", err)
	}
	idata := ed.Doc.Elements[0].Data.(document.ImageData)
	if idata.Rotation != 90 {
		t.Errorf("Rotation = %d, want 90 (44 snaps to 0, +90)", idata.Rotation)
	}
}

func pngData(t *testing.T, width, height int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewGray(image.Rect(0, 0, width, height))); err != nil {
		t.Fatalf("png.Encode() error = %v", err)
	}
	return buf.Bytes()
}

func pngDataColored(t *testing.T, width, height int) []byte {
	t.Helper()
	var buf bytes.Buffer
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.NRGBA{R: uint8(x * 10), G: uint8(y * 10), A: 255})
		}
	}
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png.Encode() error = %v", err)
	}
	return buf.Bytes()
}
