package fyneapp

import (
	"testing"

	"fyne.io/fyne/v2/test"

	"github.com/timboli111/PrintCat/internal/document"
	"github.com/timboli111/PrintCat/internal/printer"
)

func TestNewWindow(t *testing.T) {
	application := test.NewApp()
	defer application.Quit()
	window := NewWindow(application)
	if window.Title() != "PrintCat" {
		t.Fatalf("window title = %q, want PrintCat", window.Title())
	}
}

func TestParseSmartTrimMarginUm(t *testing.T) {
	tests := []struct {
		value string
		want  document.Unit
	}{
		{value: "0", want: 0},         // zero selects the encoder default
		{value: "0.0004", want: 1},    // positive sub-micrometre input stays non-zero
		{value: "0.0005", want: 1},    // half-micrometre rounds up
		{value: "0.0006", want: 1},    // millimetres to micrometres
		{value: "2.5", want: 2500},    // millimetres to micrometres
		{value: "100", want: 100_000}, // inclusive maximum: 100 mm
	}
	for _, tt := range tests {
		got, err := parseSmartTrimMarginUm(tt.value)
		if err != nil {
			t.Errorf("parseSmartTrimMarginUm(%q): %v", tt.value, err)
			continue
		}
		if got != tt.want {
			t.Errorf("parseSmartTrimMarginUm(%q) = %d, want %d", tt.value, got, tt.want)
		}
	}

	for _, value := range []string{"", "NaN", "Inf", "-0.1", "100.0001", "1000001", "1e309", "not a number"} {
		if _, err := parseSmartTrimMarginUm(value); err == nil {
			t.Errorf("parseSmartTrimMarginUm(%q) succeeded, want validation error", value)
		}
	}
}

func TestApplySettingsSmartTrimMarginRepairsInvalidProfileRegardlessOfProtocolAndToggle(t *testing.T) {
	tests := []struct {
		name       string
		protocol   printer.Protocol
		enabled    bool
		marginText string
		wantMargin document.Unit
	}{
		{
			name:       "non-ESC/POS and disabled accepts zero",
			protocol:   printer.ZPL,
			enabled:    false,
			marginText: "0",
			wantMargin: 0,
		},
		{
			name:       "non-ESC/POS and enabled repairs margin without changing toggle",
			protocol:   printer.ZPL,
			enabled:    true,
			marginText: "25",
			wantMargin: 25_000,
		},
		{
			name:       "ESC/POS and disabled remains unchanged",
			protocol:   printer.ESCPOS,
			enabled:    false,
			marginText: "100",
			wantMargin: 100_000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile := printer.PrinterProfile{
				DPI:                     203,
				Density:                 160,
				SmartTrimEnabled:        tt.enabled,
				SmartTrimBottomMarginUm: printer.MaxSmartTrimBottomMarginUm + 1,
			}
			needsRepair, err := applySettingsSmartTrimMargin(&profile, tt.enabled, tt.protocol, tt.marginText)
			if err != nil {
				t.Fatalf("applySettingsSmartTrimMargin returned error: %v", err)
			}
			if !needsRepair {
				t.Fatal("invalid stored margin was not identified for repair")
			}
			if profile.SmartTrimBottomMarginUm != tt.wantMargin {
				t.Errorf("margin = %d µm, want %d µm", profile.SmartTrimBottomMarginUm, tt.wantMargin)
			}
			if profile.SmartTrimEnabled != tt.enabled || profile.DPI != 203 || profile.Density != 160 {
				t.Errorf("repair changed unrelated profile settings: %+v", profile)
			}
		})
	}
}

func TestApplySettingsSmartTrimMarginRejectsInvalidReplacement(t *testing.T) {
	for _, value := range []string{"-0.001", "100.001", "not a number"} {
		t.Run(value, func(t *testing.T) {
			profile := printer.PrinterProfile{
				DPI:                     203,
				SmartTrimBottomMarginUm: printer.MaxSmartTrimBottomMarginUm + 1,
			}
			originalMargin := profile.SmartTrimBottomMarginUm
			needsRepair, err := applySettingsSmartTrimMargin(&profile, false, printer.ZPL, value)
			if err == nil {
				t.Fatal("invalid replacement was accepted")
			}
			if !needsRepair {
				t.Fatal("invalid stored margin was not identified for repair")
			}
			if profile.SmartTrimBottomMarginUm != originalMargin {
				t.Fatalf("failed correction changed stored margin to %d µm", profile.SmartTrimBottomMarginUm)
			}
		})
	}
}

func TestApplySettingsSmartTrimMarginPreservesNormalValidProfileBehavior(t *testing.T) {
	profile := printer.PrinterProfile{DPI: 203, SmartTrimBottomMarginUm: 2500}
	needsRepair, err := applySettingsSmartTrimMargin(&profile, false, printer.ZPL, "not a number")
	if err != nil {
		t.Fatalf("valid disabled non-ESC/POS profile parsed an unused margin: %v", err)
	}
	if needsRepair {
		t.Fatal("valid stored margin was marked for repair")
	}
	if profile.SmartTrimBottomMarginUm != 2500 {
		t.Fatalf("normal valid profile margin changed to %d µm", profile.SmartTrimBottomMarginUm)
	}

	activeProfile := printer.PrinterProfile{
		DPI:                     203,
		SmartTrimEnabled:        true,
		SmartTrimBottomMarginUm: 2500,
	}
	needsRepair, err = applySettingsSmartTrimMargin(&activeProfile, true, printer.ESCPOS, "4.25")
	if err != nil {
		t.Fatalf("valid enabled ESC/POS profile could not update its margin: %v", err)
	}
	if needsRepair || activeProfile.SmartTrimBottomMarginUm != 4250 {
		t.Fatalf("normal enabled ESC/POS update = repair:%t margin:%d µm, want repair:false margin:4250 µm", needsRepair, activeProfile.SmartTrimBottomMarginUm)
	}
}
