package fyneapp

import (
	"context"
	"fmt"
	"image"
	"math"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/timboli111/PrintCat/internal/editor"
	"github.com/timboli111/PrintCat/internal/render"
	"github.com/timboli111/PrintCat/internal/render/basic"
)

const previewFitInset = 16

// responsivePreviewLayout wraps toolbar controls into rows and assigns the
// remaining area to the scrollable page. Layout runs again when the window or
// device orientation changes, so Fit can follow the actual viewport size.
type responsivePreviewLayout struct {
	controls         []fyne.CanvasObject
	content          fyne.CanvasObject
	onContentResized func(fyne.Size)
}

func (l *responsivePreviewLayout) MinSize(_ []fyne.CanvasObject) fyne.Size {
	minWidth := float32(0)
	minControlHeight := float32(0)
	for _, control := range l.controls {
		minSize := control.MinSize()
		minWidth = maxFloat32(minWidth, minSize.Width)
		minControlHeight = maxFloat32(minControlHeight, minSize.Height)
	}
	return fyne.NewSize(minWidth, minControlHeight+theme.Padding()+64)
}

func (l *responsivePreviewLayout) Layout(_ []fyne.CanvasObject, size fyne.Size) {
	padding := theme.Padding()
	var x, y, rowHeight float32
	for _, control := range l.controls {
		controlSize := control.MinSize()
		if x > 0 && x+controlSize.Width > size.Width {
			y += rowHeight + padding
			x = 0
			rowHeight = 0
		}

		control.Move(fyne.NewPos(x, y))
		control.Resize(controlSize)
		x += controlSize.Width + padding
		rowHeight = maxFloat32(rowHeight, controlSize.Height)
	}

	toolbarHeight := y + rowHeight
	if len(l.controls) > 0 {
		toolbarHeight += padding
	}
	contentHeight := maxFloat32(0, size.Height-toolbarHeight)
	l.content.Move(fyne.NewPos(0, toolbarHeight))
	l.content.Resize(fyne.NewSize(size.Width, contentHeight))
	if l.onContentResized != nil {
		l.onContentResized(l.content.Size())
	}
}

func maxFloat32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func previewFitScale(viewport, raster fyne.Size) float64 {
	availableWidth := float64(viewport.Width) - previewFitInset
	availableHeight := float64(viewport.Height) - previewFitInset
	if availableWidth <= 0 || availableHeight <= 0 || raster.Width <= 0 || raster.Height <= 0 {
		return 0
	}
	return math.Min(availableWidth/float64(raster.Width), availableHeight/float64(raster.Height))
}

func ShowPreview(app fyne.App, ed *editor.Editor, canvasWidget *CanvasWidget, printAction func()) {
	win := app.NewWindow("Print Preview")
	// Keep the established desktop preview size. Mobile windows use the screen
	// area assigned by Fyne instead of requesting a desktop-sized window.
	if !app.Driver().Device().IsMobile() {
		win.Resize(fyne.NewSize(800, 600))
	}

	renderer := &basic.Renderer{}
	ras, err := renderPreview(renderer, ed)
	if err != nil {
		errLabel := widget.NewLabel("Preview error: " + err.Error())
		win.SetContent(container.NewVBox(
			errLabel,
			widget.NewButton("Close", win.Close),
		))
		win.Show()
		return
	}

	img := canvas.NewImageFromImage(rasterImage(ras))
	img.FillMode = canvas.ImageFillOriginal
	scroll := container.NewScroll(img)

	scale := 1.0
	fitMode := true
	scaleLabel := widget.NewLabel("100%")

	updateImage := func() {
		w := float32(float64(ras.Width) * scale)
		h := float32(float64(ras.Height) * scale)
		img.Resize(fyne.NewSize(w, h))
		img.Refresh()
		scaleLabel.SetText(strconv.Itoa(int(scale*100)) + "%")
	}

	fitToViewport := func(viewSize fyne.Size) {
		nextScale := previewFitScale(viewSize, fyne.NewSize(float32(ras.Width), float32(ras.Height)))
		if nextScale <= 0 || math.Abs(nextScale-scale) < 0.0001 {
			return
		}
		scale = nextScale
		updateImage()
	}

	minusButton := widget.NewButton("-", func() {
		fitMode = false
		scale /= 1.2
		if scale < 0.1 {
			scale = 0.1
		}
		updateImage()
	})
	plusButton := widget.NewButton("+", func() {
		fitMode = false
		scale *= 1.2
		if scale > 10.0 {
			scale = 10.0
		}
		updateImage()
	})
	fitButton := widget.NewButton("Fit", func() {
		fitMode = true
		fitToViewport(scroll.Size())
	})

	rotate := func() {
		selected := ed.SelectedElement()
		if selected == nil {
			dialog.ShowInformation("No Image Selected", "Select an image on the canvas before rotating.", win)
			return
		}
		id := selected.ID
		if err := ed.RotateSelectedImage(); err != nil {
			dialog.ShowError(err, win)
			return
		}

		canvasWidget.InvalidateObject(id)
		canvasWidget.Refresh()

		latest, err := renderPreview(renderer, ed)
		if err != nil {
			dialog.ShowError(err, win)
			return
		}
		ras = latest
		img.Image = rasterImage(ras)
		if fitMode {
			fitToViewport(scroll.Size())
		} else {
			updateImage()
		}
	}

	rotateButton := widget.NewButton("Rotate", rotate)
	printButton := widget.NewButton("Print", func() {
		if printAction != nil {
			printAction()
		}
	})
	closeButton := widget.NewButton("Close", win.Close)

	controls := []fyne.CanvasObject{
		minusButton,
		plusButton,
		fitButton,
		scaleLabel,
		rotateButton,
		printButton,
		closeButton,
	}
	layout := &responsivePreviewLayout{
		controls: controls,
		content:  scroll,
		onContentResized: func(size fyne.Size) {
			if fitMode {
				fitToViewport(size)
			}
		},
	}
	children := append([]fyne.CanvasObject(nil), controls...)
	children = append(children, scroll)
	win.SetContent(container.New(layout, children...))
	win.Show()
}

func renderPreview(renderer *basic.Renderer, ed *editor.Editor) (render.Raster, error) {
	ras, err := renderer.Render(context.Background(), *ed.Doc, render.Target{DPI: 96, Monochrome: false})
	if err != nil {
		return render.Raster{}, err
	}
	if len(ras.Pixels) != ras.Width*ras.Height {
		return render.Raster{}, fmt.Errorf("preview error: invalid raster data")
	}
	return ras, nil
}

func rasterImage(ras render.Raster) *image.Gray {
	return &image.Gray{
		Pix:    ras.Pixels,
		Stride: ras.Width,
		Rect:   image.Rect(0, 0, ras.Width, ras.Height),
	}
}
