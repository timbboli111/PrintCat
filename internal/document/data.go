package document

type TextData struct {
	Content   string `json:"content"`
	FontSize  Unit   `json:"fontSize"`
	Alignment string `json:"alignment,omitempty"`
}

type ImageData struct {
	Data     []byte `json:"data"`
	MimeType string `json:"mimeType"`
}
