// Package fyneapp is the Fyne presentation layer. It contains no printer protocol logic.
package fyneapp

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const appID = "com.printcat.app"

// New creates the standalone Fyne application used on Windows and Android.
func New() fyne.App { return app.NewWithID(appID) }

// NewWindow builds the initial application shell without initializing printer hardware.
func NewWindow(application fyne.App) fyne.Window {
	window := application.NewWindow("PrintCat")
	window.Resize(fyne.NewSize(1100, 700))

	header := container.NewHBox(
		widget.NewIcon(theme.PrintIcon()),
		widget.NewLabelWithStyle("PrintCat", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel("Thermal print workspace"),
	)

	workspace := container.NewCenter(widget.NewLabel("Thermal canvas coming in a future phase"))
	printerPanel := container.NewVBox(
		widget.NewLabelWithStyle("Printer", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewSelect([]string{"No printer selected"}, func(string) {}),
		widget.NewButton("Print", func() {}),
		widget.NewButton("Settings", func() {}),
	)
	navigation := container.NewVBox(
		widget.NewLabelWithStyle("Workspace", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewButton("Document", func() {}),
		widget.NewButton("Printers", func() {}),
	)

	window.SetContent(container.NewBorder(header, nil, navigation, printerPanel, workspace))
	return window
}
