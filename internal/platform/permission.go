package platform

import "context"

// CheckBluetoothConnectPermission returns true if the BLUETOOTH_CONNECT permission is granted.
func CheckBluetoothConnectPermission(ctx context.Context) (bool, error) {
	return checkBluetoothConnectPermission(ctx)
}

// EnsureBluetoothConnectPermission requests the BLUETOOTH_CONNECT permission if not granted,
// and waits for the user response. It returns true if granted, false if denied or timeout.
func EnsureBluetoothConnectPermission(ctx context.Context) (bool, error) {
	return ensureBluetoothConnectPermission(ctx)
}
