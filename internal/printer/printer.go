// Package printer coordinates independent printer protocol and transport implementations.
package printer

import (
	"context"
	"fmt"

	"github.com/printcat/printcat/internal/document"
	"github.com/printcat/printcat/internal/render"
)

type Protocol string

const (
	ESCPOS         Protocol = "esc-pos"
	ESCX           Protocol = "esc-x"
	StarPRNT       Protocol = "star-prnt"
	ZPL            Protocol = "zpl"
	EPL            Protocol = "epl"
	CPCL           Protocol = "cpcl"
	TSPL           Protocol = "tspl"
	DPL            Protocol = "dpl"
	SBPL           Protocol = "sbpl"
	IPL            Protocol = "ipl"
	GenericRaster  Protocol = "generic-raster"
	VendorSpecific Protocol = "vendor-specific"
)

type TransportKind string

const (
	BluetoothClassic TransportKind = "bluetooth-classic"
	BLE              TransportKind = "ble"
	USB              TransportKind = "usb"
	Serial           TransportKind = "serial"
	TCP              TransportKind = "tcp"
	WindowsSpooler   TransportKind = "windows-spooler"
	AndroidPrint     TransportKind = "android-print-framework"
)

// Connection selects an independently configurable protocol and transport endpoint.
type Connection struct {
	Protocol  Protocol
	Transport TransportKind
	Endpoint  string
	Options   map[string]string
}

// Printer is a saved, user-visible printer profile. It does not open a connection.
type Printer struct {
	ID         string
	Name       string
	Connection Connection
}

// ProtocolBackend generates bytes for exactly one printer command language.
// It never discovers devices, pairs Bluetooth, or sends bytes.
type ProtocolBackend interface {
	Protocol() Protocol
	Encode(ctx context.Context, doc document.Document, renderer render.Renderer, options map[string]string) ([]byte, error)
}

// Transport sends already-encoded printer bytes. It never formats protocol commands.
type Transport interface {
	Kind() TransportKind
	Send(ctx context.Context, endpoint string, payload []byte, options map[string]string) error
}

// Service combines registered implementations only at the final send boundary.
type Service struct {
	backends   map[Protocol]ProtocolBackend
	transports map[TransportKind]Transport
}

func NewService() *Service {
	return &Service{backends: map[Protocol]ProtocolBackend{}, transports: map[TransportKind]Transport{}}
}

func (s *Service) RegisterBackend(backend ProtocolBackend) error {
	if backend == nil {
		return fmt.Errorf("protocol backend is required")
	}
	protocol := backend.Protocol()
	if _, exists := s.backends[protocol]; exists {
		return fmt.Errorf("protocol backend already registered: %s", protocol)
	}
	s.backends[protocol] = backend
	return nil
}

func (s *Service) RegisterTransport(transport Transport) error {
	if transport == nil {
		return fmt.Errorf("transport is required")
	}
	kind := transport.Kind()
	if _, exists := s.transports[kind]; exists {
		return fmt.Errorf("transport already registered: %s", kind)
	}
	s.transports[kind] = transport
	return nil
}

// Print encodes with the selected protocol and sends through the selected transport.
func (s *Service) Print(ctx context.Context, profile Printer, doc document.Document, renderer render.Renderer) error {
	if err := doc.Validate(); err != nil {
		return fmt.Errorf("validate document: %w", err)
	}
	backend, exists := s.backends[profile.Connection.Protocol]
	if !exists {
		return fmt.Errorf("no protocol backend registered for %q", profile.Connection.Protocol)
	}
	transport, exists := s.transports[profile.Connection.Transport]
	if !exists {
		return fmt.Errorf("no transport registered for %q", profile.Connection.Transport)
	}
	payload, err := backend.Encode(ctx, doc, renderer, profile.Connection.Options)
	if err != nil {
		return fmt.Errorf("encode %s: %w", backend.Protocol(), err)
	}
	if len(payload) == 0 {
		return fmt.Errorf("encode %s: empty payload", backend.Protocol())
	}
	if err := transport.Send(ctx, profile.Connection.Endpoint, payload, profile.Connection.Options); err != nil {
		return fmt.Errorf("send via %s: %w", transport.Kind(), err)
	}
	return nil
}
