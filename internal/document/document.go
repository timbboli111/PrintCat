// Package document contains the printer-independent PrintCat document model.
package document

import "fmt"

// Unit is a logical document coordinate. One unit is one micrometre.
// Physical units prevent a saved design from depending on screen DPI or printer protocol.
type Unit int64

// Point is a document coordinate.
type Point struct {
	X Unit `json:"x"`
	Y Unit `json:"y"`
}

// Size is a document size.
type Size struct {
	Width  Unit `json:"width"`
	Height Unit `json:"height"`
}

// Rect identifies an object's printable bounds.
type Rect struct {
	Position Point `json:"position"`
	Size     Size  `json:"size"`
}

// ElementType describes the semantic content of an element, never its printer encoding.
type ElementType string

const (
	TextElement      ElementType = "text"
	ImageElement     ElementType = "image"
	QRCodeElement    ElementType = "qr-code"
	BarcodeElement   ElementType = "barcode"
	LineElement      ElementType = "line"
	RectangleElement ElementType = "rectangle"
)

// Element is the common base for every printable object. Concrete properties are
// deliberately deferred until editor and renderer requirements are implemented.
type Element struct {
	ID      string      `json:"id"`
	Type    ElementType `json:"type"`
	Bounds  Rect        `json:"bounds"`
	ZIndex  int         `json:"zIndex"`
	Locked  bool        `json:"locked,omitempty"`
	Visible bool        `json:"visible"`
}

// Document is a versioned, platform-independent thermal-print design.
type Document struct {
	Version  int       `json:"version"`
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	PageSize Size      `json:"pageSize"`
	Elements []Element `json:"elements"`
}

// New creates an empty document. Content editors add typed elements in a later phase.
func New(id, name string, pageSize Size) Document {
	return Document{Version: 1, ID: id, Name: name, PageSize: pageSize, Elements: []Element{}}
}

// Validate performs only structural checks shared by all renderers and protocols.
func (d Document) Validate() error {
	if d.Version < 1 {
		return fmt.Errorf("document version must be positive")
	}
	if d.ID == "" {
		return fmt.Errorf("document id is required")
	}
	if d.PageSize.Width <= 0 || d.PageSize.Height <= 0 {
		return fmt.Errorf("page size must be positive")
	}
	seen := make(map[string]struct{}, len(d.Elements))
	for _, element := range d.Elements {
		if element.ID == "" {
			return fmt.Errorf("element id is required")
		}
		if _, exists := seen[element.ID]; exists {
			return fmt.Errorf("duplicate element id %q", element.ID)
		}
		seen[element.ID] = struct{}{}
	}
	return nil
}
