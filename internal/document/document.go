package document

import "fmt"

type Unit int64

type Point struct {
	X Unit `json:"x"`
	Y Unit `json:"y"`
}

type Size struct {
	Width  Unit `json:"width"`
	Height Unit `json:"height"`
}

type Rect struct {
	Position Point `json:"position"`
	Size     Size  `json:"size"`
}

type ElementType string

const (
	TextElement      ElementType = "text"
	ImageElement     ElementType = "image"
	QRCodeElement    ElementType = "qr-code"
	BarcodeElement   ElementType = "barcode"
	LineElement      ElementType = "line"
	RectangleElement ElementType = "rectangle"
)

type Element struct {
	ID      string      `json:"id"`
	Type    ElementType `json:"type"`
	Bounds  Rect        `json:"bounds"`
	ZIndex  int         `json:"zIndex"`
	Locked  bool        `json:"locked,omitempty"`
	Visible bool        `json:"visible"`
	Data    interface{} `json:"data,omitempty"`
}

type Document struct {
	Version  int       `json:"version"`
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	PageSize Size      `json:"pageSize"`
	Elements []Element `json:"elements"`
}

func New(id, name string, pageSize Size) Document {
	return Document{Version: 1, ID: id, Name: name, PageSize: pageSize, Elements: []Element{}}
}

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
