//go:build !windows && !android

package bluetooth

import (
	"context"
	"fmt"
)

func (t *BluetoothTransport) send(ctx context.Context, endpoint string, payload []byte, options map[string]string) error {
	return fmt.Errorf("bluetooth transport not supported on this platform")
}
