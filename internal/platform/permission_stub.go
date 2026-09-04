//go:build !android

package platform

import "context"

func getAndroidAPIVersion() int {
	return 0
}

func checkBluetoothConnectPermission(ctx context.Context) (bool, error) {
	return false, nil
}

func ensureBluetoothConnectPermission(ctx context.Context) (bool, error) {
	return false, nil
}

func checkBluetoothScanPermission(ctx context.Context) (bool, error) {
	return false, nil
}

func ensureBluetoothScanPermission(ctx context.Context) (bool, error) {
	return false, nil
}
