package printer

import (
	"context"
	"testing"

	"github.com/printcat/printcat/internal/document"
	"github.com/printcat/printcat/internal/render"
)

type testBackend struct{ protocol Protocol }

func (b testBackend) Protocol() Protocol { return b.protocol }
func (b testBackend) Encode(_ context.Context, _ document.Document, _ render.Renderer, _ map[string]string) ([]byte, error) {
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
		profile := Printer{ID: string(transport), Connection: Connection{Protocol: ZPL, Transport: transport}}
		if err := service.Print(context.Background(), profile, doc, nil); err != nil {
			t.Fatalf("Print(%s) error = %v", transport, err)
		}
	}
	if len(usb.Payloads) != 1 || len(tcp.Payloads) != 1 {
		t.Fatalf("payloads usb=%d tcp=%d, want one each", len(usb.Payloads), len(tcp.Payloads))
	}
}
