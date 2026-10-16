package fyneapp

import (
	"image/color"
	"math"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/timboli111/PrintCat/internal/document"
	"github.com/timboli111/PrintCat/internal/editor"
)

func TestPreviewFitScaleUsesAvailableViewport(t *testing.T) {
	raster := fyne.NewSize(800, 1000)
	portrait := previewFitScale(fyne.NewSize(320, 420), raster)
	landscape := previewFitScale(fyne.NewSize(800, 320), raster)

	if math.Abs(portrait-0.38) > 0.001 {
		t.Fatalf("portrait fit scale = %.3f, want 0.380", portrait)
	}
	if math.Abs(landscape-0.304) > 0.001 {
		t.Fatalf("landscape fit scale = %.3f, want 0.304", landscape)
	}
	if previewFitScale(fyne.NewSize(12, 12), raster) != 0 {
		t.Fatal("fit scale should be zero when the viewport cannot contain the inset")
	}
}

func TestResponsivePreviewLayoutWrapsToolbarAndResizesCanvas(t *testing.T) {
	application := test.NewApp()
	defer application.Quit()

	controls := []fyne.CanvasObject{
		widget.NewButton("-", nil),
		widget.NewButton("+", nil),
		widget.NewButton("Fit", nil),
		widget.NewLabel("100%"),
		widget.NewButton("Rotate", nil),
		widget.NewButton("Print", nil),
		widget.NewButton("Close", nil),
	}
	scroll := container.NewScroll(canvas.NewRectangle(color.White))
	var viewports []fyne.Size
	var fitScales []float64
	raster := fyne.NewSize(800, 1000)
	previewLayout := &responsivePreviewLayout{
		controls: controls,
		content:  scroll,
		onContentResized: func(size fyne.Size) {
			viewports = append(viewports, size)
			fitScales = append(fitScales, previewFitScale(size, raster))
		},
	}
	children := append([]fyne.CanvasObject(nil), controls...)
	children = append(children, scroll)
	content := container.New(previewLayout, children...)

	content.Resize(fyne.NewSize(320, 568))
	portraitRows := toolbarRows(t, controls, 320, scroll.Position().Y)
	portraitViewport := scroll.Size()
	if portraitRows < 2 {
		t.Fatalf("portrait toolbar rows = %d, want wrapping across at least two rows", portraitRows)
	}
	if portraitViewport.Width <= 0 || portraitViewport.Height <= 0 {
		t.Fatalf("portrait preview viewport is empty: %v", portraitViewport)
	}

	content.Resize(fyne.NewSize(568, 320))
	landscapeRows := toolbarRows(t, controls, 568, scroll.Position().Y)
	landscapeViewport := scroll.Size()
	if landscapeRows >= portraitRows {
		t.Fatalf("landscape toolbar rows = %d, portrait = %d; expected fewer rows in wider viewport", landscapeRows, portraitRows)
	}
	if landscapeViewport.Width <= 0 || landscapeViewport.Height <= 0 {
		t.Fatalf("landscape preview viewport is empty: %v", landscapeViewport)
	}
	if fitScales[len(fitScales)-1] <= 0 || fitScales[len(fitScales)-1] == fitScales[0] {
		t.Fatalf("fit scale did not adapt to viewport resize: %v", fitScales)
	}

	content.Resize(fyne.NewSize(800, 600))
	if rows := toolbarRows(t, controls, 800, scroll.Position().Y); rows != 1 {
		t.Fatalf("desktop toolbar rows = %d, want one row at the established desktop width", rows)
	}
	if len(viewports) < 3 {
		t.Fatalf("layout resize callback count = %d, want at least 3", len(viewports))
	}
}

func TestShowPreviewFitsOnOpenAndAfterResize(t *testing.T) {
	application := test.NewApp()
	defer application.Quit()

	doc := document.New("preview", "Preview", document.Size{Width: 300_000, Height: 500_000})
	ed := editor.New(&doc)
	canvasWidget := NewCanvas(ed, nil)
	printed := false
	ShowPreview(application, ed, canvasWidget, func() { printed = true })

	var window fyne.Window
	for _, candidate := range application.Driver().AllWindows() {
		if candidate.Title() == "Print Preview" {
			window = candidate
			break
		}
	}
	if window == nil {
		t.Fatal("preview window was not created")
	}
	assertPreviewImageFits(t, window)

	window.Resize(fyne.NewSize(320, 568))
	assertPreviewImageFits(t, window)
	window.Resize(fyne.NewSize(568, 320))
	assertPreviewImageFits(t, window)

	root := window.Content().(*fyne.Container)
	printButton, ok := root.Objects[5].(*widget.Button)
	if !ok {
		t.Fatal("Print control is not a button")
	}
	test.Tap(printButton)
	if !printed {
		t.Fatal("Print control did not invoke the print action")
	}
}

func assertPreviewImageFits(t *testing.T, window fyne.Window) {
	t.Helper()
	root, ok := window.Content().(*fyne.Container)
	if !ok || len(root.Objects) != 8 {
		t.Fatalf("preview content = %T, want 7 controls and one scroll area", window.Content())
	}
	scroll, ok := root.Objects[7].(*container.Scroll)
	if !ok {
		t.Fatalf("preview canvas = %T, want scroll container", root.Objects[7])
	}
	img, ok := scroll.Content.(*canvas.Image)
	if !ok {
		t.Fatalf("preview scroll content = %T, want image", scroll.Content)
	}
	if scroll.Size().Width <= 0 || scroll.Size().Height <= 0 {
		t.Fatalf("preview viewport is empty: %v", scroll.Size())
	}
	if img.Size().Width > scroll.Size().Width || img.Size().Height > scroll.Size().Height {
		t.Fatalf("Fit image %v exceeds viewport %v", img.Size(), scroll.Size())
	}
	toolbarBottom := scroll.Position().Y
	for _, control := range root.Objects[:7] {
		position := control.Position()
		size := control.Size()
		if position.X < 0 || position.X+size.Width > scroll.Size().Width+0.1 ||
			position.Y < 0 || position.Y+size.Height > toolbarBottom+0.1 {
			t.Errorf("toolbar control is outside visible toolbar bounds: position=%v size=%v", position, size)
		}
	}
}

func toolbarRows(t *testing.T, controls []fyne.CanvasObject, viewportWidth, toolbarBottom float32) int {
	t.Helper()
	rowY := make(map[int]struct{})
	for _, control := range controls {
		position := control.Position()
		size := control.Size()
		if position.X < 0 || position.X+size.Width > viewportWidth+0.1 {
			t.Errorf("toolbar control exceeds width: position=%v size=%v viewport=%g", position, size, viewportWidth)
		}
		if position.Y < 0 || position.Y+size.Height > toolbarBottom+0.1 {
			t.Errorf("toolbar control exceeds toolbar area: position=%v size=%v toolbar bottom=%g", position, size, toolbarBottom)
		}
		rowY[int(math.Round(float64(position.Y)))] = struct{}{}
	}
	return len(rowY)
}
