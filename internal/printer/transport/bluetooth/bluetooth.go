package bluetooth

import (
	"context"
	"fmt"
	"regexp"
	"strconv"

	"github.com/timboli111/PrintCat/internal/printer"
)

type BluetoothTransport struct{}

func (t *BluetoothTransport) Kind() printer.TransportKind {
	return printer.BluetoothClassic
}

func (t *BluetoothTransport) Send(ctx context.Context, endpoint string, payload []byte, options map[string]string) error {
	if endpoint == "" {
		return fmt.Errorf("bluetooth endpoint required")
	}
	// MAC address format validation (for Android/Linux)
	// Windows may use COM ports, which don't match MAC pattern
	if !isCOMFormat(endpoint) && !isMACFormat(endpoint) {
		// For Windows, COM ports are valid; for Android, we need MAC
		// We'll let the platform-specific implementation handle validation
	}
	return t.send(ctx, endpoint, payload, options)
}

func isMACFormat(s string) bool {
	match, _ := regexp.MatchString(`^([0-9A-Fa-f]{2}:){5}[0-9A-Fa-f]{2}$`, s)
	return match
}

func isCOMFormat(s string) bool {
	match, _ := regexp.MatchString(`^COM\d+$`, s)
	return match
}

func getTimeout(options map[string]string) int {
	if v, ok := options["connect_timeout"]; ok {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 10
}

func getWriteTimeout(options map[string]string) int {
	if v, ok := options["write_timeout"]; ok {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 5
}

func getBaud(options map[string]string) int {
	if v, ok := options["baud"]; ok {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 9600
}

func getChannel(options map[string]string) int {
	if v, ok := options["channel"]; ok {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 && n <= 30 {
			return n
		}
	}
	return 1
}

// awaitSend returns pending=true when ctx expires before the operation has
// completed. The caller must treat that as a still-running native operation;
// cancellation of the context does not cancel an in-flight JNI call.
func awaitSend(ctx context.Context, done <-chan error) (err error, pending bool) {
	select {
	case err := <-done:
		return err, false
	case <-ctx.Done():
		select {
		case <-done:
			return ctx.Err(), false
		default:
			return ctx.Err(), true
		}
	}
}

func diagnosticToken(value string) string {
	if value == "" {
		return "unknown"
	}
	const maxLength = 80
	if len(value) > maxLength {
		value = value[:maxLength]
	}
	result := []byte(value)
	for i, b := range result {
		if (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') ||
			(b >= '0' && b <= '9') || b == '-' || b == '_' || b == '.' {
			continue
		}
		result[i] = '_'
	}
	return string(result)
}
