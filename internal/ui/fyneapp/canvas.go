package fyneapp

import (
	"bytes"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"runtime"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/timboli111/PrintCat/internal/document"
	"github.com/timboli111/PrintCat/internal/editor"
	"github.com/timboli111/PrintCat/internal/imageutil"
	xdraw "golang.org/x/image/draw"
)

const androidCanvasPreviewMaxPixels = 1024 * 1024

type CanvasWidget struct {
	widget.BaseWidget
	ed           *editor.Editor
	objects      map[string]fyne.CanvasObject
	cont         *fyne.Container
	selectionBox *canvas.Rectangle
	onSelect     func(id string)
	lastDragPos  fyne.Position
	isDragging   bool
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
	c.selectionBox = canvas.NewRectangle(color.Transparent)
	c.selectionBox.StrokeColor = color.NRGBA{R: 0, G: 120, B: 215, A: 255}
	c.selectionBox.StrokeWidth = 2
	c.selectionBox.Hide()
	c.cont.Add(c.selectionBox)
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

// InvalidateObject drops any cached canvas object for the given element ID
// so that the next updateObjects() call re-creates it. This is used when an
// element's payload changes in a way the cached object cannot reflect on
// its own — for example after the rotation metadata is advanced and the
// underlying canvas.Image must be rebuilt from the rotated pixels.
func (c *CanvasWidget) InvalidateObject(id string) {
	if obj, ok := c.objects[id]; ok {
		c.cont.Remove(obj)
		delete(c.objects, id)
	}
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
	c.updateSelectionBox()
}

// updateSelectionBox keeps the selection indicator on top of the element
// stack and in sync with the selected element's current position and size.
// It is a visual-only overlay: it never mutates the document model and
// does not affect the printed output.
func (c *CanvasWidget) updateSelectionBox() {
	if c.selectionBox == nil || c.cont == nil {
		return
	}
	objs := c.cont.Objects
	n := 0
	for _, o := range objs {
		if o != c.selectionBox {
			objs[n] = o
			n++
		}
	}
	objs = objs[:n]
	objs = append(objs, c.selectionBox)
	c.cont.Objects = objs

	if c.ed.SelID == "" {
		c.selectionBox.Hide()
		return
	}
	var selected *document.Element
	for i := range c.ed.Doc.Elements {
		if c.ed.Doc.Elements[i].ID == c.ed.SelID {
			selected = &c.ed.Doc.Elements[i]
			break
		}
	}
	if selected == nil {
		c.selectionBox.Hide()
		return
	}
	x, y := c.ed.DocToView(selected.Bounds.Position)
	w, h := c.ed.DocToView(document.Point{
		X: selected.Bounds.Size.Width,
		Y: selected.Bounds.Size.Height,
	})
	c.selectionBox.Move(fyne.NewPos(float32(x)-2, float32(y)-2))
	c.selectionBox.Resize(fyne.NewSize(float32(w)+4, float32(h)+4))
	c.selectionBox.Show()
}

// repositionSelectedOnly is the drag fast path. During a drag the selected
// element's size and payload do not change; only its position moves. This
// function therefore:
//
//   - reads the new position from the document model (never from the cached
//     canvas object), so the view cannot drift from the model;
//   - Move()s only the one affected object (no Resize, no re-decode, no
//     updateObjects() over the whole element list);
//   - Move()s the selection box by the same offset;
//   - Refresh()es only those two canvas objects.
//
// Whether this actually removes the perceived lag on a given Android device
// can only be determined by a physical test. Fyne v2.x on Android uses a
// GLES driver that redraws the visible canvas on any dirty flag; the
// optimisation here is about removing Go-side work per frame (the previous
// implementation iterated every element and called Move+Resize on each of
// them on every drag event), not about skipping GPU work.
//
// Ghosting — stale pixels left behind at the previous position — is a
// possible symptom on some devices. If observed, the fix is a single line:
// add c.BaseWidget.Refresh() at the end of this function to force a
// whole-widget invalidation.
func (c *CanvasWidget) repositionSelectedOnly() {
	selected := c.ed.SelectedElement()
	if selected == nil {
		return
	}
	x, y := c.ed.DocToView(selected.Bounds.Position)
	if obj, ok := c.objects[selected.ID]; ok {
		obj.Move(fyne.NewPos(float32(x), float32(y)))
		obj.Refresh()
	}
	if c.selectionBox != nil && c.selectionBox.Visible() {
		c.selectionBox.Move(fyne.NewPos(float32(x)-2, float32(y)-2))
		c.selectionBox.Refresh()
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
		// Apply the metadata rotation. The helper returns the source image
		// unchanged when Rotation is 0, so the common case has no cost.
		// The same transform is applied by the print rasterizer, keeping
		// preview and print output in agreement.
		img = canvasPreviewImage(img, idata.Rotation)
		preview := canvas.NewImageFromImage(img)
		preview.FillMode = canvas.ImageFillContain
		return preview
	default:
		return canvas.NewRectangle(color.Gray{Y: 200})
	}
}

// canvasPreviewImage bounds the bitmap retained by an Android canvas object.
// The document keeps the original encoded bytes and the print renderer never
// sees this preview bitmap. Resizing and rotation happen when the canvas
// object is created or invalidated, never during drag events.
func canvasPreviewImage(src image.Image, rotation int) image.Image {
	if runtime.GOOS == "android" {
		bounds := src.Bounds()
		width, height := bounds.Dx(), bounds.Dy()
		if width > 0 && height > 0 && int64(width)*int64(height) > androidCanvasPreviewMaxPixels {
			pixelCount := int64(width) * int64(height)
			scale := math.Sqrt(float64(androidCanvasPreviewMaxPixels) / float64(pixelCount))
			previewWidth := max(1, int(float64(width)*scale))
			previewHeight := max(1, int(float64(height)*scale))
			preview := image.NewNRGBA(image.Rect(0, 0, previewWidth, previewHeight))
			xdraw.ApproxBiLinear.Scale(preview, preview.Bounds(), src, bounds, xdraw.Over, nil)
			src = preview
		}
	}
	return imageutil.Rotate(src, rotation)
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
	if !c.isDragging {
		c.isDragging = true
		c.lastDragPos = ev.Position
		// Select the element under the initial touch point. On Android a
		// drag usually does not produce a preceding Tapped event, so this
		// is what makes "touch an element and drag it" work in one gesture.
		// Re-selecting also handles the case where the user starts a drag
		// on a different element than the currently selected one.
		prevSel := c.ed.SelID
		pos := c.ed.ViewToDoc(float64(ev.Position.X), float64(ev.Position.Y))
		c.ed.SelectByPoint(pos)
		if c.ed.SelID != prevSel && c.onSelect != nil {
			c.onSelect(c.ed.SelID)
		}
		// Full refresh on gesture start only, so the selection box appears
		// or moves to the newly selected element before the fast path takes
		// over.
		c.Refresh()
		return
	}
	if c.ed.SelID == "" {
		return
	}
	dx := ev.Position.X - c.lastDragPos.X
	dy := ev.Position.Y - c.lastDragPos.Y
	if dx == 0 && dy == 0 {
		return
	}
	scale := editor.ScreenDPI / 25.4 * c.ed.Zoom
	if scale <= 0 {
		return
	}
	docDx := document.Unit(float64(dx) * 1000.0 / scale)
	docDy := document.Unit(float64(dy) * 1000.0 / scale)
	c.ed.MoveSelected(docDx, docDy)
	c.lastDragPos = ev.Position
	c.repositionSelectedOnly()
}

func (c *CanvasWidget) DragEnd() {
	c.isDragging = false
}
