//go:build windows

package bluetooth

import (
	"context"
	"fmt"
	"time"

	"github.com/tarm/serial"
)

func (t *BluetoothTransport) send(ctx context.Context, endpoint string, payload []byte, options map[string]string) error {
	if !isCOMFormat(endpoint) {
		return fmt.Errorf("invalid COM port: %s", endpoint)
	}
	if len(payload) == 0 {
		return nil
	}

	config := &serial.Config{
		Name:        endpoint,
		Baud:        getBaud(options),
		ReadTimeout: time.Duration(getTimeout(options)) * time.Second,
	}

	port, err := serial.OpenPort(config)
	if err != nil {
		return fmt.Errorf("open COM port: %w", err)
	}
	defer port.Close()

	// Tarm/serial tidak mendukung write deadline terpisah.
	// Kita gunakan context cancellation untuk membatalkan write.
	if _, ok := ctx.Deadline(); ok {
		// Context memiliki deadline, tetapi kita mengandalkan select dengan ctx.Done()
		// untuk menangani pembatalan.
	}

	done := make(chan error, 1)
	go func() {
		_, err := port.Write(payload)
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("write to COM port: %w", err)
		}
		return nil
	case <-ctx.Done():
		// Konteks dibatalkan, tetapi operasi write mungkin masih berjalan.
		// Kita kembalikan error dan biarkan goroutine selesai.
		return ctx.Err()
	}
}
