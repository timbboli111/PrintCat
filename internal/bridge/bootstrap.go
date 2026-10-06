// Package bridge: process-wide bootstrap helper.
//
// Bootstrap initializes the shared printer engine and bridge state that
// both the Fyne UI and the Android PrintService JNI path depend on. It is
// safe to call multiple times; after the first successful call, subsequent
// calls reuse the existing state and only top up the active printer from
// config if none has been set yet.

package bridge

import (
	"log"

	"github.com/timboli111/PrintCat/internal/config"
	"github.com/timboli111/PrintCat/internal/printer"
	"github.com/timboli111/PrintCat/internal/printer/cpcl"
	"github.com/timboli111/PrintCat/internal/printer/epl"
	"github.com/timboli111/PrintCat/internal/printer/escpos"
	"github.com/timboli111/PrintCat/internal/printer/starprnt"
	"github.com/timboli111/PrintCat/internal/printer/transport/bluetooth"
	"github.com/timboli111/PrintCat/internal/printer/transport/tcp"
	"github.com/timboli111/PrintCat/internal/printer/tspl"
	"github.com/timboli111/PrintCat/internal/printer/zpl"
	"github.com/timboli111/PrintCat/internal/render/basic"
)

// Bootstrap returns the process-wide printer.Service, initializing it if
// necessary. Registration uses the standard backend and transport set that
// PrintCat ships with.
//
// If configPath is non-empty, and the state has no active printer yet, the
// config file at that path is loaded and its active saved printer (if any)
// is installed on the state.
//
// Returns nil only if the printer engine cannot be constructed.
func Bootstrap(configPath string) *printer.Service {
	if existing := GetGlobal(); existing != nil {
		if configPath != "" && existing.ActivePrinter() == nil {
			loadActivePrinterFromConfig(existing, configPath)
		}
		return existing.service
	}

	svc := printer.NewService()
	if err := registerStandardBackends(svc); err != nil {
		log.Printf("[bridge] bootstrap: register backends failed: %v", err)
		return nil
	}
	if err := registerStandardTransports(svc); err != nil {
		log.Printf("[bridge] bootstrap: register transports failed: %v", err)
		return nil
	}

	state := NewState(svc, &basic.Renderer{})
	SetGlobal(state)
	log.Printf("[bridge] bootstrap: printer service initialized")

	if configPath != "" {
		loadActivePrinterFromConfig(state, configPath)
	}
	return svc
}

func registerStandardBackends(svc *printer.Service) error {
	if err := svc.RegisterBackend(&escpos.Encoder{}); err != nil {
		return err
	}
	if err := svc.RegisterBackend(&tspl.Encoder{}); err != nil {
		return err
	}
	if err := svc.RegisterBackend(&zpl.Encoder{}); err != nil {
		return err
	}
	if err := svc.RegisterBackend(&cpcl.Encoder{}); err != nil {
		return err
	}
	if err := svc.RegisterBackend(&epl.Encoder{}); err != nil {
		return err
	}
	if err := svc.RegisterBackend(&starprnt.Encoder{}); err != nil {
		return err
	}
	return nil
}

func registerStandardTransports(svc *printer.Service) error {
	if err := svc.RegisterTransport(&tcp.TCPTransport{}); err != nil {
		return err
	}
	if err := svc.RegisterTransport(&bluetooth.BluetoothTransport{}); err != nil {
		return err
	}
	return nil
}

func loadActivePrinterFromConfig(state *State, configPath string) {
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Printf("[bridge] config load %s: %v", configPath, err)
		return
	}
	active := cfg.ActiveSavedPrinter()
	if active == nil {
		log.Printf("[bridge] config %s has no active printer", configPath)
		return
	}
	p := active.Printer
	state.SetActivePrinterWithPaper(&p, active.PaperWidthMm, active.PaperHeightMm)
	log.Printf("[bridge] active printer loaded from %s: id=%q name=%q protocol=%s transport=%s endpoint=%s paper=%dx%dmm",
		configPath,
		p.ID,
		p.Name,
		p.Connection.Protocol,
		p.Connection.Transport,
		p.Connection.Endpoint,
		active.PaperWidthMm,
		active.PaperHeightMm)
}
