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
