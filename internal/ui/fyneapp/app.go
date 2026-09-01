package fyneapp

import (
	"fmt"
	_ "image/jpeg"
	_ "image/png"
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/timboli111/PrintCat/internal/document"
	"github.com/timboli111/PrintCat/internal/editor"
)

const appID = "com.printcat.app"

func New() fyne.App {
	return app.NewWithID(appID)
}

func NewWindow(application fyne.App) fyne.Window {
	window := application.NewWindow("PrintCat")
	window.Resize(fyne.NewSize(1100, 700))

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
		selectedLabel,
		widget.NewLabel("(drag to move)"),
	)

	content := container.NewBorder(topBar, nil, leftPanel, nil, scroll)

	window.SetContent(content)
	return window
}
