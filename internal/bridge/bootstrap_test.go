package bridge

import (
	"path/filepath"
	"testing"

	"github.com/timboli111/PrintCat/internal/config"
	"github.com/timboli111/PrintCat/internal/printer"
)

func TestBootstrapCreatesState(t *testing.T) {
	SetGlobal(nil)
	t.Cleanup(func() { SetGlobal(nil) })

	svc := Bootstrap("")
	if svc == nil {
		t.Fatal("Bootstrap returned nil")
	}
	if GetGlobal() == nil {
		t.Fatal("global state is nil after Bootstrap")
	}
}

func TestBootstrapIsIdempotent(t *testing.T) {
	SetGlobal(nil)
	t.Cleanup(func() { SetGlobal(nil) })

	svc1 := Bootstrap("")
	svc2 := Bootstrap("")
	if svc1 == nil || svc2 == nil {
		t.Fatal("Bootstrap returned nil")
	}
	if svc1 != svc2 {
		t.Error("Bootstrap returned different services on second call")
	}
}

func TestBootstrapLoadsActivePrinterFromConfig(t *testing.T) {
	SetGlobal(nil)
	t.Cleanup(func() { SetGlobal(nil) })

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")

	want := printer.Printer{
		ID:   "printer-1",
		Name: "Test Printer",
		Connection: printer.Connection{
			Protocol:  printer.ZPL,
			Transport: printer.TCP,
			Endpoint:  "127.0.0.1:9100",
		},
		Profile: printer.PrinterProfile{DPI: 203},
	}

	cfg := config.Default()
	cfg.SavedPrinters = []config.SavedPrinter{{
		Printer:       want,
		PaperWidthMm:  80,
		PaperHeightMm: 200,
	}}
	cfg.ActivePrinterID = want.ID

	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	svc := Bootstrap(cfgPath)
	if svc == nil {
		t.Fatal("Bootstrap returned nil")
	}
	state := GetGlobal()
	if state == nil {
		t.Fatal("global state is nil after Bootstrap")
	}
	got := state.ActivePrinter()
	if got == nil {
		t.Fatal("ActivePrinter is nil after Bootstrap with config")
	}
	if got.ID != want.ID {
		t.Errorf("ID = %q, want %q", got.ID, want.ID)
	}
	if got.Connection.Endpoint != want.Connection.Endpoint {
		t.Errorf("Endpoint = %q, want %q",
			got.Connection.Endpoint, want.Connection.Endpoint)
	}
	if got.Profile.DPI != want.Profile.DPI {
		t.Errorf("DPI = %d, want %d", got.Profile.DPI, want.Profile.DPI)
	}
}

func TestBootstrapHandlesMissingConfigFile(t *testing.T) {
	SetGlobal(nil)
	t.Cleanup(func() { SetGlobal(nil) })

	svc := Bootstrap(filepath.Join(t.TempDir(), "does-not-exist.json"))
	if svc == nil {
		t.Fatal("Bootstrap returned nil for missing config")
	}
	if GetGlobal().ActivePrinter() != nil {
		t.Error("ActivePrinter should be nil when config missing")
	}
}

func TestBootstrapSecondCallLoadsConfigForExistingState(t *testing.T) {
	SetGlobal(nil)
	t.Cleanup(func() { SetGlobal(nil) })

	// First call: no config → no active printer.
	if svc := Bootstrap(""); svc == nil {
		t.Fatal("Bootstrap returned nil")
	}
	if GetGlobal().ActivePrinter() != nil {
		t.Fatal("ActivePrinter should be nil after first bootstrap without config")
	}

	// Second call with a config: should load the active printer onto the
	// existing state (the JNI scenario: UI ran first with no config,
	// PrintService later supplies the config path).
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")
	want := printer.Printer{
		ID:         "printer-2",
		Name:       "Later Printer",
		Connection: printer.Connection{Protocol: printer.TSPL, Transport: printer.TCP},
		Profile:    printer.PrinterProfile{DPI: 300},
	}
	cfg := config.Default()
	cfg.SavedPrinters = []config.SavedPrinter{{
		Printer:       want,
		PaperWidthMm:  58,
		PaperHeightMm: 120,
	}}
	cfg.ActivePrinterID = want.ID

	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	svc := Bootstrap(cfgPath)
	if svc == nil {
		t.Fatal("Bootstrap returned nil on second call")
	}
	got := GetGlobal().ActivePrinter()
	if got == nil {
		t.Fatal("ActivePrinter is nil after second Bootstrap with config")
	}
	if got.ID != want.ID {
		t.Errorf("ID = %q, want %q", got.ID, want.ID)
	}
}

func TestBootstrapIgnoresConfigWithoutActivePrinter(t *testing.T) {
	SetGlobal(nil)
	t.Cleanup(func() { SetGlobal(nil) })

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")

	// Config with saved printers but no active selection.
	cfg := config.Default()
	cfg.SavedPrinters = []config.SavedPrinter{{
		Printer: printer.Printer{
			ID:         "id-1",
			Name:       "Saved But Not Active",
			Connection: printer.Connection{Protocol: printer.ZPL, Transport: printer.TCP},
			Profile:    printer.PrinterProfile{DPI: 203},
		},
	}}
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	svc := Bootstrap(cfgPath)
	if svc == nil {
		t.Fatal("Bootstrap returned nil")
	}
	if GetGlobal().ActivePrinter() != nil {
		t.Error("ActivePrinter should be nil when ActivePrinterID is empty")
	}
}
