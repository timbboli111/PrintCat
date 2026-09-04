package platform

import "context"

func GetAndroidAPIVersion() int {
	return getAndroidAPIVersion()
}

func CheckBluetoothConnectPermission(ctx context.Context) (bool, error) {
	return checkBluetoothConnectPermission(ctx)
}

func EnsureBluetoothConnectPermission(ctx context.Context) (bool, error) {
	return ensureBluetoothConnectPermission(ctx)
}

func CheckBluetoothScanPermission(ctx context.Context) (bool, error) {
	return checkBluetoothScanPermission(ctx)
}

func EnsureBluetoothScanPermission(ctx context.Context) (bool, error) {
	return ensureBluetoothScanPermission(ctx)
}
