package platform

import (
	"context"
	"fmt"

	"github.com/timboli111/PrintCat/internal/printer"
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
		if len(value) < 4 || value[:3] != "COM" {
			continue
		}
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

	return devices, nil
}

func (d *discoveryWindows) RequestAccess(ctx context.Context, device Device) error {
	return nil
}

func newDiscovery() Integration {
	return &discoveryWindows{}
}
