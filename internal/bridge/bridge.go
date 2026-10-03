// Package bridge connects external print-job sources (such as the Android
// Print Framework via JNI) to the existing PrintCat printer engine. It does
// not implement any printer protocol, transport, or rendering itself — it
// delegates everything to internal/printer.Service and internal/render.
package bridge

import (
	"context"
	"fmt"
	"sync"

	"github.com/timboli111/PrintCat/internal/document"
	"github.com/timboli111/PrintCat/internal/printer"
	"github.com/timboli111/PrintCat/internal/render"
)

// SubmitRequest describes one logical print job submitted from outside
// PrintCat (for example, from the Android Print Framework). Each page is a
// pre-rendered PNG image; the bridge wraps each page in a document.Document
// and hands it to printer.Service.Print.
type SubmitRequest struct {
	Pages         [][]byte
	PageWidthsUm  []int64
	PageHeightsUm []int64
	DPI           int
	// PrinterID is reserved for future multi-printer support. When empty,
	// the currently configured active printer is used.
	PrinterID string
}

// State holds the shared PrintCat engine used by both the Fyne UI and the
// external print-job path. It is safe for concurrent use.
type State struct {
	service  *printer.Service
	renderer render.Renderer

	mu            sync.RWMutex
	activePrinter *printer.Printer
}

// NewState creates a State around the already-initialized printer service
// and renderer. The service must have its protocol backends and transports
// registered before Submit is called.
func NewState(service *printer.Service, renderer render.Renderer) *State {
	return &State{service: service, renderer: renderer}
}

// SetActivePrinter records the printer that external jobs will be sent to.
// Pass nil to clear.
func (s *State) SetActivePrinter(p *printer.Printer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.activePrinter = p
}

// ActivePrinter returns the currently configured printer, or nil if none.
func (s *State) ActivePrinter() *printer.Printer {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.activePrinter
}

// Submit processes one external print job. Each page in req.Pages is
// rendered as a single ImageElement document, encoded by the registered
// protocol backend for the active printer, and sent through the registered
// transport.
//
// An error is returned if:
//   - no active printer is configured
//   - the request has no pages or mismatched page metadata
//   - the request DPI is not positive
//   - the engine fails to encode or send any page
func (s *State) Submit(ctx context.Context, req SubmitRequest) error {
	if s.service == nil {
		return fmt.Errorf("printer service is not initialized")
	}
	if s.renderer == nil {
		return fmt.Errorf("renderer is not initialized")
	}

	p := s.ActivePrinter()
	if p == nil {
		return fmt.Errorf("no active printer configured")
	}
	if len(req.Pages) == 0 {
		return fmt.Errorf("no pages to print")
	}
	if len(req.PageWidthsUm) != len(req.Pages) || len(req.PageHeightsUm) != len(req.Pages) {
		return fmt.Errorf("page metadata mismatch: %d pages, %d widths, %d heights",
			len(req.Pages), len(req.PageWidthsUm), len(req.PageHeightsUm))
	}
	if req.DPI <= 0 {
		return fmt.Errorf("invalid DPI: %d", req.DPI)
	}

	// Work on a copy so we do not mutate the persisted active printer.
	target := *p
	target.Profile.DPI = req.DPI

	for i := range req.Pages {
		widthUm := req.PageWidthsUm[i]
		heightUm := req.PageHeightsUm[i]
		if widthUm <= 0 || heightUm <= 0 {
			return fmt.Errorf("page %d has invalid size: %dx%d um", i+1, widthUm, heightUm)
		}

		doc := document.New(
			fmt.Sprintf("print-job-page-%d", i),
			fmt.Sprintf("Page %d", i+1),
			document.Size{
				Width:  document.Unit(widthUm),
				Height: document.Unit(heightUm),
			},
		)
		doc.Elements = append(doc.Elements, document.NewImageElement(
			fmt.Sprintf("page-%d", i),
			req.Pages[i],
			"image/png",
			document.Rect{
				Position: document.Point{},
				Size:     document.Size{Width: doc.PageSize.Width, Height: doc.PageSize.Height},
			},
		))

		if err := s.service.Print(ctx, target, doc, s.renderer); err != nil {
			return fmt.Errorf("page %d: %w", i+1, err)
		}
	}
	return nil
}
