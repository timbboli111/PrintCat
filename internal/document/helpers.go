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

func NewImageElement(id string, data []byte, mimeType string, bounds Rect) Element {
	return Element{
		ID:      id,
		Type:    ImageElement,
		Bounds:  bounds,
		ZIndex:  0,
		Visible: true,
		Data:    ImageData{Data: data, MimeType: mimeType},
	}
}
