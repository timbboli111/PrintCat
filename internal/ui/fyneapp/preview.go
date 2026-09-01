package fyneapp

import (
	"context"
	"image"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/timboli111/PrintCat/internal/editor"
	"github.com/timboli111/PrintCat/internal/render"
	"github.com/timboli111/PrintCat/internal/render/basic"
)

func ShowPreview(app fyne.App, ed *editor.Editor) {
	win := app.NewWindow("Print Preview")
	win.Resize(fyne.NewSize(800, 600))

	renderer := &basic.Renderer{}
	ras, err := renderer.Render(context.Background(), *ed.Doc, render.Target{DPI: 96, Monochrome: false})
	if err != nil {
		errLabel := widget.NewLabel("Preview error: " + err.Error())
		win.SetContent(container.NewVBox(
			errLabel,
			widget.NewButton("Close", win.Close),
		))
		win.Show()
		return
	}

	if len(ras.Pixels) != ras.Width*ras.Height {
		errLabel := widget.NewLabel("Preview error: invalid raster data")
		win.SetContent(container.NewVBox(
			errLabel,
			widget.NewButton("Close", win.Close),
		))
		win.Show()
		return
	}

	gray := &image.Gray{
		Pix:    ras.Pixels,
		Stride: ras.Width,
		Rect:   image.Rect(0, 0, ras.Width, ras.Height),
	}

	img := canvas.NewImageFromImage(gray)
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
		widget.NewButton("Close", win.Close),
	)

	content := container.NewBorder(toolbar, nil, nil, nil, scroll)
	win.SetContent(content)

	win.Show()
}
