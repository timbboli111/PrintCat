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
}

func (p PrinterProfile) Validate() error {
	if p.DPI <= 0 {
		return fmt.Errorf("DPI must be positive (got %d)", p.DPI)
	}

	return nil
}
