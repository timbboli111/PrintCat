package editor

import (
	"fmt"
	"github.com/timboli111/PrintCat/internal/document"
)

const ScreenDPI = 96.0

type Editor struct {
	Doc   *document.Document
	Zoom  float64
	SelID string
}

func New(doc *document.Document) *Editor {
	return &Editor{
		Doc:   doc,
		Zoom:  1.0,
		SelID: "",
	}
}

func (e *Editor) UnitToMm(u document.Unit) float64 {
	return float64(u) / 1000.0
}

func (e *Editor) MmToView(mm float64) float64 {
	return mm * (ScreenDPI / 25.4) * e.Zoom
}

func (e *Editor) ViewToMm(px float64) float64 {
	return px / ((ScreenDPI / 25.4) * e.Zoom)
}

func (e *Editor) DocToView(p document.Point) (float64, float64) {
	return e.MmToView(e.UnitToMm(p.X)), e.MmToView(e.UnitToMm(p.Y))
}

func (e *Editor) ViewToDoc(x, y float64) document.Point {
	return document.Point{
		X: document.Unit(e.ViewToMm(x) * 1000.0),
		Y: document.Unit(e.ViewToMm(y) * 1000.0),
	}
}

func (e *Editor) SetZoom(z float64) {
	if z < 0.1 {
		z = 0.1
	}
	e.Zoom = z
}

func (e *Editor) SetPaperSize(w, h document.Unit) {
	if w <= 0 || h <= 0 {
		return
	}
	e.Doc.PageSize.Width = w
	e.Doc.PageSize.Height = h
}

func (e *Editor) AddText(content string, pos document.Point, size document.Size, fontSize document.Unit) {
	id := fmt.Sprintf("text-%d", len(e.Doc.Elements)+1)
	elem := document.NewTextElement(id, content, document.Rect{Position: pos, Size: size}, fontSize)
	e.Doc.Elements = append(e.Doc.Elements, elem)
}

func (e *Editor) AddImage(data []byte, mime string, pos document.Point, size document.Size) {
	id := fmt.Sprintf("img-%d", len(e.Doc.Elements)+1)
	elem := document.NewImageElement(id, data, mime, document.Rect{Position: pos, Size: size})
	e.Doc.Elements = append(e.Doc.Elements, elem)
}

func (e *Editor) DeleteSelected() {
	if e.SelID == "" {
		return
	}
	newElems := make([]document.Element, 0, len(e.Doc.Elements)-1)
	for _, el := range e.Doc.Elements {
		if el.ID != e.SelID {
			newElems = append(newElems, el)
		}
	}
	e.Doc.Elements = newElems
	e.SelID = ""
}

func (e *Editor) MoveSelected(dx, dy document.Unit) {
	if e.SelID == "" {
		return
	}
	for i := range e.Doc.Elements {
		if e.Doc.Elements[i].ID == e.SelID {
			e.Doc.Elements[i].Bounds.Position.X += dx
			e.Doc.Elements[i].Bounds.Position.Y += dy
			break
		}
	}
}

func (e *Editor) SelectByPoint(p document.Point) {
	e.SelID = ""
	for i := len(e.Doc.Elements) - 1; i >= 0; i-- {
		el := e.Doc.Elements[i]
		if el.Visible && pointInRect(p, el.Bounds) {
			e.SelID = el.ID
			return
		}
	}
}

func pointInRect(p document.Point, r document.Rect) bool {
	return p.X >= r.Position.X && p.X <= r.Position.X+r.Size.Width &&
		p.Y >= r.Position.Y && p.Y <= r.Position.Y+r.Size.Height
}
