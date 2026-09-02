//go:build !windows && !android

package platform

import (
	"context"

	"github.com/timboli111/PrintCat/internal/printer"
)

// discoveryStub implements Integration for platforms without native discovery.
type discoveryStub struct{}

func (d *discoveryStub) Discover(ctx context.Context, kind printer.TransportKind) ([]Device, error) {
	return nil, nil
}

func (d *discoveryStub) RequestAccess(ctx context.Context, device Device) error {
	return nil
}

func newDiscovery() Integration {
	return &discoveryStub{}
}
