package fyneapp

import (
	"bytes"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/timboli111/PrintCat/internal/document"
	"github.com/timboli111/PrintCat/internal/editor"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
)

type CanvasWidget struct {
	widget.BaseWidget
	ed          *editor.Editor
	objects     map[string]fyne.CanvasObject
	cont        *fyne.Container
	onSelect    func(id string)
	lastDragPos fyne.Position
	isDragging  bool
}

func NewCanvas(ed *editor.Editor, onSelect func(id string)) *CanvasWidget {
	c := &CanvasWidget{
		ed:       ed,
		objects:  make(map[string]fyne.CanvasObject),
		onSelect: onSelect,
	}
	c.ExtendBaseWidget(c)
	return c
}

func (c *CanvasWidget) CreateRenderer() fyne.WidgetRenderer {
	c.cont = container.NewWithoutLayout()
	c.updateObjects()
	return widget.NewSimpleRenderer(c.cont)
}

func (c *CanvasWidget) MinSize() fyne.Size {
	w, h := c.ed.DocToView(document.Point{X: c.ed.Doc.PageSize.Width, Y: c.ed.Doc.PageSize.Height})
	return fyne.NewSize(float32(w), float32(h))
}

func (c *CanvasWidget) Refresh() {
	c.updateObjects()
	c.BaseWidget.Refresh()
}

func (c *CanvasWidget) updateObjects() {
	if c.cont == nil {
		return
	}
	current := make(map[string]bool)
	for _, el := range c.ed.Doc.Elements {
		current[el.ID] = true
	}
	for id, obj := range c.objects {
		if !current[id] {
			c.cont.Remove(obj)
			delete(c.objects, id)
		}
	}
	for _, el := range c.ed.Doc.Elements {
		obj, exists := c.objects[el.ID]
		if !exists {
			obj = c.makeObject(el)
			c.objects[el.ID] = obj
			c.cont.Add(obj)
		}
		c.positionObject(obj, el)
		if textObj, ok := obj.(*canvas.Text); ok {
			if td, ok := el.Data.(document.TextData); ok {
				textObj.Text = td.Content
				textObj.TextSize = float32(c.ed.MmToView(c.ed.UnitToMm(td.FontSize)))
				textObj.Alignment = c.alignToFyne(td.Alignment)
				textObj.Refresh()
			}
		}
	}
}

func (c *CanvasWidget) makeObject(el document.Element) fyne.CanvasObject {
	switch el.Type {
	case document.TextElement:
		td := el.Data.(document.TextData)
		txt := canvas.NewText(td.Content, color.Black)
		txt.TextSize = float32(c.ed.MmToView(c.ed.UnitToMm(td.FontSize)))
		txt.Alignment = c.alignToFyne(td.Alignment)
		return txt
	case document.ImageElement:
		idata := el.Data.(document.ImageData)
		img, _, err := image.Decode(bytes.NewReader(idata.Data))
		if err != nil {
			return canvas.NewRectangle(color.Gray{Y: 200})
		}
		return canvas.NewImageFromImage(img)
	default:
		return canvas.NewRectangle(color.Gray{Y: 200})
	}
}

func (c *CanvasWidget) positionObject(obj fyne.CanvasObject, el document.Element) {
	x, y := c.ed.DocToView(el.Bounds.Position)
	w, h := c.ed.DocToView(document.Point{X: el.Bounds.Size.Width, Y: el.Bounds.Size.Height})
	obj.Move(fyne.NewPos(float32(x), float32(y)))
	obj.Resize(fyne.NewSize(float32(w), float32(h)))
}

func (c *CanvasWidget) alignToFyne(a string) fyne.TextAlign {
	switch a {
	case "center":
		return fyne.TextAlignCenter
	case "right":
		return fyne.TextAlignTrailing
	default:
		return fyne.TextAlignLeading
	}
}

func (c *CanvasWidget) Tapped(ev *fyne.PointEvent) {
	pos := c.ed.ViewToDoc(float64(ev.Position.X), float64(ev.Position.Y))
	c.ed.SelectByPoint(pos)
	if c.onSelect != nil {
		c.onSelect(c.ed.SelID)
	}
	c.Refresh()
}

func (c *CanvasWidget) Dragged(ev *fyne.DragEvent) {
	if c.ed.SelID == "" {
		return
	}
	if !c.isDragging {
		c.isDragging = true
		c.lastDragPos = ev.Position
		return
	}
	dx := ev.Position.X - c.lastDragPos.X
	dy := ev.Position.Y - c.lastDragPos.Y
	if dx == 0 && dy == 0 {
		return
	}
	docDx := document.Unit(float64(dx) * 1000.0 / (editor.ScreenDPI / 25.4 * c.ed.Zoom))
	docDy := document.Unit(float64(dy) * 1000.0 / (editor.ScreenDPI / 25.4 * c.ed.Zoom))
	c.ed.MoveSelected(docDx, docDy)
	c.lastDragPos = ev.Position
	c.Refresh()
}

func (c *CanvasWidget) DragEnd() {
	c.isDragging = false
}
