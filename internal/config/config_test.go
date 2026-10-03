package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/timboli111/PrintCat/internal/printer"
)

func TestLoadReturnsDefaultWhenMissing(t *testing.T) {
	dir := t.TempDir()
	cfg, err := Load(filepath.Join(dir, "missing.json"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Version != 1 {
		t.Errorf("expected default version 1, got %d", cfg.Version)
	}
	if cfg.ActivePrinter != nil {
		t.Errorf("expected nil active printer, got %+v", cfg.ActivePrinter)
	}
}

func TestSaveLoadRoundtripActivePrinter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	want := printer.Printer{
		ID:   "test-printer",
		Name: "RPP02N",
		Connection: printer.Connection{
			Protocol:  printer.ZPL,
			Transport: printer.BluetoothClassic,
			Endpoint:  "DC:0D:51:9F:73:CE",
		},
		Profile: printer.PrinterProfile{
			DPI:        203,
			MediaWidth: 80000,
		},
	}

	cfg := Default()
	cfg.ActivePrinter = &want
	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.ActivePrinter == nil {
		t.Fatal("ActivePrinter is nil after roundtrip")
	}
	if got.ActivePrinter.ID != want.ID {
		t.Errorf("ID: got %q, want %q", got.ActivePrinter.ID, want.ID)
	}
	if got.ActivePrinter.Connection.Endpoint != want.Connection.Endpoint {
		t.Errorf("Endpoint: got %q, want %q",
			got.ActivePrinter.Connection.Endpoint, want.Connection.Endpoint)
	}
	if got.ActivePrinter.Profile.DPI != want.Profile.DPI {
		t.Errorf("DPI: got %d, want %d", got.ActivePrinter.Profile.DPI, want.Profile.DPI)
	}
}

func TestSaveCreatesParentDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "config.json")
	if err := Save(path, Default()); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file to exist: %v", err)
	}
}
