package editor

import (
	"github.com/timboli111/PrintCat/internal/document"
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
