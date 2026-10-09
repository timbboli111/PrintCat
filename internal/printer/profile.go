package printer

import (
	"fmt"

	"github.com/timboli111/PrintCat/internal/document"
)

type MediaType string

const (
	MediaReceipt    MediaType = "receipt"
	MediaLabel      MediaType = "label"
	MediaTag        MediaType = "tag"
	MediaContinuous MediaType = "continuous"
)

// PrinterProfile describes the physical and logical capabilities of a
// printer. Fields are filled in from the saved printer configuration and
// travel with the printer definition through the pipeline into the
// protocol backends.
//
// Density is the raster binarization threshold used by the ESC/POS
// encoder. It is ignored by all other protocol backends. Valid range is
// 1-254; a value <= 0 means "use the encoder default".
type PrinterProfile struct {
	Vendor              string          `json:"vendor,omitempty"`
	Model               string          `json:"model,omitempty"`
	DPI                 int             `json:"dpi"`
	MediaType           MediaType       `json:"mediaType,omitempty"`
	MediaWidth          document.Unit   `json:"mediaWidth,omitempty"`
	SupportsCutter      bool            `json:"supportsCutter,omitempty"`
	SupportsLabel       bool            `json:"supportsLabel,omitempty"`
	SupportsReceipt     bool            `json:"supportsReceipt,omitempty"`
	MonochromeOnly      bool            `json:"monochromeOnly,omitempty"`
	SupportedProtocols  []Protocol      `json:"supportedProtocols,omitempty"`
	SupportedTransports []TransportKind `json:"supportedTransports,omitempty"`
	// Density is the raster threshold for ESC/POS. 0 or negative means
	// "use the encoder default (160)".
	Density int `json:"density,omitempty"`
}

func (p PrinterProfile) Validate() error {
	if p.DPI <= 0 {
		return fmt.Errorf("DPI must be positive (got %d)", p.DPI)
	}
	return nil
}
