package document

func NewTextElement(id, content string, bounds Rect, fontSize Unit) Element {
	return Element{
		ID:      id,
		Type:    TextElement,
		Bounds:  bounds,
		ZIndex:  0,
		Visible: true,
		Data:    TextData{Content: content, FontSize: fontSize},
	}
}

// NewImageElement creates an image element and seeds BaseSize with the
// caller-provided bounds. BaseSize is the unrotated, unconstrained size of
// the image; the editor uses it as the source of truth when recomputing
// bounds across rotations, so that a rotation that had to shrink the
// element to fit the page does not permanently reduce its size.
func NewImageElement(id string, data []byte, mimeType string, bounds Rect) Element {
	return Element{
		ID:      id,
		Type:    ImageElement,
		Bounds:  bounds,
		ZIndex:  0,
		Visible: true,
		Data: ImageData{
			Data:     data,
			MimeType: mimeType,
			BaseSize: bounds.Size,
		},
	}
}
