//go:build !android

package platform

import "context"

func checkBluetoothConnectPermission(ctx context.Context) (bool, error) {
	return false, nil
}

func ensureBluetoothConnectPermission(ctx context.Context) (bool, error) {
	return false, nil
}
