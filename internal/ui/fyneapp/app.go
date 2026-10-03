package fyneapp

import (
	"context"
	"fmt"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/timboli111/PrintCat/internal/bridge"
	"github.com/timboli111/PrintCat/internal/config"
	"github.com/timboli111/PrintCat/internal/document"
	"github.com/timboli111/PrintCat/internal/editor"
	"github.com/timboli111/PrintCat/internal/platform"
	"github.com/timboli111/PrintCat/internal/printer"
	"github.com/timboli111/PrintCat/internal/render/basic"
)

const appID = "com.printcat.app"

func New() fyne.App {
	application := app.NewWithID(appID)
	if runtime.GOOS == "android" {
		application.Settings().SetTheme(&androidTheme{})
	}
	return application
}

func discoverBluetoothPrinters(ctx context.Context, window fyne.Window) ([]platform.Device, error) {
	if runtime.GOOS != "android" {
		return nil, nil
	}

	if platform.GetAndroidAPIVersion() >= 31 {
		connectGranted, err := platform.EnsureBluetoothConnectPermission(ctx)
		if err != nil {
			return nil, fmt.Errorf("bluetooth connect permission error: %w", err)
		}
		if !connectGranted {
			return nil, fmt.Errorf("bluetooth connect permission denied")
		}
	}

	scanGranted, err := platform.EnsureBluetoothScanPermission(ctx)
	if err != nil {
		return nil, fmt.Errorf("bluetooth scan permission error: %w", err)
	}
	if !scanGranted {
		return nil, fmt.Errorf("bluetooth scan permission denied")
	}

	integration := platform.GetIntegration()
	discovered, err := integration.Discover(ctx, printer.BluetoothClassic)
	if err != nil {
		return nil, fmt.Errorf("discovery failed: %w", err)
	}
	return discovered, nil
}

func savedPrinterDisplay(sp config.SavedPrinter) string {
	if sp.Printer.Name != "" {
		return fmt.Sprintf("%s (%s)", sp.Printer.Name, sp.Printer.Connection.Endpoint)
	}
	return fmt.Sprintf("Unknown (%s)", sp.Printer.Connection.Endpoint)
}

var printerProtocols = map[string]printer.Protocol{
	"ESCPOS":   printer.ESCPOS,
	"TSPL":     printer.TSPL,
	"ZPL":      printer.ZPL,
	"CPCL":     printer.CPCL,
	"EPL":      printer.EPL,
	"StarPRNT": printer.StarPRNT,
}

var printerProtocolOptions = []string{"ESCPOS", "TSPL", "ZPL", "CPCL", "EPL", "StarPRNT"}

var printerTransports = map[string]printer.TransportKind{
	"TCP":               printer.TCP,
	"Bluetooth Classic": printer.BluetoothClassic,
}

var printerTransportOptions = []string{"TCP", "Bluetooth Classic"}

func protocolDisplayName(p printer.Protocol) string {
	for name, candidate := range printerProtocols {
		if candidate == p {
			return name
		}
	}
	return ""
}

func transportDisplayName(t printer.TransportKind) string {
	for name, candidate := range printerTransports {
		if candidate == t {
			return name
		}
	}
	return ""
}

func NewWindow(application fyne.App) fyne.Window {
	window := application.NewWindow("PrintCat")
	window.Resize(fyne.NewSize(1100, 700))

	configPath := filepath.Join(application.Storage().RootURI().Path(), "config.json")

	service := bridge.Bootstrap(configPath)
	if service == nil {
		dialog.ShowError(fmt.Errorf("failed to initialize print engine"), window)
		return window
	}
	renderer := &basic.Renderer{}
	bridgeState := bridge.GetGlobal()
	if bridgeState == nil {
		dialog.ShowError(fmt.Errorf("bridge state not initialized"), window)
		return window
	}

	startupCfg, startupErr := config.Load(configPath)
	if startupErr != nil {
		startupCfg = config.Default()
	}
	initialWidth, initialHeight := 80_000, 200_000
	initialPaperW := 80
	initialPaperH := 200
	var initialActive printer.Printer
	hasInitialActive := false
	if sp := startupCfg.ActiveSavedPrinter(); sp != nil {
		if sp.PaperWidthMm > 0 && sp.PaperHeightMm > 0 {
			initialWidth = sp.PaperWidthMm * 1000
			initialHeight = sp.PaperHeightMm * 1000
			initialPaperW = sp.PaperWidthMm
			initialPaperH = sp.PaperHeightMm
		}
		initialActive = sp.Printer
		hasInitialActive = true
		pCopy := initialActive
		bridgeState.SetActivePrinter(&pCopy)
	}

	doc := document.New("doc1", "My Document", document.Size{
		Width:  document.Unit(initialWidth),
		Height: document.Unit(initialHeight),
	})
	ed := editor.New(&doc)

	// =====================================================================
	// MUTABLE STATE
	// =====================================================================
	//
	// All widget mutations must run on the Fyne UI thread. To keep the
	// boundary explicit, we separate:
	//   - xxxUI()  : closes over widgets; call only from UI thread
	//   - xxx()    : safe from any goroutine; wraps xxxUI() in fyne.Do

	var activePrinter *printer.Printer
	var androidCombinedStatus *widget.Label

	var pendingFilePath string
	var pendingFileData []byte
	var pendingFileInfo struct {
		Name string
		Path string
		Ext  string
		Size string
	}

	var isPrinting bool
	var printerSelectUpdating bool

	// Forward-declared UI-only closures.
	var updateCombinedStatusDisplayUI func()
	var updateConfiguredStatusUI func()
	var reloadSavedPrintersUI func(config.Config)

	// Forward-declared goroutine-safe wrappers.
	var reloadSavedPrinters func()
	var updateConfiguredStatus func()
	var applySavedPrinter func(config.SavedPrinter) error
	var deleteSavedPrinter func(string, func())
	var openConfigureDialog func(*platform.Device, *config.SavedPrinter, func())

	// =====================================================================
	// WIDGET CONSTRUCTION
	// =====================================================================
	//
	// In Fyne, constructing widgets is safe from any goroutine. Mutating
	// them afterwards (SetText, SetSelected, Refresh, Options=, Enable,
	// Disable, Show, Hide, SetContent) requires the UI thread.

	selectedLabel := widget.NewLabel("Selected: none")

	canvasWidget := NewCanvas(ed, func(id string) {
		if id == "" {
			selectedLabel.SetText("Selected: none")
		} else {
			selectedLabel.SetText(fmt.Sprintf("Selected: %s", id))
		}
		if runtime.GOOS == "android" && updateCombinedStatusDisplayUI != nil {
			updateCombinedStatusDisplayUI()
		}
	})

	scroll := container.NewScroll(canvasWidget)

	zoomLabel := widget.NewLabel("100%")
	zoomIn := widget.NewButton("+", func() {
		ed.SetZoom(ed.Zoom + 0.1)
		zoomLabel.SetText(fmt.Sprintf("%.0f%%", ed.Zoom*100))
		canvasWidget.Refresh()
	})
	zoomOut := widget.NewButton("-", func() {
		ed.SetZoom(ed.Zoom - 0.1)
		zoomLabel.SetText(fmt.Sprintf("%.0f%%", ed.Zoom*100))
		canvasWidget.Refresh()
	})
	fitButton := widget.NewButton("Fit", func() {
		winW := window.Canvas().Size().Width
		docW, _ := ed.DocToView(document.Point{X: ed.Doc.PageSize.Width, Y: 0})
		if docW > 0 {
			ed.SetZoom(float64(winW) / docW)
			zoomLabel.SetText(fmt.Sprintf("%.0f%%", ed.Zoom*100))
			canvasWidget.Refresh()
		}
	})

	paperWidth := widget.NewEntry()
	paperHeight := widget.NewEntry()
	paperApply := widget.NewButton("Set Paper", func() {
		w, errW := strconv.Atoi(paperWidth.Text)
		h, errH := strconv.Atoi(paperHeight.Text)
		if errW != nil || errH != nil || w <= 0 || h <= 0 {
			return
		}
		ed.SetPaperSize(document.Unit(w)*1000, document.Unit(h)*1000)
		canvasWidget.Refresh()
	})

	var addText *widget.Button
	var addImage *widget.Button
	var deleteButton *widget.Button
	var previewButton *widget.Button

	if runtime.GOOS == "android" {
		addText = widget.NewButton("Text", func() {
			pos := document.Point{X: 10_000, Y: 10_000}
			size := document.Size{Width: 40_000, Height: 10_000}
			ed.AddText("Hello", pos, size, 12_000)
			canvasWidget.Refresh()
		})
		addImage = widget.NewButton("Image", func() {
			dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
				if err != nil || reader == nil {
					return
				}
				defer reader.Close()
				data, err := io.ReadAll(reader)
				if err != nil {
					return
				}
				if err := ed.AddImageFitToPage(data, "image/png"); err != nil {
					dialog.ShowError(fmt.Errorf("open image: %w", err), window)
					return
				}
				canvasWidget.Refresh()
			}, window)
		})
		deleteButton = widget.NewButton("Del", func() {
			ed.DeleteSelected()
			canvasWidget.Refresh()
		})
		previewButton = widget.NewButton("Prev", func() {
			ShowPreview(application, ed)
		})
	} else {
		addText = widget.NewButton("Add Text", func() {
			pos := document.Point{X: 10_000, Y: 10_000}
			size := document.Size{Width: 40_000, Height: 10_000}
			ed.AddText("Hello", pos, size, 12_000)
			canvasWidget.Refresh()
		})
		addImage = widget.NewButton("Add Image", func() {
			dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
				if err != nil || reader == nil {
					return
				}
				defer reader.Close()
				data, err := os.ReadFile(reader.URI().Path())
				if err != nil {
					return
				}
				if err := ed.AddImageFitToPage(data, "image/png"); err != nil {
					dialog.ShowError(fmt.Errorf("open image: %w", err), window)
					return
				}
				canvasWidget.Refresh()
			}, window)
		})
		deleteButton = widget.NewButton("Delete", func() {
			ed.DeleteSelected()
			canvasWidget.Refresh()
		})
		previewButton = widget.NewButton("Preview", func() {
			ShowPreview(application, ed)
		})
	}

	printerStatus := widget.NewLabel("")
	configuredStatus := widget.NewLabel("Not configured")

	printerSelect := widget.NewSelect([]string{}, nil)
	printerSelect.PlaceHolder = "No saved printers"

	var printButton *widget.Button

	// =====================================================================
	// UI-THREAD-ONLY CLOSURES
	// =====================================================================
	//
	// These may mutate widgets directly. Callers must either already be on
	// the Fyne UI thread (Fyne callbacks) or wrap the call in fyne.Do.

	updateCombinedStatusDisplayUI = func() {
		if androidCombinedStatus == nil {
			return
		}
		statusParts := []string{}
		if printerStatus.Text != "" {
			statusParts = append(statusParts, printerStatus.Text)
		}
		if configuredStatus.Text != "" && configuredStatus.Text != "Not configured" {
			statusParts = append(statusParts, configuredStatus.Text)
		}
		statusStr := ""
		if len(statusParts) > 0 {
			statusStr = " | " + statusParts[0]
			for i := 1; i < len(statusParts); i++ {
				statusStr += " | " + statusParts[i]
			}
		}
		selText := selectedLabel.Text
		if selText == "" || selText == "Selected: none" {
			selText = "No selection"
		}
		fileStatus := ""
		if pendingFilePath != "" && pendingFileInfo.Name != "" {
			fileStatus = " | File: " + pendingFileInfo.Name
		}
		if statusStr != "" {
			androidCombinedStatus.SetText("Status:" + statusStr + fileStatus + " | " + selText + " (drag to move)")
		} else if fileStatus != "" {
			androidCombinedStatus.SetText("Status:" + fileStatus + " | " + selText + " (drag to move)")
		} else {
			androidCombinedStatus.SetText(selText + " (drag to move)")
		}
	}

	updateConfiguredStatusUI = func() {
		if activePrinter == nil {
			configuredStatus.SetText("Not configured")
		} else {
			configuredStatus.SetText(fmt.Sprintf(
				"Active: %s [%s / %s DPI %d]",
				activePrinter.Name,
				protocolDisplayName(activePrinter.Connection.Protocol),
				transportDisplayName(activePrinter.Connection.Transport),
				activePrinter.Profile.DPI,
			))
		}
		updateCombinedStatusDisplayUI()
	}

	reloadSavedPrintersUI = func(loaded config.Config) {
		options := make([]string, 0, len(loaded.SavedPrinters))
		var selectedDisplay string
		for _, sp := range loaded.SavedPrinters {
			display := savedPrinterDisplay(sp)
			options = append(options, display)
			if sp.Printer.ID == loaded.ActivePrinterID {
				selectedDisplay = display
			}
		}
		printerSelectUpdating = true
		printerSelect.Options = options
		printerSelect.Refresh()
		if selectedDisplay != "" {
			printerSelect.SetSelected(selectedDisplay)
		} else if len(options) == 0 {
			printerSelect.ClearSelected()
		}
		printerSelectUpdating = false
	}

	// =====================================================================
	// GOROUTINE-SAFE WRAPPERS
	// =====================================================================

	reloadSavedPrinters = func() {
		loaded, err := config.Load(configPath)
		if err != nil {
			loaded = config.Default()
		}
		loadedCopy := loaded
		fyne.Do(func() {
			reloadSavedPrintersUI(loadedCopy)
		})
	}

	updateConfiguredStatus = func() {
		fyne.Do(updateConfiguredStatusUI)
	}

	applySavedPrinter = func(sp config.SavedPrinter) error {
		loaded, err := config.Load(configPath)
		if err != nil {
			loaded = config.Default()
		}
		loaded.UpsertSavedPrinter(sp)
		loaded.ActivePrinterID = sp.Printer.ID
		if err := config.Save(configPath, loaded); err != nil {
			return err
		}
		spCopy := sp
		loadedCopy := loaded
		fyne.Do(func() {
			p := spCopy.Printer
			activePrinter = &p
			bridgeState.SetActivePrinter(&p)
			if spCopy.PaperWidthMm > 0 && spCopy.PaperHeightMm > 0 {
				ed.SetPaperSize(
					document.Unit(spCopy.PaperWidthMm)*1000,
					document.Unit(spCopy.PaperHeightMm)*1000,
				)
				paperWidth.SetText(strconv.Itoa(spCopy.PaperWidthMm))
				paperHeight.SetText(strconv.Itoa(spCopy.PaperHeightMm))
				canvasWidget.Refresh()
			}
			reloadSavedPrintersUI(loadedCopy)
			updateConfiguredStatusUI()
		})
		return nil
	}

	deleteSavedPrinter = func(id string, onDone func()) {
		loaded, err := config.Load(configPath)
		if err != nil {
			return
		}
		target := loaded.FindSavedPrinter(id)
		if target == nil {
			return
		}
		name := target.Printer.Name
		if name == "" {
			name = target.Printer.Connection.Endpoint
		}
		dialog.ShowConfirm(
			"Delete Printer",
			fmt.Sprintf("Remove %q from saved printers?", name),
			func(confirmed bool) {
				if !confirmed {
					return
				}
				fresh, err := config.Load(configPath)
				if err != nil {
					dialog.ShowError(fmt.Errorf("failed to load config: %w", err), window)
					return
				}
				if !fresh.RemoveSavedPrinter(id) {
					return
				}
				if err := config.Save(configPath, fresh); err != nil {
					dialog.ShowError(fmt.Errorf("failed to save config: %w", err), window)
					return
				}
				if activePrinter != nil && activePrinter.ID == id {
					activePrinter = nil
					bridgeState.SetActivePrinter(nil)
				}
				freshCopy := fresh
				fyne.Do(func() {
					reloadSavedPrintersUI(freshCopy)
					updateConfiguredStatusUI()
				})
				if onDone != nil {
					onDone()
				}
			},
			window,
		)
	}

	openConfigureDialog = func(dev *platform.Device, sp *config.SavedPrinter, onSaved func()) {
		if dev == nil && sp == nil {
			dialog.ShowInformation("No Printer", "Select a printer first.", window)
			return
		}

		var initialName, initialEndpoint string
		var initialDPI int
		var initialProtocol printer.Protocol
		var initialTransport printer.TransportKind
		var baseProfile printer.PrinterProfile

		if sp != nil {
			initialName = sp.Printer.Name
			initialEndpoint = sp.Printer.Connection.Endpoint
			initialDPI = sp.Printer.Profile.DPI
			initialProtocol = sp.Printer.Connection.Protocol
			initialTransport = sp.Printer.Connection.Transport
			baseProfile = sp.Printer.Profile
		} else if dev != nil {
			initialName = dev.Name
			initialEndpoint = dev.Endpoint
			initialDPI = dev.Profile.DPI
			baseProfile = dev.Profile
			initialProtocol = printer.ESCPOS
			initialTransport = printer.BluetoothClassic
			if dev.Kind == printer.TCP {
				initialTransport = printer.TCP
			}
		}
		if initialDPI <= 0 {
			initialDPI = 203
		}
		if initialProtocol == "" {
			initialProtocol = printer.ESCPOS
		}
		if initialTransport == "" {
			initialTransport = printer.BluetoothClassic
		}

		protocolSelect := widget.NewSelect(printerProtocolOptions, nil)
		transportSelect := widget.NewSelect(printerTransportOptions, nil)
		nameEntry := widget.NewEntry()
		endpointEntry := widget.NewEntry()
		dpiEntry := widget.NewEntry()
		paperWidthEntry := widget.NewEntry()
		paperHeightEntry := widget.NewEntry()
		errorLabel := widget.NewLabel("")

		protocolSelect.SetSelected(protocolDisplayName(initialProtocol))
		transportSelect.SetSelected(transportDisplayName(initialTransport))
		nameEntry.SetText(initialName)
		endpointEntry.SetText(initialEndpoint)
		dpiEntry.SetText(strconv.Itoa(initialDPI))
		if sp != nil && sp.PaperWidthMm > 0 {
			paperWidthEntry.SetText(strconv.Itoa(sp.PaperWidthMm))
		} else {
			paperWidthEntry.SetText("80")
		}
		if sp != nil && sp.PaperHeightMm > 0 {
			paperHeightEntry.SetText(strconv.Itoa(sp.PaperHeightMm))
		} else {
			paperHeightEntry.SetText("200")
		}

		title := "Add Printer"
		if sp != nil {
			title = "Configure Printer"
		}
		configWindow := application.NewWindow(title)
		configWindow.Resize(fyne.NewSize(420, 560))

		applyButton := widget.NewButton("Apply", func() {
			errorLabel.SetText("")

			selectedProtocol, okP := printerProtocols[protocolSelect.Selected]
			if !okP {
				errorLabel.SetText("Please select a protocol")
				return
			}
			selectedTransport, okT := printerTransports[transportSelect.Selected]
			if !okT {
				errorLabel.SetText("Please select a transport")
				return
			}
			endpoint := endpointEntry.Text
			if endpoint == "" {
				errorLabel.SetText("Endpoint cannot be empty")
				return
			}
			name := nameEntry.Text
			if name == "" {
				name = endpoint
			}
			dpi, err := strconv.Atoi(dpiEntry.Text)
			if err != nil || dpi <= 0 {
				errorLabel.SetText("DPI must be a positive integer")
				return
			}
			pw, errW := strconv.Atoi(paperWidthEntry.Text)
			if errW != nil || pw <= 0 {
				errorLabel.SetText("Paper width must be a positive integer")
				return
			}
			ph, errH := strconv.Atoi(paperHeightEntry.Text)
			if errH != nil || ph <= 0 {
				errorLabel.SetText("Paper height must be a positive integer")
				return
			}

			var id string
			if sp != nil {
				id = sp.Printer.ID
			} else if dev != nil {
				id = dev.ID
			} else {
				id = endpoint
			}

			profile := baseProfile
			profile.DPI = dpi

			savedPrinter := config.SavedPrinter{
				Printer: printer.Printer{
					ID:   id,
					Name: name,
					Connection: printer.Connection{
						Protocol:  selectedProtocol,
						Transport: selectedTransport,
						Endpoint:  endpoint,
					},
					Profile: profile,
				},
				PaperWidthMm:  pw,
				PaperHeightMm: ph,
			}

			if err := savedPrinter.Printer.Validate(); err != nil {
				errorLabel.SetText(fmt.Sprintf("Validation error: %v", err))
				return
			}

			if err := applySavedPrinter(savedPrinter); err != nil {
				errorLabel.SetText(fmt.Sprintf("Failed to save config: %v", err))
				return
			}
			if onSaved != nil {
				onSaved()
			}
			configWindow.Close()
		})

		cancelButton := widget.NewButton("Cancel", func() {
			configWindow.Close()
		})

		form := container.NewVBox(
			widget.NewLabelWithStyle("Name:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			nameEntry,
			widget.NewLabelWithStyle("Endpoint (MAC / host:port):", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			endpointEntry,
			widget.NewLabelWithStyle("Protocol:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			protocolSelect,
			widget.NewLabelWithStyle("Transport:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			transportSelect,
			widget.NewLabelWithStyle("DPI:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			dpiEntry,
			widget.NewLabelWithStyle("Paper width (mm):", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			paperWidthEntry,
			widget.NewLabelWithStyle("Paper height (mm):", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			paperHeightEntry,
			errorLabel,
			container.NewHBox(applyButton, cancelButton),
		)

		configWindow.SetContent(container.NewVScroll(form))
		configWindow.Show()
	}

	// =====================================================================
	// WIDGET EVENT HANDLERS
	// =====================================================================
	//
	// Fyne invokes these on the UI thread.

	printerSelect.OnChanged = func(selected string) {
		if printerSelectUpdating {
			return
		}
		if selected == "" {
			return
		}
		loaded, err := config.Load(configPath)
		if err != nil {
			return
		}
		for i := range loaded.SavedPrinters {
			sp := loaded.SavedPrinters[i]
			if savedPrinterDisplay(sp) != selected {
				continue
			}
			if loaded.ActivePrinterID == sp.Printer.ID {
				return
			}
			loaded.ActivePrinterID = sp.Printer.ID
			if err := config.Save(configPath, loaded); err != nil {
				dialog.ShowError(fmt.Errorf("failed to save config: %w", err), window)
				return
			}
			p := sp.Printer
			activePrinter = &p
			bridgeState.SetActivePrinter(&p)
			if sp.PaperWidthMm > 0 && sp.PaperHeightMm > 0 {
				ed.SetPaperSize(
					document.Unit(sp.PaperWidthMm)*1000,
					document.Unit(sp.PaperHeightMm)*1000,
				)
				paperWidth.SetText(strconv.Itoa(sp.PaperWidthMm))
				paperHeight.SetText(strconv.Itoa(sp.PaperHeightMm))
				canvasWidget.Refresh()
			}
			updateConfiguredStatusUI()
			return
		}
	}

	printButton = widget.NewButton("Print", func() {
		if isPrinting {
			return
		}
		if activePrinter == nil {
			dialog.ShowInformation("No Printer", "Please configure an active printer first.", window)
			return
		}
		if err := activePrinter.Validate(); err != nil {
			dialog.ShowError(fmt.Errorf("invalid printer: %w", err), window)
			return
		}

		printerStatus.SetText("Printing...")
		updateCombinedStatusDisplayUI()
		isPrinting = true
		printButton.Disable()

		printerCopy := *activePrinter

		go func() {
			defer func() {
				fyne.Do(func() {
					isPrinting = false
					printButton.Enable()
				})
			}()

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			if printerCopy.Connection.Transport == printer.BluetoothClassic {
				granted, err := platform.EnsureBluetoothConnectPermission(ctx)
				if err != nil {
					fyne.Do(func() {
						printerStatus.SetText(fmt.Sprintf("Permission error: %v", err))
						updateCombinedStatusDisplayUI()
						dialog.ShowError(fmt.Errorf("bluetooth permission error: %w", err), window)
					})
					return
				}
				if !granted {
					fyne.Do(func() {
						printerStatus.SetText("Bluetooth permission denied")
						updateCombinedStatusDisplayUI()
						dialog.ShowInformation("Permission Denied", "Bluetooth permission is required to print.", window)
					})
					return
				}
			}

			err := service.Print(ctx, printerCopy, doc, renderer)
			if err != nil {
				fyne.Do(func() {
					printerStatus.SetText(fmt.Sprintf("Print failed: %v", err))
					updateCombinedStatusDisplayUI()
					dialog.ShowError(fmt.Errorf("print failed: %w", err), window)
				})
				return
			}

			fyne.Do(func() {
				printerStatus.SetText("Print successful")
				updateCombinedStatusDisplayUI()
				dialog.ShowInformation("Success", "Print job completed successfully.", window)
			})
		}()
	})

	refreshButton := widget.NewButton("Refresh", func() {
		reloadSavedPrinters()
		updateConfiguredStatus()
	})
	configureButton := widget.NewButton("Configure", func() {
		selected := printerSelect.Selected
		if selected == "" {
			if activePrinter != nil {
				loaded, err := config.Load(configPath)
				if err != nil {
					return
				}
				sp := loaded.FindSavedPrinter(activePrinter.ID)
				if sp != nil {
					spCopy := *sp
					openConfigureDialog(nil, &spCopy, nil)
					return
				}
			}
			dialog.ShowInformation("No Printer", "Add a printer from Scan Printer first.", window)
			return
		}
		loaded, err := config.Load(configPath)
		if err != nil {
			return
		}
		for i := range loaded.SavedPrinters {
			if savedPrinterDisplay(loaded.SavedPrinters[i]) == selected {
				spCopy := loaded.SavedPrinters[i]
				openConfigureDialog(nil, &spCopy, nil)
				return
			}
		}
	})

	printerLabel := widget.NewLabelWithStyle("Printer:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	// androidCombinedStatus is a widget; assignment of fields (Wrapping)
	// happens here on the constructor goroutine, but Fyne only reads them
	// on render. The first render happens after SetContent on the UI thread.
	androidCombinedStatus = widget.NewLabel("")
	androidCombinedStatus.Wrapping = fyne.TextWrapWord

	// =====================================================================
	// INITIAL UI STATE — on the Fyne UI thread
	// =====================================================================

	startupCfgCopy := startupCfg
	fyne.Do(func() {
		if hasInitialActive {
			p := initialActive
			activePrinter = &p
		}
		paperWidth.SetText(strconv.Itoa(initialPaperW))
		paperHeight.SetText(strconv.Itoa(initialPaperH))
		reloadSavedPrintersUI(startupCfgCopy)
		updateConfiguredStatusUI()
		updateCombinedStatusDisplayUI()
	})

	// =====================================================================
	// SCREEN LAYOUTS
	// =====================================================================

	var editorContent *fyne.Container
	var homeContent fyne.CanvasObject

	if runtime.GOOS == "android" {
		topBar := container.NewHBox(
			widget.NewButton("←", func() {
				window.SetContent(homeContent)
			}),
			widget.NewLabel("PrintCat"),
			layout.NewSpacer(),
			zoomOut, zoomIn, fitButton, zoomLabel,
		)

		toolsToolbar := container.NewHBox(addText, addImage, deleteButton, previewButton)

		bottomControls := container.NewVBox(
			printerSelect,
			container.NewHBox(refreshButton, configureButton, printButton),
			androidCombinedStatus,
		)

		editorContent = container.NewBorder(
			topBar,
			bottomControls,
			nil,
			nil,
			container.NewBorder(
				toolsToolbar,
				nil,
				nil,
				nil,
				scroll,
			),
		)
	} else {
		topBar := container.NewHBox(
			widget.NewLabel("PrintCat"),
			widget.NewLabel("|"),
			widget.NewLabel("Paper:"), paperWidth, widget.NewLabel("mm x"), paperHeight, widget.NewLabel("mm"), paperApply,
			widget.NewLabel("| Zoom:"), zoomOut, zoomIn, fitButton, zoomLabel,
		)

		leftPanel := container.NewVBox(
			widget.NewLabel("Tools"),
			container.NewHBox(addText, addImage),
			container.NewHBox(deleteButton, previewButton),
			widget.NewSeparator(),
			printerLabel,
			container.NewHBox(printerSelect, refreshButton),
			container.NewHBox(configureButton, printButton),
			printerStatus,
			configuredStatus,
			widget.NewSeparator(),
			selectedLabel,
			widget.NewLabel("(drag to move)"),
		)

		content := container.NewBorder(topBar, nil, leftPanel, nil, scroll)

		footerText1 := canvas.NewText("© 2026 PrintCat — Printing Tool by Pram", theme.ForegroundColor())
		footerText1.Alignment = fyne.TextAlignCenter
		footerText1.TextSize = 14

		footerText2 := canvas.NewText("Dedicated to my beloved wife, Apdini Nurrayani", color.Gray{Y: 120})
		footerText2.Alignment = fyne.TextAlignCenter
		footerText2.TextSize = 11

		footer := container.NewCenter(
			container.NewVBox(
				footerText1,
				footerText2,
			),
		)

		editorContent = container.NewBorder(nil, footer, nil, nil, content)
	}

	// ---------------------------------------------------------------------
	// Scan Printer screen
	// ---------------------------------------------------------------------

	buildScanPrinterScreen := func() fyne.CanvasObject {
		statusLabel := widget.NewLabel("Tap Scan to discover printers")

		savedRows := container.NewVBox()
		discoveredRows := container.NewVBox()

		var discovered []platform.Device

		var renderSavedRows func()
		var renderDiscoveredRows func()

		renderSavedRows = func() {
			loaded, err := config.Load(configPath)
			if err != nil {
				loaded = config.Default()
			}
			loadedCopy := loaded
			fyne.Do(func() {
				savedRows.RemoveAll()
				if len(loadedCopy.SavedPrinters) == 0 {
					savedRows.Add(widget.NewLabel("(no saved printers yet)"))
					savedRows.Refresh()
					return
				}
				for i := range loadedCopy.SavedPrinters {
					spCopy := loadedCopy.SavedPrinters[i]
					idCopy := spCopy.Printer.ID
					isActive := loadedCopy.ActivePrinterID == idCopy

					titleText := spCopy.Printer.Name
					if titleText == "" {
						titleText = spCopy.Printer.Connection.Endpoint
					}
					subtitle := fmt.Sprintf("%s  |  %s / %s  |  DPI %d",
						spCopy.Printer.Connection.Endpoint,
						protocolDisplayName(spCopy.Printer.Connection.Protocol),
						transportDisplayName(spCopy.Printer.Connection.Transport),
						spCopy.Printer.Profile.DPI)
					if spCopy.PaperWidthMm > 0 && spCopy.PaperHeightMm > 0 {
						subtitle += fmt.Sprintf("  |  %dx%d mm", spCopy.PaperWidthMm, spCopy.PaperHeightMm)
					}

					titleLabel := widget.NewLabelWithStyle(titleText, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
					subLabel := widget.NewLabel(subtitle)
					activeLabel := widget.NewLabel("")
					if isActive {
						activeLabel.SetText("[Active]")
					}
					info := container.NewVBox(titleLabel, subLabel, activeLabel)

					spForSelect := spCopy
					spForConfig := spCopy
					titleForStatus := titleText

					selectBtn := widget.NewButton("Select Active", func() {
						if err := applySavedPrinter(spForSelect); err != nil {
							dialog.ShowError(fmt.Errorf("failed to set active: %w", err), window)
							return
						}
						statusLabel.SetText("Active: " + titleForStatus)
						renderSavedRows()
					})
					configureBtn := widget.NewButton("Configure", func() {
						openConfigureDialog(nil, &spForConfig, func() {
							renderSavedRows()
						})
					})
					deleteBtn := widget.NewButton("Delete", func() {
						deleteSavedPrinter(idCopy, func() {
							renderSavedRows()
						})
					})

					row := container.NewBorder(
						nil, nil, nil,
						container.NewHBox(selectBtn, configureBtn, deleteBtn),
						info,
					)
					savedRows.Add(row)
					savedRows.Add(widget.NewSeparator())
				}
				savedRows.Refresh()
			})
		}

		renderDiscoveredRows = func() {
			loaded, err := config.Load(configPath)
			if err != nil {
				loaded = config.Default()
			}
			snapshot := make([]platform.Device, len(discovered))
			copy(snapshot, discovered)
			loadedCopy := loaded
			fyne.Do(func() {
				discoveredRows.RemoveAll()
				if len(snapshot) == 0 {
					discoveredRows.Add(widget.NewLabel("(no discovered printers)"))
					discoveredRows.Refresh()
					return
				}
				for i := range snapshot {
					devCopy := snapshot[i]

					titleText := devCopy.Name
					if titleText == "" {
						titleText = devCopy.Endpoint
					}
					titleLabel := widget.NewLabelWithStyle(titleText, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
					subLabel := widget.NewLabel(devCopy.Endpoint)
					info := container.NewVBox(titleLabel, subLabel)

					var existing *config.SavedPrinter
					for j := range loadedCopy.SavedPrinters {
						if loadedCopy.SavedPrinters[j].Printer.ID == devCopy.ID {
							existing = &loadedCopy.SavedPrinters[j]
							break
						}
						if loadedCopy.SavedPrinters[j].Printer.Connection.Endpoint == devCopy.Endpoint {
							existing = &loadedCopy.SavedPrinters[j]
							break
						}
					}

					var actionBtn *widget.Button
					if existing != nil {
						spExisting := *existing
						actionBtn = widget.NewButton("Configure", func() {
							openConfigureDialog(nil, &spExisting, func() {
								renderSavedRows()
								renderDiscoveredRows()
							})
						})
					} else {
						devForAdd := devCopy
						actionBtn = widget.NewButton("Add", func() {
							openConfigureDialog(&devForAdd, nil, func() {
								renderSavedRows()
								renderDiscoveredRows()
							})
						})
					}

					row := container.NewBorder(nil, nil, nil, actionBtn, info)
					discoveredRows.Add(row)
					discoveredRows.Add(widget.NewSeparator())
				}
				discoveredRows.Refresh()
			})
		}

		renderSavedRows()
		renderDiscoveredRows()

		var isScanning bool
		scanButton := widget.NewButton("Scan", nil)
		scanButton.OnTapped = func() {
			if isScanning {
				return
			}
			isScanning = true
			scanButton.Disable()
			statusLabel.SetText("Scanning...")

			go func() {
				ctx := context.Background()
				results, err := discoverBluetoothPrinters(ctx, window)
				fyne.Do(func() {
					isScanning = false
					scanButton.Enable()
					if err != nil {
						statusLabel.SetText(fmt.Sprintf("Scan error: %v", err))
						dialog.ShowError(fmt.Errorf("discovery error: %w", err), window)
						return
					}
					discovered = results
					if len(results) == 0 {
						statusLabel.SetText("No printers found")
					} else {
						statusLabel.SetText(fmt.Sprintf("Found %d printer(s)", len(results)))
					}
					renderDiscoveredRows()
				})
			}()
		}

		header := container.NewHBox(
			widget.NewButton("Back", func() {
				window.SetContent(homeContent)
			}),
			widget.NewLabelWithStyle("Scan Printer", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			layout.NewSpacer(),
			scanButton,
		)

		body := container.NewVBox(
			statusLabel,
			widget.NewSeparator(),
			widget.NewLabelWithStyle("Saved Printers", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			savedRows,
			widget.NewSeparator(),
			widget.NewLabelWithStyle("Discovered Printers", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			discoveredRows,
		)

		return container.NewBorder(header, nil, nil, nil, container.NewVScroll(body))
	}

	// ---------------------------------------------------------------------
	// Settings screen
	// ---------------------------------------------------------------------

	buildSettingsScreen := func() fyne.CanvasObject {
		loadedSettings, loadErr := config.Load(configPath)
		if loadErr != nil {
			loadedSettings = config.Default()
		}
		active := loadedSettings.ActiveSavedPrinter()

		protocolSelect := widget.NewSelect(printerProtocolOptions, nil)
		transportSelect := widget.NewSelect(printerTransportOptions, nil)
		dpiEntry := widget.NewEntry()
		paperWidthEntry := widget.NewEntry()
		paperHeightEntry := widget.NewEntry()
		statusLabel := widget.NewLabel("")

		nameLabel := widget.NewLabel("")
		endpointLabel := widget.NewLabel("")

		if active != nil {
			activeCopy := *active
			nameLabel.SetText(fmt.Sprintf("Editing: %s", activeCopy.Printer.Name))
			endpointLabel.SetText(activeCopy.Printer.Connection.Endpoint)
			protocolSelect.SetSelected(protocolDisplayName(activeCopy.Printer.Connection.Protocol))
			transportSelect.SetSelected(transportDisplayName(activeCopy.Printer.Connection.Transport))
			if activeCopy.Printer.Profile.DPI > 0 {
				dpiEntry.SetText(strconv.Itoa(activeCopy.Printer.Profile.DPI))
			} else {
				dpiEntry.SetText("203")
			}
			if activeCopy.PaperWidthMm > 0 {
				paperWidthEntry.SetText(strconv.Itoa(activeCopy.PaperWidthMm))
			} else {
				paperWidthEntry.SetText("80")
			}
			if activeCopy.PaperHeightMm > 0 {
				paperHeightEntry.SetText(strconv.Itoa(activeCopy.PaperHeightMm))
			} else {
				paperHeightEntry.SetText("200")
			}
		} else {
			nameLabel.SetText("No active printer configured")
			endpointLabel.SetText("Use Scan Printer to add and activate a printer first.")
			protocolSelect.SetSelected("ESCPOS")
			transportSelect.SetSelected("Bluetooth Classic")
			dpiEntry.SetText("203")
			paperWidthEntry.SetText("80")
			paperHeightEntry.SetText("200")
		}

		saveButton := widget.NewButton("Save", func() {
			statusLabel.SetText("")
			current, err := config.Load(configPath)
			if err != nil {
				current = config.Default()
			}
			ap := current.ActiveSavedPrinter()
			if ap == nil {
				statusLabel.SetText("No active printer to edit")
				return
			}

			selectedProtocol, okP := printerProtocols[protocolSelect.Selected]
			if !okP {
				statusLabel.SetText("Please select a protocol")
				return
			}
			selectedTransport, okT := printerTransports[transportSelect.Selected]
			if !okT {
				statusLabel.SetText("Please select a transport")
				return
			}
			dpi, errD := strconv.Atoi(dpiEntry.Text)
			if errD != nil || dpi <= 0 {
				statusLabel.SetText("DPI must be a positive integer")
				return
			}
			pw, errW := strconv.Atoi(paperWidthEntry.Text)
			if errW != nil || pw <= 0 {
				statusLabel.SetText("Paper width must be a positive integer")
				return
			}
			ph, errH := strconv.Atoi(paperHeightEntry.Text)
			if errH != nil || ph <= 0 {
				statusLabel.SetText("Paper height must be a positive integer")
				return
			}

			ap.Printer.Connection.Protocol = selectedProtocol
			ap.Printer.Connection.Transport = selectedTransport
			ap.Printer.Profile.DPI = dpi
			ap.PaperWidthMm = pw
			ap.PaperHeightMm = ph

			if err := config.Save(configPath, current); err != nil {
				statusLabel.SetText(fmt.Sprintf("Save failed: %v", err))
				return
			}
			p := ap.Printer
			activePrinter = &p
			bridgeState.SetActivePrinter(&p)
			ed.SetPaperSize(document.Unit(pw)*1000, document.Unit(ph)*1000)
			paperWidth.SetText(strconv.Itoa(pw))
			paperHeight.SetText(strconv.Itoa(ph))
			canvasWidget.Refresh()
			currentCopy := current
			reloadSavedPrintersUI(currentCopy)
			updateConfiguredStatusUI()
			statusLabel.SetText("Saved")
		})

		header := container.NewHBox(
			widget.NewButton("Back", func() {
				window.SetContent(homeContent)
			}),
			widget.NewLabelWithStyle("Settings", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		)

		body := container.NewVBox(
			nameLabel,
			endpointLabel,
			widget.NewSeparator(),
			widget.NewLabelWithStyle("Protocol", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			protocolSelect,
			widget.NewLabelWithStyle("Transport", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			transportSelect,
			widget.NewLabelWithStyle("DPI", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			dpiEntry,
			widget.NewSeparator(),
			widget.NewLabelWithStyle("Paper", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			container.NewHBox(widget.NewLabel("Width (mm)"), paperWidthEntry),
			container.NewHBox(widget.NewLabel("Height (mm)"), paperHeightEntry),
			widget.NewSeparator(),
			saveButton,
			statusLabel,
		)

		return container.NewBorder(header, nil, nil, nil, container.NewVScroll(body))
	}

	// ---------------------------------------------------------------------
	// Add File screen
	// ---------------------------------------------------------------------

	buildAddFileScreen := func() fyne.CanvasObject {
		statusLabel := widget.NewLabel("No file selected")
		nameLabel := widget.NewLabel("")
		pathLabel := widget.NewLabel("")
		typeLabel := widget.NewLabel("")
		sizeLabel := widget.NewLabel("")

		chooseButton := widget.NewButton("Choose File", nil)

		openInEditorButton := widget.NewButton("Open in Editor", nil)
		openInEditorButton.Hide()

		chooseAnotherButton := widget.NewButton("Choose Another File", nil)
		chooseAnotherButton.Hide()

		updateFileInfo := func(name, path, ext, sizeStr string) {
			if name != "" {
				statusLabel.SetText("Selected File:")
				nameLabel.SetText(fmt.Sprintf("Name: %s", name))
				pathLabel.SetText(fmt.Sprintf("Path: %s", path))
				typeLabel.SetText("Type: Unknown")
				if sizeStr != "" {
					sizeLabel.SetText(fmt.Sprintf("Size: %s", sizeStr))
				} else {
					sizeLabel.SetText("Size: Unknown")
				}
				openInEditorButton.Show()
				chooseAnotherButton.Show()
			} else {
				statusLabel.SetText("No file selected")
				nameLabel.SetText("")
				pathLabel.SetText("")
				typeLabel.SetText("")
				sizeLabel.SetText("")
				openInEditorButton.Hide()
				chooseAnotherButton.Hide()
			}
		}

		openFilePicker := func() {
			dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
				if err != nil {
					statusLabel.SetText("Error selecting file")
					return
				}
				if reader == nil {
					return
				}
				defer reader.Close()

				uri := reader.URI()
				name := uri.Name()
				path := uri.Path()
				ext := uri.Extension()

				var sizeStr string
				info, statErr := os.Stat(path)
				if statErr == nil {
					sizeBytes := info.Size()
					if sizeBytes < 1024 {
						sizeStr = fmt.Sprintf("%d B", sizeBytes)
					} else if sizeBytes < 1024*1024 {
						sizeStr = fmt.Sprintf("%.1f KB", float64(sizeBytes)/1024)
					} else {
						sizeStr = fmt.Sprintf("%.1f MB", float64(sizeBytes)/(1024*1024))
					}
				}

				data, readErr := io.ReadAll(reader)
				if readErr != nil {
					statusLabel.SetText("Error reading file")
					return
				}

				pendingFilePath = path
				pendingFileData = data
				pendingFileInfo.Name = name
				pendingFileInfo.Path = path
				pendingFileInfo.Ext = ext
				pendingFileInfo.Size = sizeStr

				updateFileInfo(name, path, ext, sizeStr)
				updateCombinedStatusDisplayUI()
			}, window)
		}

		chooseButton.OnTapped = openFilePicker
		chooseAnotherButton.OnTapped = openFilePicker

		openInEditorButton.OnTapped = func() {
			if pendingFilePath != "" {
				if len(pendingFileData) > 0 {
					ed.AddImage(pendingFileData, "image/png",
						document.Point{X: 20_000, Y: 20_000},
						document.Size{Width: 40_000, Height: 30_000})
					canvasWidget.Refresh()
				}
				updateCombinedStatusDisplayUI()
				window.SetContent(editorContent)
			}
		}

		header := container.NewHBox(
			widget.NewButton("Back", func() {
				window.SetContent(homeContent)
			}),
			widget.NewLabelWithStyle("Add File", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		)

		body := container.NewVBox(
			statusLabel,
			chooseButton,
			openInEditorButton,
			chooseAnotherButton,
			widget.NewSeparator(),
			nameLabel,
			pathLabel,
			typeLabel,
			sizeLabel,
		)

		return container.NewBorder(header, nil, nil, nil, body)
	}

	// ---------------------------------------------------------------------
	// Home screen
	// ---------------------------------------------------------------------

	buildHomeScreen := func() fyne.CanvasObject {
		title := canvas.NewText("PrintCat", color.Black)
		title.Alignment = fyne.TextAlignCenter
		title.TextSize = 28
		title.TextStyle = fyne.TextStyle{Bold: true}

		menu := container.NewVBox(
			widget.NewButton("Settings", func() {
				window.SetContent(buildSettingsScreen())
			}),
			widget.NewButton("Scan Printer", func() {
				window.SetContent(buildScanPrinterScreen())
			}),
			widget.NewButton("Add File", func() {
				window.SetContent(buildAddFileScreen())
			}),
			widget.NewButton("Blank Canvas", func() {
				pendingFilePath = ""
				pendingFileData = nil
				pendingFileInfo = struct{ Name, Path, Ext, Size string }{}
				updateCombinedStatusDisplayUI()
				window.SetContent(editorContent)
			}),
		)

		content := container.NewCenter(
			container.NewVBox(
				title,
				widget.NewLabel(""),
				menu,
			),
		)

		footerText1 := canvas.NewText("© 2026 PrintCat — Printing Tool by Pram", color.Gray{Y: 80})
		footerText1.Alignment = fyne.TextAlignCenter
		footerText1.TextSize = 14

		footerText2 := canvas.NewText("Dedicated to my beloved wife, Apdini Nurrayani", color.Gray{Y: 120})
		footerText2.Alignment = fyne.TextAlignCenter
		footerText2.TextSize = 11

		footer := container.NewCenter(
			container.NewVBox(
				footerText1,
				footerText2,
			),
		)

		return container.NewBorder(
			nil,
			footer,
			nil, nil,
			content,
		)
	}

	if runtime.GOOS == "android" {
		homeContent = buildHomeScreen()
		window.SetContent(homeContent)
	} else {
		window.SetContent(editorContent)
	}

	return window
}
