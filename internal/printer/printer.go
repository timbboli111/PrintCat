package printer

import (
	"context"
	"fmt"

	"github.com/timboli111/PrintCat/internal/document"
	"github.com/timboli111/PrintCat/internal/render"
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

type Connection struct {
	Protocol  Protocol
	Transport TransportKind
	Endpoint  string
	Options   map[string]string
}

type Printer struct {
	ID         string         `json:"id"`
	Name       string         `json:"name"`
	Connection Connection     `json:"connection"`
	Profile    PrinterProfile `json:"profile,omitempty"`
}

func (p Printer) Validate() error {
	if err := p.Profile.Validate(); err != nil {
		return fmt.Errorf("profile validation failed: %w", err)
	}
	if len(p.Profile.SupportedProtocols) > 0 {
		found := false
		for _, prot := range p.Profile.SupportedProtocols {
			if prot == p.Connection.Protocol {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("protocol %s not supported by profile (supported: %v)", p.Connection.Protocol, p.Profile.SupportedProtocols)
		}
	}
	if len(p.Profile.SupportedTransports) > 0 {
		found := false
		for _, tr := range p.Profile.SupportedTransports {
			if tr == p.Connection.Transport {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("transport %s not supported by profile (supported: %v)", p.Connection.Transport, p.Profile.SupportedTransports)
		}
	}
	return nil
}

type ProtocolBackend interface {
	Protocol() Protocol
	Encode(ctx context.Context, doc document.Document, renderer render.Renderer, profile PrinterProfile, options map[string]string) ([]byte, error)
}

type Transport interface {
	Kind() TransportKind
	Send(ctx context.Context, endpoint string, payload []byte, options map[string]string) error
}

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

func (s *Service) Print(ctx context.Context, profile Printer, doc document.Document, renderer render.Renderer) error {
	if err := profile.Validate(); err != nil {
		return fmt.Errorf("invalid printer: %w", err)
	}
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
	payload, err := backend.Encode(ctx, doc, renderer, profile.Profile, profile.Connection.Options)
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
