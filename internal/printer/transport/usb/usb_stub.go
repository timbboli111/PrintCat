//go:build !android

package usb

import (
	"context"
	"fmt"
)

func (t *USBTransport) send(ctx context.Context, endpoint string, payload []byte, options map[string]string) error {
	return fmt.Errorf("usb transport is only supported on Android")
}
