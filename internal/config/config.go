// Package config owns persistent application preferences, not printer protocol settings.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/timboli111/PrintCat/internal/printer"
)

// SavedPrinter is a printer the user has explicitly saved to persistent
// configuration. It bundles the underlying printer definition with the
// per-printer paper dimensions chosen by the user.
type SavedPrinter struct {
	Printer       printer.Printer `json:"printer"`
	PaperWidthMm  int             `json:"paperWidthMm,omitempty"`
	PaperHeightMm int             `json:"paperHeightMm,omitempty"`
}

// Config is the versioned root of PrintCat user configuration.
//
// Version 2 introduced multi-printer support: SavedPrinters holds an
// ordered list of user-saved printers, and ActivePrinterID points at the
// entry that external print jobs (Android Print Framework) should use.
type Config struct {
	Version          int            `json:"version"`
	SelectedPrinter  string         `json:"selectedPrinter,omitempty"`
	DefaultPaperName string         `json:"defaultPaperName,omitempty"`
	SavedPrinters    []SavedPrinter `json:"savedPrinters,omitempty"`
	ActivePrinterID  string         `json:"activePrinterId,omitempty"`
}

// Default returns safe application defaults for a new installation.
func Default() Config {
	return Config{
		Version:          2,
		DefaultPaperName: "80 mm thermal",
	}
}

// Load reads Config from path, migrating legacy single-printer configs into
// the multi-printer format on the fly. If the file does not exist, Default
// is returned with a nil error so first-run code can call Load
// unconditionally.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Default(), nil
		}
		return Default(), fmt.Errorf("read config: %w", err)
	}
	cfg := Default()
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Default(), fmt.Errorf("parse config: %w", err)
	}
	return cfg, nil
}

// Save writes Config to path, creating parent directories as needed. It
// always writes the current (version 2) structure; legacy fields are not
// serialized.
func Save(path string, cfg Config) error {
	if cfg.Version < 2 {
		cfg.Version = 2
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create config dir: %w", err)
		}
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

// UnmarshalJSON handles migration from the legacy single-printer config
// format (version 1) into the multi-printer format (version 2). It also
// tolerates the intermediate shape used during an earlier Step 1C iteration
// where paperWidthMm/paperHeightMm lived at the top level.
func (c *Config) UnmarshalJSON(data []byte) error {
	type alias struct {
		Version          int            `json:"version"`
		SelectedPrinter  string         `json:"selectedPrinter,omitempty"`
		DefaultPaperName string         `json:"defaultPaperName,omitempty"`
		SavedPrinters    []SavedPrinter `json:"savedPrinters,omitempty"`
		ActivePrinterID  string         `json:"activePrinterId,omitempty"`

		// Legacy v1 fields:
		ActivePrinter *printer.Printer `json:"activePrinter,omitempty"`
		PaperWidthMm  int              `json:"paperWidthMm,omitempty"`
		PaperHeightMm int              `json:"paperHeightMm,omitempty"`
	}
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}

	c.Version = a.Version
	c.SelectedPrinter = a.SelectedPrinter
	c.DefaultPaperName = a.DefaultPaperName
	c.SavedPrinters = a.SavedPrinters
	c.ActivePrinterID = a.ActivePrinterID

	// Migration: fold the legacy single activePrinter into the saved list.
	if a.ActivePrinter != nil && len(c.SavedPrinters) == 0 {
		c.SavedPrinters = []SavedPrinter{{
			Printer:       *a.ActivePrinter,
			PaperWidthMm:  a.PaperWidthMm,
			PaperHeightMm: a.PaperHeightMm,
		}}
		if c.ActivePrinterID == "" {
			c.ActivePrinterID = a.ActivePrinter.ID
		}
		c.Version = 2
	}
	if c.Version < 2 && len(c.SavedPrinters) > 0 {
		c.Version = 2
	}
	return nil
}

// FindSavedPrinter returns a pointer to the saved printer with the given
// ID (matched against printer.Printer.ID), or nil if not found.
func (c *Config) FindSavedPrinter(id string) *SavedPrinter {
	for i := range c.SavedPrinters {
		if c.SavedPrinters[i].Printer.ID == id {
			return &c.SavedPrinters[i]
		}
	}
	return nil
}

// ActiveSavedPrinter returns the saved printer referenced by
// ActivePrinterID, or nil if there is no active printer.
func (c *Config) ActiveSavedPrinter() *SavedPrinter {
	if c.ActivePrinterID == "" {
		return nil
	}
	return c.FindSavedPrinter(c.ActivePrinterID)
}

// UpsertSavedPrinter adds a new saved printer or replaces the existing one
// with the same Printer.ID. Returns true if the printer was newly added.
func (c *Config) UpsertSavedPrinter(sp SavedPrinter) bool {
	for i := range c.SavedPrinters {
		if c.SavedPrinters[i].Printer.ID == sp.Printer.ID {
			c.SavedPrinters[i] = sp
			return false
		}
	}
	c.SavedPrinters = append(c.SavedPrinters, sp)
	return true
}

// RemoveSavedPrinter removes the saved printer with the given ID. If the
// removed printer was active, ActivePrinterID is cleared. Returns true if a
// printer was removed.
func (c *Config) RemoveSavedPrinter(id string) bool {
	for i := range c.SavedPrinters {
		if c.SavedPrinters[i].Printer.ID == id {
			c.SavedPrinters = append(c.SavedPrinters[:i], c.SavedPrinters[i+1:]...)
			if c.ActivePrinterID == id {
				c.ActivePrinterID = ""
			}
			return true
		}
	}
	return false
}

// SetActivePrinter marks the saved printer with the given ID as active.
// Returns false if no saved printer with that ID exists.
func (c *Config) SetActivePrinter(id string) bool {
	if c.FindSavedPrinter(id) == nil {
		return false
	}
	c.ActivePrinterID = id
	return true
}
