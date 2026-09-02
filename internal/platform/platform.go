package platform

import (
	"context"

	"github.com/timboli111/PrintCat/internal/printer"
)

type Device struct {
	ID      string
	Name    string
	Kind    printer.TransportKind
	Profile printer.PrinterProfile
}

type Integration interface {
	Discover(ctx context.Context, kind printer.TransportKind) ([]Device, error)
	RequestAccess(ctx context.Context, device Device) error
}
