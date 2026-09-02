//go:build windows

package platform

import (
	"context"
	"fmt"
	"unsafe"

	"github.com/timboli111/PrintCat/internal/printer"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

type discoveryWindows struct{}

func (d *discoveryWindows) Discover(ctx context.Context, kind printer.TransportKind) ([]Device, error) {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `HARDWARE\DEVICEMAP\SERIALCOMM`, registry.QUERY_VALUE)
	if err != nil {
		if err == registry.ErrNotExist {
			return []Device{}, nil
		}
		return nil, fmt.Errorf("failed to open SERIALCOMM registry key: %w", err)
	}
	defer key.Close()

	values, err := key.ReadValueNames(0)
	if err != nil {
		return nil, fmt.Errorf("failed to read SERIALCOMM values: %w", err)
	}

	deviceMap := make(map[string]struct{})
	devices := []Device{}

	for _, name := range values {
		value, _, err := key.GetStringValue(name)
		if err != nil {
			continue
		}
		if value == "" {
			continue
		}
		// Windows COM port naming: COMx (case insensitive)
		// Filter value to only COM ports
		if len(value) < 4 || value[:3] != "COM" {
			continue
		}
		// Skip duplicate endpoints
		if _, exists := deviceMap[value]; exists {
			continue
		}
		deviceMap[value] = struct{}{}

		devices = append(devices, Device{
			ID:       value,
			Name:     value,
			Kind:     printer.Serial,
			Endpoint: value,
			Profile: printer.PrinterProfile{
				SupportedProtocols:  []printer.Protocol{},
				SupportedTransports: []printer.TransportKind{printer.Serial},
			},
		})
	}

	// If no COM ports found, return empty slice (not error)
	return devices, nil
}

func (d *discoveryWindows) RequestAccess(ctx context.Context, device Device) error {
	// For Windows MVP, no special access request is needed.
	// The COM port will be opened by the transport when printing.
	return nil
}

func newDiscovery() Integration {
	return &discoveryWindows{}
}
