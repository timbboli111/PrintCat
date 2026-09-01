// Package platform contains interfaces implemented by Windows and Android adapters.
package platform

import (
	"context"

	"github.com/printcat/printcat/internal/printer"
)

// Device is a discovered platform endpoint; capabilities are resolved separately.
type Device struct {
	ID   string
	Name string
	Kind printer.TransportKind
}

// Integration isolates platform discovery, permissions, and native handoff from the core.
type Integration interface {
	Discover(ctx context.Context, kind printer.TransportKind) ([]Device, error)
	RequestAccess(ctx context.Context, device Device) error
}
