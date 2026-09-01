// Package render defines reusable document rendering contracts for previews and output.
package render

import (
	"context"

	"github.com/printcat/printcat/internal/document"
)

// Target describes requested output characteristics without selecting a printer protocol.
type Target struct {
	DPI        int
	Monochrome bool
}

// Raster is protocol-neutral rendered image data. Pixels are intentionally opaque
// to this foundation; concrete encoders decide their accepted image representation.
type Raster struct {
	Width  int
	Height int
	Pixels []byte
}

// Renderer turns a document into a protocol-neutral visual representation.
// It is shared by previews, generic raster printing, and protocol backends.
type Renderer interface {
	Render(ctx context.Context, doc document.Document, target Target) (Raster, error)
}
