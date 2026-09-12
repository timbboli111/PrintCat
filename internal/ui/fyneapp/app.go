package fyneapp

import (
	"context"
	"fmt"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
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

	"github.com/timboli111/PrintCat/internal/document"
	"github.com/timboli111/PrintCat/internal/editor"
	"github.com/timboli111/PrintCat/internal/platform"
	"github.com/timboli111/PrintCat/internal/printer"
	"github.com/timboli111/PrintCat/internal/printer/cpcl"
	"github.com/timboli111/PrintCat/internal/printer/epl"
	"github.com/timboli111/PrintCat/internal/printer/escpos"
	"github.com/timboli111/PrintCat/internal/printer/starprnt"
	"github.com/timboli111/PrintCat/internal/printer/transport/bluetooth"
	"github.com/timboli111/PrintCat/internal/printer/transport/tcp"
	"github.com/timboli111/PrintCat/internal/printer/tspl"
	"github.com/timboli111/PrintCat/internal/printer/zpl"
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

func NewWindow(application fyne.App) fyne.Window {
	window := application.NewWindow("PrintCat")
	window.Resize(fyne.NewSize(1100, 700))

	service := printer.NewService()
	if err := service.RegisterBackend(&escpos.Encoder{}); err != nil {
		dialog.ShowError(fmt.Errorf("failed to register ESC/POS: %w", err), window)
	}
	if err := service.RegisterBackend(&tspl.Encoder{}); err != nil {
		dialog.ShowError(fmt.Errorf("failed to register TSPL: %w", err), window)
	}
	if err := service.RegisterBackend(&zpl.Encoder{}); err != nil {
		dialog.ShowError(fmt.Errorf("failed to register ZPL: %w", err), window)
	}
	if err := service.RegisterBackend(&cpcl.Encoder{}); err != nil {
		dialog.ShowError(fmt.Errorf("failed to register CPCL: %w", err), window)
	}
	if err := service.RegisterBackend(&epl.Encoder{}); err != nil {
		dialog.ShowError(fmt.Errorf("failed to register EPL: %w", err), window)
	}
	if err := service.RegisterBackend(&starprnt.Encoder{}); err != nil {
		dialog.ShowError(fmt.Errorf("failed to register StarPRNT: %w", err), window)
	}
	if err := service.RegisterTransport(&tcp.TCPTransport{}); err != nil {
		dialog.ShowError(fmt.Errorf("failed to register TCP: %w", err), window)
	}
	if err := service.RegisterTransport(&bluetooth.BluetoothTransport{}); err != nil {
		dialog.ShowError(fmt.Errorf("failed to register Bluetooth: %w", err), window)
	}

	renderer := &basic.Renderer{}
	doc := document.New("doc1", "My Document", document.Size{Width: 80_000, Height: 200_000})
	ed := editor.New(&doc)

	var androidCombinedStatus *widget.Label
	var updateCombinedStatusDisplay func()

	var pendingFilePath string
	var pendingFileData []byte
	var pendingFileInfo struct {
		Name string
		Path string
		Ext  string
		Size string
	}

	selectedLabel := widget.NewLabel("Selected: none")
	canvasWidget := NewCanvas(ed, func(id string) {
		if id == "" {
			selectedLabel.SetText("Selected: none")
		} else {
			selectedLabel.SetText(fmt.Sprintf("Selected: %s", id))
		}
		if runtime.GOOS == "android" && updateCombinedStatusDisplay != nil {
			updateCombinedStatusDisplay()
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
	paperWidth.SetText("80")
	paperHeight := widget.NewEntry()
	paperHeight.SetText("200")
	paperApply := widget.NewButton("Set Paper", func() {
		var w, h float64
		n1, _ := fmt.Sscanf(paperWidth.Text, "%f", &w)
		n2, _ := fmt.Sscanf(paperHeight.Text, "%f", &h)
		if n1 == 1 && n2 == 1 && w > 0 && h > 0 {
			ed.SetPaperSize(document.Unit(w*1000), document.Unit(h*1000))
			canvasWidget.Refresh()
		}
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

	var devices []platform.Device
	var selectedDevice *platform.Device
	var configuredPrinter *printer.Printer
	var isPrinting bool
	var isRefreshing bool
	var printButton *widget.Button

	printerStatus := widget.NewLabel("")
	configuredStatus := widget.NewLabel("Not configured")
	printerSelect := widget.NewSelect([]string{}, func(selected string) {
		for i, dev := range devices {
			display := formatDeviceDisplay(dev)
			if display == selected {
				selectedDevice = &devices[i]
				printerStatus.SetText(fmt.Sprintf("Selected: %s", display))
				configuredStatus.SetText("Not configured")
				configuredPrinter = nil
				if updateCombinedStatusDisplay != nil {
					updateCombinedStatusDisplay()
				}
				return
			}
		}
	})
	printerSelect.PlaceHolder = "No printer found"

	updateConfiguredStatus := func() {
		if configuredPrinter == nil {
			configuredStatus.SetText("Not configured")
		} else {
			configuredStatus.SetText(fmt.Sprintf(
				"Configured: %s / %s (DPI: %d)",
				configuredPrinter.Connection.Protocol,
				configuredPrinter.Connection.Transport,
				configuredPrinter.Profile.DPI,
			))
		}
		if updateCombinedStatusDisplay != nil {
			updateCombinedStatusDisplay()
		}
	}

	updateCombinedStatusDisplay = func() {
		if androidCombinedStatus != nil {
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
			} else {
				androidCombinedStatus.SetText(selText + " (drag to move)")
			}
		}
	}

	refreshDevices := func() {
		if isRefreshing {
			return
		}
		isRefreshing = true
		printerSelect.Disable()
		printerSelect.Refresh()
		printerStatus.SetText("Scanning...")
		if updateCombinedStatusDisplay != nil {
			updateCombinedStatusDisplay()
		}

		var prevID string
		if selectedDevice != nil {
			prevID = selectedDevice.ID
		}

		go func() {
			defer fyne.Do(func() {
				isRefreshing = false
				printerSelect.Enable()
				printerSelect.Refresh()
			})

			ctx := context.Background()

			var discovered []platform.Device
			var err error

			if runtime.GOOS == "android" {
				discovered, err = discoverBluetoothPrinters(ctx, window)
			} else if runtime.GOOS == "windows" {
				integration := platform.GetIntegration()
				discovered, err = integration.Discover(ctx, printer.Serial)
			} else {
				discovered, err = nil, nil
			}

			if err != nil {
				fyne.Do(func() {
					printerStatus.SetText(fmt.Sprintf("Error: %v", err))
					if updateCombinedStatusDisplay != nil {
						updateCombinedStatusDisplay()
					}
					dialog.ShowError(fmt.Errorf("discovery error: %w", err), window)
				})
				return
			}

			fyne.Do(func() {
				devices = discovered
				if len(devices) == 0 {
					printerSelect.Options = []string{}
					printerSelect.PlaceHolder = "No printer found"
					printerStatus.SetText("No printers found")
					selectedDevice = nil
					configuredPrinter = nil
					updateConfiguredStatus()
				} else {
					options := make([]string, len(devices))
					for i, dev := range devices {
						options[i] = formatDeviceDisplay(dev)
					}
					printerSelect.Options = options
					printerSelect.PlaceHolder = "Select a printer"

					selectedIdx := 0
					if prevID != "" {
						for i, dev := range devices {
							if dev.ID == prevID {
								selectedIdx = i
								break
							}
						}
					}
					printerSelect.SetSelected(options[selectedIdx])
					selectedDevice = &devices[selectedIdx]
					printerStatus.SetText(fmt.Sprintf("Selected: %s", options[selectedIdx]))
					if updateCombinedStatusDisplay != nil {
						updateCombinedStatusDisplay()
					}
				}
				printerSelect.Refresh()
			})
		}()
	}

	showConfigDialog := func() {
		if selectedDevice == nil {
			dialog.ShowInformation("No Printer", "Please select a printer first.", window)
			return
		}

		dev := selectedDevice

		protocolOptions := []string{"ESCPOS", "TSPL", "ZPL", "CPCL", "EPL", "StarPRNT"}
		protocolMap := map[string]printer.Protocol{
			"ESCPOS":   printer.ESCPOS,
			"TSPL":     printer.TSPL,
			"ZPL":      printer.ZPL,
			"CPCL":     printer.CPCL,
			"EPL":      printer.EPL,
			"StarPRNT": printer.StarPRNT,
		}

		transportOptions := []string{"TCP", "BluetoothClassic"}
		transportMap := map[string]printer.TransportKind{
			"TCP":              printer.TCP,
			"BluetoothClassic": printer.BluetoothClassic,
		}

		protocolSelect := widget.NewSelect(protocolOptions, nil)
		protocolSelect.SetSelected("ZPL")

		transportSelect := widget.NewSelect(transportOptions, nil)
		transportSelect.SetSelected("TCP")

		endpointEntry := widget.NewEntry()
		endpointEntry.SetText(dev.Endpoint)

		dpiEntry := widget.NewEntry()
		if dev.Profile.DPI > 0 {
			dpiEntry.SetText(fmt.Sprintf("%d", dev.Profile.DPI))
		} else {
			dpiEntry.SetText("203")
		}

		errorLabel := widget.NewLabel("")

		configWindow := application.NewWindow("Configure Printer")
		configWindow.Resize(fyne.NewSize(400, 350))

		applyButton := widget.NewButton("Apply", func() {
			errorLabel.SetText("")

			protocolStr := protocolSelect.Selected
			transportStr := transportSelect.Selected
			endpoint := endpointEntry.Text
			dpiStr := dpiEntry.Text

			if protocolStr == "" {
				errorLabel.SetText("Please select a protocol")
				return
			}
			if transportStr == "" {
				errorLabel.SetText("Please select a transport")
				return
			}
			if endpoint == "" {
				errorLabel.SetText("Endpoint cannot be empty")
				return
			}

			dpi, err := strconv.Atoi(dpiStr)
			if err != nil || dpi <= 0 {
				errorLabel.SetText("DPI must be a positive number")
				return
			}

			selectedProtocol := protocolMap[protocolStr]
			selectedTransport := transportMap[transportStr]

			cfgPrinter := &printer.Printer{
				ID:   dev.ID,
				Name: dev.Name,
				Connection: printer.Connection{
					Protocol:  selectedProtocol,
					Transport: selectedTransport,
					Endpoint:  endpoint,
					Options:   nil,
				},
				Profile: printer.PrinterProfile{
					Vendor:          dev.Profile.Vendor,
					Model:           dev.Profile.Model,
					DPI:             dpi,
					MediaType:       dev.Profile.MediaType,
					MediaWidth:      dev.Profile.MediaWidth,
					SupportsCutter:  dev.Profile.SupportsCutter,
					SupportsLabel:   dev.Profile.SupportsLabel,
					SupportsReceipt: dev.Profile.SupportsReceipt,
					MonochromeOnly:  dev.Profile.MonochromeOnly,
				},
			}

			if err := cfgPrinter.Validate(); err != nil {
				errorLabel.SetText(fmt.Sprintf("Validation error: %v", err))
				return
			}

			configuredPrinter = cfgPrinter
			updateConfiguredStatus()
			configWindow.Close()
		})

		cancelButton := widget.NewButton("Cancel", func() {
			configWindow.Close()
		})

		form := container.NewVBox(
			widget.NewLabelWithStyle("Protocol:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			protocolSelect,
			widget.NewLabelWithStyle("Transport:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			transportSelect,
			widget.NewLabelWithStyle("Endpoint:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			endpointEntry,
			widget.NewLabelWithStyle("DPI:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			dpiEntry,
			errorLabel,
			container.NewHBox(applyButton, cancelButton),
		)

		configWindow.SetContent(form)
		configWindow.Show()
	}

	_ = printButton
	printButton = widget.NewButton("Print", func() {
		if isPrinting {
			return
		}

		if configuredPrinter == nil {
			dialog.ShowInformation("No Printer", "Please configure a printer first.", window)
			return
		}

		if err := configuredPrinter.Validate(); err != nil {
			dialog.ShowError(fmt.Errorf("invalid printer: %w", err), window)
			return
		}

		printerStatus.SetText("Printing...")
		if updateCombinedStatusDisplay != nil {
			updateCombinedStatusDisplay()
		}
		isPrinting = true
		printButton.Disable()

		go func() {
			defer func() {
				isPrinting = false
				printButton.Enable()
			}()

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			if configuredPrinter.Connection.Transport == printer.BluetoothClassic {
				granted, err := platform.EnsureBluetoothConnectPermission(ctx)
				if err != nil {
					fyne.Do(func() {
						printerStatus.SetText(fmt.Sprintf("Permission error: %v", err))
						if updateCombinedStatusDisplay != nil {
							updateCombinedStatusDisplay()
						}
						dialog.ShowError(fmt.Errorf("bluetooth permission error: %w", err), window)
					})
					return
				}
				if !granted {
					fyne.Do(func() {
						printerStatus.SetText("Bluetooth permission denied")
						if updateCombinedStatusDisplay != nil {
							updateCombinedStatusDisplay()
						}
						dialog.ShowInformation("Permission Denied", "Bluetooth permission is required to print.", window)
					})
					return
				}
			}

			err := service.Print(ctx, *configuredPrinter, doc, renderer)
			if err != nil {
				fyne.Do(func() {
					printerStatus.SetText(fmt.Sprintf("Print failed: %v", err))
					if updateCombinedStatusDisplay != nil {
						updateCombinedStatusDisplay()
					}
					dialog.ShowError(fmt.Errorf("print failed: %w", err), window)
				})
				return
			}

			fyne.Do(func() {
				printerStatus.SetText("Print successful")
				if updateCombinedStatusDisplay != nil {
					updateCombinedStatusDisplay()
				}
				dialog.ShowInformation("Success", "Print job completed successfully.", window)
			})
		}()
	})

	refreshButton := widget.NewButton("Refresh", refreshDevices)
	configureButton := widget.NewButton("Configure", showConfigDialog)

	printerLabel := widget.NewLabelWithStyle("Printer:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	refreshDevices()

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

		androidCombinedStatus = widget.NewLabel("")
		androidCombinedStatus.Wrapping = fyne.TextWrapWord
		updateCombinedStatusDisplay()

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

	buildScanPrinterScreen := func() fyne.CanvasObject {
		var scanDevices []platform.Device
		var selectedScanDevice *platform.Device
		isScanning := false

		statusLabel := widget.NewLabel("Ready to scan")
		scanButton := widget.NewButton("Scan", nil)

		deviceList := widget.NewList(
			func() int {
				return len(scanDevices)
			},
			func() fyne.CanvasObject {
				return container.NewHBox(
					widget.NewLabel(""),
					widget.NewLabel(""),
				)
			},
			func(id widget.ListItemID, obj fyne.CanvasObject) {
				if id < 0 || id >= len(scanDevices) {
					return
				}
				dev := scanDevices[id]
				box := obj.(*fyne.Container)
				if len(box.Objects) >= 2 {
					nameLabel := box.Objects[0].(*widget.Label)
					endpointLabel := box.Objects[1].(*widget.Label)
					nameLabel.Text = dev.Name
					endpointLabel.Text = dev.Endpoint
					nameLabel.Refresh()
					endpointLabel.Refresh()
				}
			},
		)

		deviceList.OnSelected = func(id widget.ListItemID) {
			if id < 0 || id >= len(scanDevices) {
				return
			}
			selectedScanDevice = &scanDevices[id]
			statusLabel.SetText(fmt.Sprintf("Selected: %s (%s)", selectedScanDevice.Name, selectedScanDevice.Endpoint))
			deviceList.Refresh()
		}

		scanButton.OnTapped = func() {
			if isScanning {
				return
			}
			isScanning = true
			scanButton.Disable()
			statusLabel.SetText("Scanning...")
			selectedScanDevice = nil
			scanDevices = nil
			deviceList.Refresh()

			go func() {
				ctx := context.Background()
				discovered, err := discoverBluetoothPrinters(ctx, window)

				fyne.Do(func() {
					isScanning = false
					scanButton.Enable()

					if err != nil {
						statusLabel.SetText(fmt.Sprintf("Error: %v", err))
						dialog.ShowError(fmt.Errorf("discovery error: %w", err), window)
						return
					}

					scanDevices = discovered
					if len(scanDevices) == 0 {
						statusLabel.SetText("No printers found")
					} else {
						statusLabel.SetText(fmt.Sprintf("Found %d printer(s)", len(scanDevices)))
					}
					deviceList.Refresh()
				})
			}()
		}

		return container.NewBorder(
			container.NewHBox(
				widget.NewButton("Back", func() {
					window.SetContent(homeContent)
				}),
				widget.NewLabelWithStyle("Scan Printer", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			),
			nil, nil, nil,
			container.NewVBox(
				statusLabel,
				scanButton,
				container.NewVBox(
					widget.NewLabel("Printers:"),
					deviceList,
				),
			),
		)
	}

	buildSettingsScreen := func() fyne.CanvasObject {
		protocolOptions := []string{"ESCPOS", "TSPL", "ZPL", "CPCL", "EPL", "StarPRNT"}
		protocolSelect := widget.NewSelect(protocolOptions, nil)
		protocolSelect.SetSelected("ZPL")

		transportDisplayOptions := []string{"TCP", "Bluetooth Classic"}
		transportSelect := widget.NewSelect(transportDisplayOptions, nil)
		transportSelect.SetSelected("Bluetooth Classic")

		dpiEntry := widget.NewEntry()
		dpiEntry.SetText("203")
		dpiEntry.Validator = func(s string) error {
			if s == "" {
				return nil
			}
			_, err := strconv.Atoi(s)
			return err
		}

		widthEntry := widget.NewEntry()
		widthEntry.SetText("80")
		widthEntry.Validator = func(s string) error {
			if s == "" {
				return nil
			}
			_, err := strconv.Atoi(s)
			return err
		}

		heightEntry := widget.NewEntry()
		heightEntry.SetText("200")
		heightEntry.Validator = func(s string) error {
			if s == "" {
				return nil
			}
			_, err := strconv.Atoi(s)
			return err
		}

		header := container.NewHBox(
			widget.NewButton("Back", func() {
				window.SetContent(homeContent)
			}),
			widget.NewLabelWithStyle("Settings", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		)

		body := container.NewVBox(
			widget.NewLabelWithStyle("Printer", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			container.NewHBox(
				widget.NewLabel("Protocol"),
				protocolSelect,
			),
			container.NewHBox(
				widget.NewLabel("Transport"),
				transportSelect,
			),
			container.NewHBox(
				widget.NewLabel("DPI"),
				dpiEntry,
			),
			widget.NewSeparator(),
			widget.NewLabelWithStyle("Paper", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			container.NewHBox(
				widget.NewLabel("Width"),
				widthEntry,
				widget.NewLabel("mm"),
			),
			container.NewHBox(
				widget.NewLabel("Height"),
				heightEntry,
				widget.NewLabel("mm"),
			),
		)

		return container.NewBorder(header, nil, nil, nil, body)
	}

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
			}, window)
		}

		chooseButton.OnTapped = openFilePicker

		chooseAnotherButton.OnTapped = openFilePicker

		openInEditorButton.OnTapped = func() {
			if pendingFilePath != "" {
				if len(pendingFileData) > 0 {
					ed.AddImage(pendingFileData, "image/png", document.Point{X: 20_000, Y: 20_000}, document.Size{Width: 40_000, Height: 30_000})
					canvasWidget.Refresh()
				}
				updateCombinedStatusDisplay()
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

	buildHomeScreen := func() fyne.CanvasObject {
		title := canvas.NewText("PrintCat", color.Black)
		title.Alignment = fyne.TextAlignCenter
		title.TextSize = 28
		title.TextStyle = fyne.TextStyle{Bold: true}

		menu := container.NewVBox(
			widget.NewButton("Settings", func() {
				settingsContent := buildSettingsScreen()
				window.SetContent(settingsContent)
			}),
			widget.NewButton("Scan Printer", func() {
				scanContent := buildScanPrinterScreen()
				window.SetContent(scanContent)
			}),
			widget.NewButton("Add File", func() {
				addFileContent := buildAddFileScreen()
				window.SetContent(addFileContent)
			}),
			widget.NewButton("Blank Canvas", func() {
				pendingFilePath = ""
				pendingFileData = nil
				pendingFileInfo = struct{ Name, Path, Ext, Size string }{}
				updateCombinedStatusDisplay()
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

func formatDeviceDisplay(dev platform.Device) string {
	if dev.Name != "" {
		return fmt.Sprintf("%s (%s)", dev.Name, dev.Endpoint)
	}
	return fmt.Sprintf("Unknown (%s)", dev.Endpoint)
}
