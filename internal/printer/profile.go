package printer

import (
	"fmt"

	"github.com/timboli111/PrintCat/internal/document"
)

const MaxSmartTrimBottomMarginUm document.Unit = 100_000

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
	// SmartTrimEnabled enables conservative bottom trimming in the ESC/POS
	// encoder. Other protocol backends currently ignore these settings.
	SmartTrimEnabled bool `json:"smartTrimEnabled,omitempty"`
	// SmartTrimBottomMarginUm is the retained blank margin below detected
	// content, stored in micrometres (1 mm = 1,000 µm). Zero selects the
	// ESC/POS encoder's safe default margin of 2.5 mm.
	SmartTrimBottomMarginUm document.Unit `json:"smartTrimBottomMarginUm,omitempty"`
}

func (p PrinterProfile) Validate() error {
	if p.DPI <= 0 {
		return fmt.Errorf("DPI must be positive (got %d)", p.DPI)
	}
	if err := ValidateSmartTrimBottomMarginUm(p.SmartTrimBottomMarginUm); err != nil {
		return err
	}
	return nil
}

// ValidateSmartTrimBottomMarginUm checks the stored Smart Trim margin even
// when trimming is disabled, so an invalid persisted value cannot become an
// active setting later without being corrected.
func ValidateSmartTrimBottomMarginUm(marginUm document.Unit) error {
	if marginUm < 0 || marginUm > MaxSmartTrimBottomMarginUm {
		return fmt.Errorf("Smart Trim bottom margin must be between 0 and %d µm (got %d µm)",
			MaxSmartTrimBottomMarginUm, marginUm)
	}
	return nil
}
