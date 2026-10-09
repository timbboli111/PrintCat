// Package usb implements the printer.Transport for USB thermal printers.
//
// On Android the transport talks to the Android USB Host API through a
// small Java helper (UsbPrinterHelper). On other platforms the transport
// returns "not supported". USB is a distinct transport from Serial: a
// USB-serial device that appears as a CDC/ACM interface is NOT handled
// here; it belongs to the Serial transport.
package usb

import (
	"context"
	"fmt"

	"github.com/timboli111/PrintCat/internal/printer"
)

// USBTransport sends already-encoded protocol bytes to a USB thermal
// printer over a bulk OUT endpoint. It does not encode anything itself.
type USBTransport struct{}

func (t *USBTransport) Kind() printer.TransportKind {
	return printer.USB
}

// Send writes payload to the USB device identified by endpoint.
//
// endpoint is the USB device identifier returned by discovery. It is
// intentionally opaque to callers: the discovery layer chooses the format.
func (t *USBTransport) Send(ctx context.Context, endpoint string, payload []byte, options map[string]string) error {
	if endpoint == "" {
		return fmt.Errorf("usb endpoint (device id) required")
	}
	if len(payload) == 0 {
		return nil
	}
	return t.send(ctx, endpoint, payload, options)
}
