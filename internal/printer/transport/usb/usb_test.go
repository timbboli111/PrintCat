package usb

import (
	"context"
	"runtime"
	"testing"

	"github.com/timboli111/PrintCat/internal/printer"
)

func TestKind(t *testing.T) {
	tr := &USBTransport{}
	if tr.Kind() != printer.USB {
		t.Errorf("expected USB, got %v", tr.Kind())
	}
}

func TestSendRejectsEmptyEndpoint(t *testing.T) {
	tr := &USBTransport{}
	err := tr.Send(context.Background(), "", []byte{0x01}, nil)
	if err == nil {
		t.Error("expected error for empty endpoint")
	}
}

func TestSendEmptyPayloadIsNoop(t *testing.T) {
	tr := &USBTransport{}
	// Empty payload short-circuits before touching the platform-specific
	// path, so this must succeed on any OS.
	err := tr.Send(context.Background(), "0", []byte{}, nil)
	if err != nil {
		t.Errorf("empty payload should be a no-op, got %v", err)
	}
}

func TestSendUnsupportedOnNonAndroid(t *testing.T) {
	if runtime.GOOS == "android" {
		t.Skip("this test verifies non-Android stub behaviour")
	}
	tr := &USBTransport{}
	err := tr.Send(context.Background(), "0", []byte{0x01}, nil)
	if err == nil {
		t.Error("expected error on non-Android platform")
	}
}

func TestServiceAcceptsUSBTransportRegistration(t *testing.T) {
	svc := printer.NewService()
	if err := svc.RegisterTransport(&USBTransport{}); err != nil {
		t.Fatalf("RegisterTransport(USB) failed: %v", err)
	}
	// Second registration must fail with "already registered".
	if err := svc.RegisterTransport(&USBTransport{}); err == nil {
		t.Error("second RegisterTransport(USB) should fail")
	}
}
