package editor

import (
	"bytes"
	"github.com/timboli111/PrintCat/internal/document"
	"image"
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

func pngData(t *testing.T, width, height int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewGray(image.Rect(0, 0, width, height))); err != nil {
		t.Fatalf("png.Encode() error = %v", err)
	}
	return buf.Bytes()
}
