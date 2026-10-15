package editor

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"

	"github.com/timboli111/PrintCat/internal/document"
)

const ScreenDPI = 96.0

// DefaultImageMargin is the physical margin used when an imported image is
// initially placed on a document. Document units are micrometres, so this is
// a 5 mm margin. It gives imported labels a printable inset while still using
// the available paper width instead of an arbitrary screen-sized rectangle.
const DefaultImageMargin document.Unit = 5_000

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

// AddImageFitToPage decodes an imported image and adds it centred within the
// document's printable area. Its aspect ratio is preserved in physical
// document units, which keeps the editor preview and later print rendering in
// agreement regardless of the source image pixel density.
func (e *Editor) AddImageFitToPage(data []byte, mime string) error {
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("decode image: %w", err)
	}
	if config.Width <= 0 || config.Height <= 0 {
		return fmt.Errorf("decode image: invalid dimensions %dx%d", config.Width, config.Height)
	}

	margin := DefaultImageMargin
	if e.Doc.PageSize.Width <= 2*margin || e.Doc.PageSize.Height <= 2*margin {
		margin = 0
	}
	availableWidth := e.Doc.PageSize.Width - 2*margin
	availableHeight := e.Doc.PageSize.Height - 2*margin
	if availableWidth <= 0 || availableHeight <= 0 {
		return fmt.Errorf("document page size must be positive")
	}

	width := availableWidth
	height := document.Unit(int64(width) * int64(config.Height) / int64(config.Width))
	if height > availableHeight {
		height = availableHeight
		width = document.Unit(int64(height) * int64(config.Width) / int64(config.Height))
	}
	if width <= 0 || height <= 0 {
		return fmt.Errorf("image does not fit document page")
	}

	e.AddImage(data, mime, document.Point{
		X: (e.Doc.PageSize.Width - width) / 2,
		Y: (e.Doc.PageSize.Height - height) / 2,
	}, document.Size{Width: width, Height: height})
	return nil
}

// DetectImageMime inspects the leading bytes of data and returns the MIME
// type for a supported image format. Supported formats are PNG and JPEG
// (the blank imports at the top of this file register their decoders).
// It never trusts the file extension or any caller-supplied hint.
// Returns an error for empty, corrupt, or unsupported image data.
func DetectImageMime(data []byte) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("image data is empty")
	}
	_, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("decode image: %w", err)
	}
	switch format {
	case "jpeg":
		return "image/jpeg", nil
	case "png":
		return "image/png", nil
	default:
		return "", fmt.Errorf("unsupported image format: %q", format)
	}
}

// SelectedElement returns a pointer to the currently selected element in the
// document, or nil if there is no selection or the selection is stale.
func (e *Editor) SelectedElement() *document.Element {
	if e.SelID == "" {
		return nil
	}
	for i := range e.Doc.Elements {
		if e.Doc.Elements[i].ID == e.SelID {
			return &e.Doc.Elements[i]
		}
	}
	return nil
}

// snapToQuarterTurn rounds deg to the nearest of 0, 90, 180, 270 (mod 360).
// It is used to normalize potentially corrupted metadata before advancing
// the rotation, so that bounds and pixel transform always agree.
func snapToQuarterTurn(deg int) int {
	d := ((deg % 360) + 360) % 360
	switch {
	case d < 45:
		return 0
	case d < 135:
		return 90
	case d < 225:
		return 180
	case d < 315:
		return 270
	default:
		return 0
	}
}

// RotateSelectedImage advances the rotation of the currently selected image
// element by 90° clockwise.
//
// It is a no-op when nothing is selected, and it returns an error when the
// selection is not an image. The pixel payload is never re-encoded: only the
// ImageData.Rotation field changes, so both the Fyne preview and the print
// rasterizer will apply the same rotation on top of the original bytes. This
// avoids cumulative JPEG generation loss and any decoding cost when the user
// presses the button.
//
// The element's intrinsic, unrotated size is stored on ImageData.BaseSize.
// When the current bounds have been shrunk by a previous rotation to fit the
// page, the new bounds are still computed from BaseSize, not from the shrunk
// bounds, so repeated rotations do not shrink the element further.
// Legacy elements (BaseSize zero) lazily acquire BaseSize from the current
// bounds on their first rotation; this is safe because legacy elements have
// Rotation=0, so their current bounds ARE the unrotated size.
//
// The rotated bounds are constrained to the page: if the desired size does
// not fit within (page minus the same margin used by AddImageFitToPage), it
// is scaled down proportionally, preserving aspect ratio and never cropping.
// The visual centre is kept when the rotated element fits symmetrically;
// otherwise the position is clamped back onto the page. The clamp uses
// [0, pageSize], not [margin, pageSize-margin], because DefaultImageMargin
// is a placement convention for newly imported images, not a hard
// constraint on where the user may position content.
func (e *Editor) RotateSelectedImage() error {
	el := e.SelectedElement()
	if el == nil {
		return nil
	}
	if el.Type != document.ImageElement {
		return fmt.Errorf("selection is not an image")
	}
	idata, ok := el.Data.(document.ImageData)
	if !ok {
		return fmt.Errorf("selected image has invalid data")
	}

	idata.Rotation = snapToQuarterTurn(idata.Rotation)

	if idata.BaseSize.Width <= 0 || idata.BaseSize.Height <= 0 {
		idata.BaseSize = el.Bounds.Size
	}

	newRotation := (idata.Rotation + 90) % 360
	idata.Rotation = newRotation
	el.Data = idata

	var desiredW, desiredH document.Unit
	if newRotation == 90 || newRotation == 270 {
		desiredW = idata.BaseSize.Height
		desiredH = idata.BaseSize.Width
	} else {
		desiredW = idata.BaseSize.Width
		desiredH = idata.BaseSize.Height
	}

	margin := DefaultImageMargin
	if e.Doc.PageSize.Width <= 2*margin || e.Doc.PageSize.Height <= 2*margin {
		margin = 0
	}
	maxW := e.Doc.PageSize.Width - 2*margin
	maxH := e.Doc.PageSize.Height - 2*margin
	if maxW <= 0 {
		maxW = e.Doc.PageSize.Width
	}
	if maxH <= 0 {
		maxH = e.Doc.PageSize.Height
	}

	finalW := desiredW
	finalH := desiredH
	if finalW > maxW || finalH > maxH {
		scaleW := float64(maxW) / float64(finalW)
		scaleH := float64(maxH) / float64(finalH)
		s := scaleW
		if scaleH < s {
			s = scaleH
		}
		finalW = document.Unit(float64(finalW) * s)
		finalH = document.Unit(float64(finalH) * s)
		if finalW <= 0 {
			finalW = 1
		}
		if finalH <= 0 {
			finalH = 1
		}
	}

	cx := el.Bounds.Position.X + el.Bounds.Size.Width/2
	cy := el.Bounds.Position.Y + el.Bounds.Size.Height/2

	newX := cx - finalW/2
	newY := cy - finalH/2

	if newX < 0 {
		newX = 0
	}
	if newY < 0 {
		newY = 0
	}
	if newX+finalW > e.Doc.PageSize.Width {
		newX = e.Doc.PageSize.Width - finalW
	}
	if newY+finalH > e.Doc.PageSize.Height {
		newY = e.Doc.PageSize.Height - finalH
	}
	if newX < 0 {
		newX = 0
	}
	if newY < 0 {
		newY = 0
	}

	el.Bounds.Size.Width = finalW
	el.Bounds.Size.Height = finalH
	el.Bounds.Position.X = newX
	el.Bounds.Position.Y = newY
	return nil
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
