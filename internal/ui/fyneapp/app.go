package fyneapp

import (
	"context"
	"fmt"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"runtime"
	"strconv"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
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
	return app.NewWithID(appID)
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

	selectedLabel := widget.NewLabel("Selected: none")
	canvasWidget := NewCanvas(ed, func(id string) {
		if id == "" {
			selectedLabel.SetText("Selected: none")
		} else {
			selectedLabel.SetText(fmt.Sprintf("Selected: %s", id))
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

	addText := widget.NewButton("Add Text", func() {
		pos := document.Point{X: 10_000, Y: 10_000}
		size := document.Size{Width: 40_000, Height: 10_000}
		ed.AddText("Hello", pos, size, 12_000)
		canvasWidget.Refresh()
	})

	addImage := widget.NewButton("Add Image", func() {
		dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			defer reader.Close()
			data, err := os.ReadFile(reader.URI().Path())
			if err != nil {
				return
			}
			ed.AddImage(data, "image/png", document.Point{X: 20_000, Y: 20_000}, document.Size{Width: 40_000, Height: 30_000})
			canvasWidget.Refresh()
		}, window)
	})

	deleteButton := widget.NewButton("Delete", func() {
		ed.DeleteSelected()
		canvasWidget.Refresh()
	})

	previewButton := widget.NewButton("Preview", func() {
		ShowPreview(application, ed)
	})

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
				return
			}
		}
	})
	printerSelect.PlaceHolder = "No printer found"

	updateConfiguredStatus := func() {
		if configuredPrinter == nil {
			configuredStatus.SetText("Not configured")
			return
		}
		configuredStatus.SetText(fmt.Sprintf(
			"Configured: %s / %s (DPI: %d)",
			configuredPrinter.Connection.Protocol,
			configuredPrinter.Connection.Transport,
			configuredPrinter.Profile.DPI,
		))
	}

	refreshDevices := func() {
		if isRefreshing {
			return
		}
		isRefreshing = true
		printerSelect.Disable()
		printerSelect.Refresh()
		printerStatus.SetText("Scanning...")

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

			if runtime.GOOS == "android" {
				if platform.GetAndroidAPIVersion() >= 31 {
					connectGranted, err := platform.EnsureBluetoothConnectPermission(ctx)
					if err != nil {
						fyne.Do(func() {
							printerStatus.SetText(fmt.Sprintf("Permission error: %v", err))
							dialog.ShowError(fmt.Errorf("bluetooth connect permission error: %w", err), window)
						})
						return
					}
					if !connectGranted {
						fyne.Do(func() {
							printerStatus.SetText("Bluetooth connect permission denied")
							dialog.ShowInformation("Permission Denied", "Bluetooth connect permission is required to discover printers.", window)
						})
						return
					}
				}

				scanGranted, err := platform.EnsureBluetoothScanPermission(ctx)
				if err != nil {
					fyne.Do(func() {
						printerStatus.SetText(fmt.Sprintf("Permission error: %v", err))
						dialog.ShowError(fmt.Errorf("bluetooth scan permission error: %w", err), window)
					})
					return
				}
				if !scanGranted {
					fyne.Do(func() {
						printerStatus.SetText("Bluetooth scan permission denied")
						dialog.ShowInformation("Permission Denied", "Bluetooth scan permission is required to discover printers.", window)
					})
					return
				}
			}

			integration := platform.GetIntegration()

			var targetKind printer.TransportKind
			if runtime.GOOS == "windows" {
				targetKind = printer.Serial
			} else if runtime.GOOS == "android" {
				targetKind = printer.BluetoothClassic
			} else {
				targetKind = ""
			}

			discovered, err := integration.Discover(ctx, targetKind)
			if err != nil {
				fyne.Do(func() {
					printerStatus.SetText(fmt.Sprintf("Error: %v", err))
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
						dialog.ShowError(fmt.Errorf("bluetooth permission error: %w", err), window)
					})
					return
				}
				if !granted {
					fyne.Do(func() {
						printerStatus.SetText("Bluetooth permission denied")
						dialog.ShowInformation("Permission Denied", "Bluetooth permission is required to print.", window)
					})
					return
				}
			}

			err := service.Print(ctx, *configuredPrinter, doc, renderer)
			if err != nil {
				fyne.Do(func() {
					printerStatus.SetText(fmt.Sprintf("Print failed: %v", err))
					dialog.ShowError(fmt.Errorf("print failed: %w", err), window)
				})
				return
			}

			fyne.Do(func() {
				printerStatus.SetText("Print successful")
				dialog.ShowInformation("Success", "Print job completed successfully.", window)
			})
		}()
	})

	refreshButton := widget.NewButton("Refresh", refreshDevices)
	configureButton := widget.NewButton("Configure", showConfigDialog)

	printerLabel := widget.NewLabelWithStyle("Printer:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	printerBox := container.NewVBox(
		printerLabel,
		container.NewHBox(printerSelect, refreshButton),
		container.NewHBox(configureButton, printButton),
		printerStatus,
		configuredStatus,
	)

	refreshDevices()

	topBar := container.NewHBox(
		widget.NewLabel("PrintCat"),
		widget.NewLabel("|"),
		widget.NewLabel("Paper:"), paperWidth, widget.NewLabel("mm x"), paperHeight, widget.NewLabel("mm"), paperApply,
		widget.NewLabel("| Zoom:"), zoomOut, zoomIn, fitButton, zoomLabel,
	)

	leftPanel := container.NewVBox(
		widget.NewLabel("Tools"),
		addText,
		addImage,
		deleteButton,
		previewButton,
		widget.NewSeparator(),
		printerBox,
		widget.NewSeparator(),
		selectedLabel,
		widget.NewLabel("(drag to move)"),
	)

	content := container.NewBorder(topBar, nil, leftPanel, nil, scroll)

	footerText1 := canvas.NewText("© 2026 PrintCat — Printing Tool by Pram", theme.ForegroundColor())
	footerText1.Alignment = fyne.TextAlignCenter
	footerText1.TextSize = 14

	footerText2 := canvas.NewText("Dedicated to my beloved wife, Apdini Nurrayani", color.Gray{Y: 170})
	footerText2.Alignment = fyne.TextAlignCenter
	footerText2.TextSize = 11

	footer := container.NewCenter(
		container.NewVBox(
			footerText1,
			footerText2,
		),
	)

	fullLayout := container.NewBorder(nil, footer, nil, nil, content)

	window.SetContent(fullLayout)
	return window
}

func formatDeviceDisplay(dev platform.Device) string {
	if dev.Name != "" {
		return fmt.Sprintf("%s (%s)", dev.Name, dev.Endpoint)
	}
	return fmt.Sprintf("Unknown (%s)", dev.Endpoint)
}
