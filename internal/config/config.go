// Package config owns persistent application preferences, not printer protocol settings.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/timboli111/PrintCat/internal/printer"
)

// Config is the versioned root of PrintCat user configuration.
type Config struct {
	Version          int    `json:"version"`
	SelectedPrinter  string `json:"selectedPrinter,omitempty"`
	DefaultPaperName string `json:"defaultPaperName,omitempty"`
	// ActivePrinter is the printer that external print jobs (for example
	// from the Android Print Framework) are sent to. It is written when
	// the user configures a printer from the Fyne UI and read on startup
	// so that the PrintService can route jobs without the UI being open.
	ActivePrinter *printer.Printer `json:"activePrinter,omitempty"`
}

// Default returns safe application defaults for a new installation.
func Default() Config {
	return Config{Version: 1, DefaultPaperName: "80 mm thermal"}
}

// Load reads Config from path. If the file does not exist, Default is
// returned with a nil error so first-run code can call Load unconditionally.
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

// Save writes Config to path, creating parent directories as needed.
func Save(path string, cfg Config) error {
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
