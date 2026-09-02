package printer

import (
	"context"
	"testing"

	"github.com/timboli111/PrintCat/internal/document"
	"github.com/timboli111/PrintCat/internal/render"
)

type testBackend struct{ protocol Protocol }

func (b testBackend) Protocol() Protocol { return b.protocol }
func (b testBackend) Encode(_ context.Context, _ document.Document, _ render.Renderer, _ PrinterProfile, _ map[string]string) ([]byte, error) {
	return []byte("encoded"), nil
}

func TestProtocolCanUseMultipleTransports(t *testing.T) {
	service := NewService()
	if err := service.RegisterBackend(testBackend{protocol: ZPL}); err != nil {
		t.Fatal(err)
	}
	usb := &MemoryTransport{TransportKind: USB}
	tcp := &MemoryTransport{TransportKind: TCP}
	if err := service.RegisterTransport(usb); err != nil {
		t.Fatal(err)
	}
	if err := service.RegisterTransport(tcp); err != nil {
		t.Fatal(err)
	}
	doc := document.New("doc-1", "Test", document.Size{Width: 80_000, Height: 100_000})
	for _, transport := range []TransportKind{USB, TCP} {
		profile := Printer{
			ID:         string(transport),
			Connection: Connection{Protocol: ZPL, Transport: transport},
			Profile:    PrinterProfile{DPI: 203},
		}
		if err := service.Print(context.Background(), profile, doc, nil); err != nil {
			t.Fatalf("Print(%s) error = %v", transport, err)
		}
	}
	if len(usb.Payloads) != 1 || len(tcp.Payloads) != 1 {
		t.Fatalf("payloads usb=%d tcp=%d, want one each", len(usb.Payloads), len(tcp.Payloads))
	}
}

func TestProfileValidate(t *testing.T) {
	valid := PrinterProfile{DPI: 203}
	if err := valid.Validate(); err != nil {
		t.Errorf("valid profile should pass, got %v", err)
	}
	invalidDPI := PrinterProfile{DPI: 0}
	if err := invalidDPI.Validate(); err == nil {
		t.Error("zero DPI should fail")
	}
	// MediaWidth zero is allowed
	zeroWidth := PrinterProfile{DPI: 203, MediaWidth: 0}
	if err := zeroWidth.Validate(); err != nil {
		t.Errorf("zero media width should be allowed, got %v", err)
	}
}

func TestPrinterValidate(t *testing.T) {
	// valid
	p := Printer{
		Connection: Connection{Protocol: ZPL, Transport: USB},
		Profile: PrinterProfile{
			DPI:                 203,
			SupportedProtocols:  []Protocol{ZPL},
			SupportedTransports: []TransportKind{USB},
		},
	}
	if err := p.Validate(); err != nil {
		t.Errorf("valid printer should pass, got %v", err)
	}

	// protocol not supported
	p2 := Printer{
		Connection: Connection{Protocol: ESCPOS},
		Profile: PrinterProfile{
			DPI:                 203,
			SupportedProtocols:  []Protocol{ZPL},
			SupportedTransports: []TransportKind{USB},
		},
	}
	if err := p2.Validate(); err == nil {
		t.Error("protocol not supported should fail")
	}

	// transport not supported
	p3 := Printer{
		Connection: Connection{Transport: TCP},
		Profile: PrinterProfile{
			DPI:                 203,
			SupportedProtocols:  []Protocol{ZPL},
			SupportedTransports: []TransportKind{USB},
		},
	}
	if err := p3.Validate(); err == nil {
		t.Error("transport not supported should fail")
	}

	// supported lists empty → should not reject any
	p4 := Printer{
		Connection: Connection{Protocol: ZPL, Transport: USB},
		Profile: PrinterProfile{
			DPI:                 203,
			SupportedProtocols:  []Protocol{},
			SupportedTransports: []TransportKind{},
		},
	}
	if err := p4.Validate(); err != nil {
		t.Errorf("empty supported lists should pass, got %v", err)
	}
}

func TestServicePrintValidation(t *testing.T) {
	service := NewService()
	// register dummy backend/transport agar tidak error di tahap selanjutnya
	service.RegisterBackend(testBackend{protocol: ZPL})
	service.RegisterTransport(&MemoryTransport{TransportKind: USB})
	doc := document.New("doc-1", "Test", document.Size{Width: 80_000, Height: 100_000})

	// printer with invalid DPI
	p := Printer{
		Connection: Connection{Protocol: ZPL, Transport: USB},
		Profile:    PrinterProfile{DPI: 0},
	}
	err := service.Print(context.Background(), p, doc, nil)
	if err == nil {
		t.Error("Print should fail due to invalid DPI")
	}
}
