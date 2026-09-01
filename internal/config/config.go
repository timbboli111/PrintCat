// Package config owns persistent application preferences, not printer protocol settings.
package config

// Config is the versioned root of PrintCat user configuration.
type Config struct {
	Version          int    `json:"version"`
	SelectedPrinter  string `json:"selectedPrinter,omitempty"`
	DefaultPaperName string `json:"defaultPaperName,omitempty"`
}

// Default returns safe application defaults for a new installation.
func Default() Config {
	return Config{Version: 1, DefaultPaperName: "80 mm thermal"}
}
