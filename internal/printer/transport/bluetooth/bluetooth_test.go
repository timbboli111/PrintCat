package bluetooth

import (
	"context"
	"runtime"
	"testing"

	"github.com/timboli111/PrintCat/internal/printer"
)

func TestKind(t *testing.T) {
	tr := &BluetoothTransport{}
	if tr.Kind() != printer.BluetoothClassic {
		t.Errorf("expected BluetoothClassic, got %v", tr.Kind())
	}
}

func TestSendEmptyEndpoint(t *testing.T) {
	tr := &BluetoothTransport{}
	ctx := context.Background()
	err := tr.Send(ctx, "", []byte{0x01, 0x02}, nil)
	if err == nil {
		t.Error("expected error for empty endpoint")
	}
}

func TestSendEmptyPayload(t *testing.T) {
	tr := &BluetoothTransport{}
	ctx := context.Background()

	// On Linux, Bluetooth is unsupported. The stub returns an error.
	if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
		err := tr.Send(ctx, "00:11:22:33:44:55", []byte{}, nil)
		if err == nil {
			t.Error("expected unsupported platform error on Linux/macOS")
		}
		// We don't check the exact error message, but we know it's not nil.
		return
	}

	// On Windows/Android, we cannot test actual hardware connection in CI.
	// We'll skip this test to avoid requiring a real Bluetooth device.
	// The endpoint validation and common logic are tested elsewhere.
	t.Skip("skipping on Windows/Android (requires real Bluetooth hardware)")
}

func TestInvalidEndpointFormat(t *testing.T) {
	tr := &BluetoothTransport{}
	ctx := context.Background()

	// On Linux, any endpoint is accepted by Send() because validation is not strict.
	// The stub will return "not supported" anyway.
	if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
		err := tr.Send(ctx, "invalid", []byte{0x01}, nil)
		if err == nil {
			t.Error("expected unsupported platform error")
		}
		return
	}

	// Windows: must be COM port format
	if runtime.GOOS == "windows" {
		err := tr.Send(ctx, "invalid", []byte{0x01}, nil)
		if err == nil {
			t.Error("expected error for invalid COM port format")
		}
	}

	// Android: must be MAC address format
	if runtime.GOOS == "android" {
		err := tr.Send(ctx, "invalid", []byte{0x01}, nil)
		if err == nil {
			t.Error("expected error for invalid MAC address format")
		}
	}
}

func TestValidEndpointFormat(t *testing.T) {
	// This test only verifies that endpoint format validation does not fail
	// for valid formats. It does not attempt to connect.
	tr := &BluetoothTransport{}
	ctx := context.Background()

	if runtime.GOOS == "windows" {
		// Valid COM port
		err := tr.Send(ctx, "COM5", []byte{0x01}, nil)
		// Error may occur due to no hardware, but not due to format.
		// We just ensure it's not a format error.
		if err != nil && err.Error() == "invalid COM port: COM5" {
			t.Error("valid COM port rejected")
		}
	}

	if runtime.GOOS == "android" {
		// Valid MAC address
		err := tr.Send(ctx, "00:11:22:33:44:55", []byte{0x01}, nil)
		if err != nil && err.Error() == "invalid MAC address: 00:11:22:33:44:55" {
			t.Error("valid MAC address rejected")
		}
	}

	if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
		// Linux accepts any endpoint (validation not strict)
		err := tr.Send(ctx, "00:11:22:33:44:55", []byte{0x01}, nil)
		if err == nil {
			t.Error("expected unsupported platform error")
		}
	}
}

func TestContextCancellation(t *testing.T) {
	tr := &BluetoothTransport{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := tr.Send(ctx, "00:11:22:33:44:55", []byte{0x01}, nil)

	if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
		// Stub: should return unsupported error
		if err == nil {
			t.Error("expected unsupported platform error")
		}
	} else {
		// On supported platforms, connection will fail; just ensure no panic.
		_ = err
	}
}

func TestOptionValidation(t *testing.T) {
	tr := &BluetoothTransport{}
	ctx := context.Background()

	if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
		// On unsupported platforms, we don't care about options; stub returns error.
		return
	}

	endpoint := "00:11:22:33:44:55"
	if runtime.GOOS == "windows" {
		endpoint = "COM5"
	}

	// Invalid channel (only validated in platform-specific code, but we can test common logic)
	options := map[string]string{"channel": "99"}
	err := tr.Send(ctx, endpoint, []byte{0x01}, options)
	// We just ensure it doesn't panic; error is expected due to no hardware.
	_ = err

	// Invalid baud (Windows only)
	if runtime.GOOS == "windows" {
		options = map[string]string{"baud": "0"}
		err := tr.Send(ctx, endpoint, []byte{0x01}, options)
		// Error may be due to invalid baud or connection failure; not panic.
		_ = err
	}
}

func TestIsMACFormat(t *testing.T) {
	testCases := []struct {
		input string
		want  bool
	}{
		{"00:11:22:33:44:55", true},
		{"00:11:22:33:44:55:66", false},
		{"00:11:22:33:44:5", false},
		{"00-11-22-33-44-55", false},
		{"001122334455", false},
		{"COM5", false},
		{"", false},
	}
	for _, tc := range testCases {
		got := isMACFormat(tc.input)
		if got != tc.want {
			t.Errorf("isMACFormat(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestIsCOMFormat(t *testing.T) {
	testCases := []struct {
		input string
		want  bool
	}{
		{"COM1", true},
		{"COM5", true},
		{"COM10", true},
		{"COM", false},
		{"COM0", true},
		{"COMA", false},
		{"00:11:22:33:44:55", false},
	}
	for _, tc := range testCases {
		got := isCOMFormat(tc.input)
		if got != tc.want {
			t.Errorf("isCOMFormat(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}
