package fyneapp

import (
	"testing"

	"fyne.io/fyne/v2/test"
)

func TestNewWindow(t *testing.T) {
	application := test.NewApp()
	defer application.Quit()
	window := NewWindow(application)
	if window.Title() != "PrintCat" {
		t.Fatalf("window title = %q, want PrintCat", window.Title())
	}
}
