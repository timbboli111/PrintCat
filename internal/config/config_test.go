package config

import (
	"encoding/json"
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
	if cfg.Version != 2 {
		t.Errorf("expected default version 2, got %d", cfg.Version)
	}
	if cfg.ActiveSavedPrinter() != nil {
		t.Errorf("expected nil active printer, got %+v", cfg.ActiveSavedPrinter())
	}
}

func TestSaveLoadRoundtripMultiPrinter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	p1 := SavedPrinter{
		Printer: printer.Printer{
			ID:   "DC:0D:51:9F:73:CE",
			Name: "RPP02N",
			Connection: printer.Connection{
				Protocol:  printer.ESCPOS,
				Transport: printer.BluetoothClassic,
				Endpoint:  "DC:0D:51:9F:73:CE",
			},
			Profile: printer.PrinterProfile{DPI: 203},
		},
		PaperWidthMm:  58,
		PaperHeightMm: 120,
	}
	p2 := SavedPrinter{
		Printer: printer.Printer{
			ID:   "AA:BB:CC:DD:EE:FF",
			Name: "Printer B",
			Connection: printer.Connection{
				Protocol:  printer.TSPL,
				Transport: printer.BluetoothClassic,
				Endpoint:  "AA:BB:CC:DD:EE:FF",
			},
			Profile: printer.PrinterProfile{DPI: 203},
		},
		PaperWidthMm:  80,
		PaperHeightMm: 200,
	}

	cfg := Default()
	cfg.SavedPrinters = []SavedPrinter{p1, p2}
	cfg.ActivePrinterID = p2.Printer.ID

	if err := Save(path, cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got.SavedPrinters) != 2 {
		t.Fatalf("SavedPrinters count = %d, want 2", len(got.SavedPrinters))
	}
	if got.ActivePrinterID != p2.Printer.ID {
		t.Errorf("ActivePrinterID = %q, want %q", got.ActivePrinterID, p2.Printer.ID)
	}

	active := got.ActiveSavedPrinter()
	if active == nil {
		t.Fatal("ActiveSavedPrinter returned nil")
	}
	if active.Printer.Name != "Printer B" {
		t.Errorf("active name = %q, want %q", active.Printer.Name, "Printer B")
	}
	if active.PaperWidthMm != 80 || active.PaperHeightMm != 200 {
		t.Errorf("active paper = %dx%d, want 80x200",
			active.PaperWidthMm, active.PaperHeightMm)
	}

	first := got.FindSavedPrinter(p1.Printer.ID)
	if first == nil {
		t.Fatal("RPP02N not found after roundtrip")
	}
	if first.PaperWidthMm != 58 || first.PaperHeightMm != 120 {
		t.Errorf("RPP02N paper = %dx%d, want 58x120",
			first.PaperWidthMm, first.PaperHeightMm)
	}
}

func TestMigrationFromLegacySinglePrinter(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	legacy := map[string]interface{}{
		"version":          1,
		"defaultPaperName": "80 mm thermal",
		"paperWidthMm":     58,
		"paperHeightMm":    120,
		"activePrinter": map[string]interface{}{
			"id":   "DC:0D:51:9F:73:CE",
			"name": "RPP02N",
			"connection": map[string]interface{}{
				"Protocol":  "esc-pos",
				"Transport": "bluetooth-classic",
				"Endpoint":  "DC:0D:51:9F:73:CE",
			},
			"profile": map[string]interface{}{"dpi": 203},
		},
	}
	raw, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Version != 2 {
		t.Errorf("Version = %d, want 2 (migrated)", cfg.Version)
	}
	if len(cfg.SavedPrinters) != 1 {
		t.Fatalf("SavedPrinters = %d, want 1", len(cfg.SavedPrinters))
	}
	sp := cfg.SavedPrinters[0]
	if sp.Printer.ID != "DC:0D:51:9F:73:CE" {
		t.Errorf("migrated ID = %q", sp.Printer.ID)
	}
	if sp.PaperWidthMm != 58 || sp.PaperHeightMm != 120 {
		t.Errorf("migrated paper = %dx%d, want 58x120",
			sp.PaperWidthMm, sp.PaperHeightMm)
	}
	if cfg.ActivePrinterID != "DC:0D:51:9F:73:CE" {
		t.Errorf("ActivePrinterID = %q, want legacy ID", cfg.ActivePrinterID)
	}

	// Save the migrated config and verify it writes the new format.
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	reloaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(reloaded.SavedPrinters) != 1 {
		t.Errorf("after re-save, SavedPrinters = %d, want 1", len(reloaded.SavedPrinters))
	}
}

func TestUpsertSavedPrinterAddsAndUpdates(t *testing.T) {
	cfg := Default()

	p := SavedPrinter{
		Printer: printer.Printer{
			ID:   "AA:BB:CC:DD:EE:FF",
			Name: "Printer B",
			Connection: printer.Connection{
				Protocol:  printer.TSPL,
				Transport: printer.BluetoothClassic,
			},
			Profile: printer.PrinterProfile{DPI: 203},
		},
		PaperWidthMm:  80,
		PaperHeightMm: 200,
	}

	if added := cfg.UpsertSavedPrinter(p); !added {
		t.Error("UpsertSavedPrinter returned false for new printer")
	}
	if len(cfg.SavedPrinters) != 1 {
		t.Fatalf("SavedPrinters = %d, want 1", len(cfg.SavedPrinters))
	}

	p.Printer.Profile.DPI = 300
	p.PaperWidthMm = 58
	p.PaperHeightMm = 120
	if added := cfg.UpsertSavedPrinter(p); added {
		t.Error("UpsertSavedPrinter returned true for existing printer")
	}
	if len(cfg.SavedPrinters) != 1 {
		t.Fatalf("SavedPrinters = %d, want 1 after update", len(cfg.SavedPrinters))
	}
	if cfg.SavedPrinters[0].Printer.Profile.DPI != 300 {
		t.Errorf("DPI after update = %d, want 300", cfg.SavedPrinters[0].Printer.Profile.DPI)
	}
}

func TestRemoveSavedPrinterClearsActive(t *testing.T) {
	cfg := Default()
	cfg.SavedPrinters = []SavedPrinter{
		{Printer: printer.Printer{ID: "A"}, PaperWidthMm: 80, PaperHeightMm: 200},
		{Printer: printer.Printer{ID: "B"}, PaperWidthMm: 58, PaperHeightMm: 120},
	}
	cfg.ActivePrinterID = "A"

	if !cfg.RemoveSavedPrinter("A") {
		t.Fatal("RemoveSavedPrinter returned false")
	}
	if len(cfg.SavedPrinters) != 1 {
		t.Fatalf("SavedPrinters = %d, want 1", len(cfg.SavedPrinters))
	}
	if cfg.SavedPrinters[0].Printer.ID != "B" {
		t.Errorf("remaining printer ID = %q, want B", cfg.SavedPrinters[0].Printer.ID)
	}
	if cfg.ActivePrinterID != "" {
		t.Errorf("ActivePrinterID = %q, want empty after removing active", cfg.ActivePrinterID)
	}
}

func TestSetActivePrinterRejectsUnknownID(t *testing.T) {
	cfg := Default()
	cfg.SavedPrinters = []SavedPrinter{
		{Printer: printer.Printer{ID: "A"}},
	}
	if cfg.SetActivePrinter("nonexistent") {
		t.Error("SetActivePrinter accepted unknown ID")
	}
	if cfg.ActivePrinterID != "" {
		t.Errorf("ActivePrinterID changed to %q", cfg.ActivePrinterID)
	}
	if !cfg.SetActivePrinter("A") {
		t.Error("SetActivePrinter rejected known ID")
	}
	if cfg.ActivePrinterID != "A" {
		t.Errorf("ActivePrinterID = %q, want A", cfg.ActivePrinterID)
	}
}
