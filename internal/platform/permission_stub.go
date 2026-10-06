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

func checkFineLocationPermission(ctx context.Context) (bool, error) {
	return false, nil
}

func ensureFineLocationPermission(ctx context.Context) (bool, error) {
	// Non-Android platforms do not require location permission for
	// Bluetooth discovery. The caller in fyneapp.discoverBluetoothPrinters
	// early-returns on non-Android anyway, so this value is only a
	// safeguard.
	return true, nil
}
