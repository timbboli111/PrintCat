package fyneapp

import (
	"context"
	"fmt"
	"image"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/timboli111/PrintCat/internal/editor"
	"github.com/timboli111/PrintCat/internal/render"
	"github.com/timboli111/PrintCat/internal/render/basic"
)

func ShowPreview(app fyne.App, ed *editor.Editor, canvasWidget *CanvasWidget, printAction func()) {
	win := app.NewWindow("Print Preview")
	win.Resize(fyne.NewSize(800, 600))

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

	scale := 1.0
	scaleLabel := widget.NewLabel("100%")

	updateImage := func() {
		w := float32(float64(ras.Width) * scale)
		h := float32(float64(ras.Height) * scale)
		img.Resize(fyne.NewSize(w, h))
		img.Refresh()
		scaleLabel.SetText(strconv.Itoa(int(scale*100)) + "%")
	}

	fit := func() {
		winSize := win.Canvas().Size()
		availW := float64(winSize.Width - 20)
		availH := float64(winSize.Height - 80)
		if availW <= 0 || availH <= 0 {
			return
		}
		scaleW := availW / float64(ras.Width)
		scaleH := availH / float64(ras.Height)
		scale = scaleW
		if scaleH < scaleW {
			scale = scaleH
		}
		if scale < 0.1 {
			scale = 0.1
		}
		updateImage()
	}

	updateImage()

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
		updateImage()
	}

	scroll := container.NewScroll(img)

	toolbar := container.NewHBox(
		widget.NewButton("-", func() {
			scale /= 1.2
			if scale < 0.1 {
				scale = 0.1
			}
			updateImage()
		}),
		widget.NewButton("+", func() {
			scale *= 1.2
			if scale > 10.0 {
				scale = 10.0
			}
			updateImage()
		}),
		widget.NewButton("Fit", fit),
		scaleLabel,
		widget.NewButton("Rotate", rotate),
		widget.NewButton("Print", func() {
			if printAction != nil {
				printAction()
			}
		}),
		widget.NewButton("Close", win.Close),
	)

	content := container.NewBorder(toolbar, nil, nil, nil, scroll)
	win.SetContent(content)

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
