package bridge

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"path/filepath"
	"testing"

	"github.com/timboli111/PrintCat/internal/config"
	"github.com/timboli111/PrintCat/internal/document"
	"github.com/timboli111/PrintCat/internal/printer"
	"github.com/timboli111/PrintCat/internal/printer/escpos"
	"github.com/timboli111/PrintCat/internal/render"
	"github.com/timboli111/PrintCat/internal/render/basic"
)

type fakeBackend struct{ protocol printer.Protocol }

func (b *fakeBackend) Protocol() printer.Protocol { return b.protocol }

func (b *fakeBackend) Encode(_ context.Context, doc document.Document, _ render.Renderer, _ printer.PrinterProfile, _ map[string]string) ([]byte, error) {
	return []byte(fmt.Sprintf("fake-page-%s-%dx%d",
		doc.ID, doc.PageSize.Width, doc.PageSize.Height)), nil
}

type fakeRenderer struct{}

func (f *fakeRenderer) Render(_ context.Context, _ document.Document, _ render.Target) (render.Raster, error) {
	return render.Raster{Width: 1, Height: 1, Pixels: []byte{0}}, nil
}

func newTestState(t *testing.T) (*State, *printer.MemoryTransport) {
	t.Helper()
	svc := printer.NewService()
	if err := svc.RegisterBackend(&fakeBackend{protocol: printer.ZPL}); err != nil {
		t.Fatalf("RegisterBackend: %v", err)
	}
	tr := &printer.MemoryTransport{TransportKind: printer.TCP}
	if err := svc.RegisterTransport(tr); err != nil {
		t.Fatalf("RegisterTransport: %v", err)
	}
	return NewState(svc, &fakeRenderer{}), tr
}

func TestSubmitRequiresActivePrinter(t *testing.T) {
	s, _ := newTestState(t)
	err := s.Submit(context.Background(), SubmitRequest{
		Pages:         [][]byte{{1, 2, 3}},
		PageWidthsUm:  []int64{1000},
		PageHeightsUm: []int64{1000},
		DPI:           203,
	})
	if err == nil {
		t.Fatal("expected error when no active printer")
	}
}

func TestSubmitSendsOneCallPerPage(t *testing.T) {
	s, tr := newTestState(t)
	p := &printer.Printer{
		ID:         "test",
		Name:       "Test Printer",
		Connection: printer.Connection{Protocol: printer.ZPL, Transport: printer.TCP},
		Profile:    printer.PrinterProfile{DPI: 203},
	}
	s.SetActivePrinter(p)

	req := SubmitRequest{
		Pages:         [][]byte{{0xAA}, {0xBB}, {0xCC}},
		PageWidthsUm:  []int64{10000, 10000, 10000},
		PageHeightsUm: []int64{10000, 10000, 10000},
		DPI:           203,
		JobID:         "android-job-abc",
	}
	if err := s.Submit(context.Background(), req); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if len(tr.Payloads) != 3 {
		t.Fatalf("expected 3 payloads, got %d", len(tr.Payloads))
	}
	if len(tr.JobDiagnostics) != 3 {
		t.Fatalf("expected diagnostics for 3 pages, got %d", len(tr.JobDiagnostics))
	}
	for i, info := range tr.JobDiagnostics {
		if info.JobID != req.JobID || info.Source != printer.SourceAndroidPrintFramework {
			t.Errorf("page %d diagnostics = %+v", i+1, info)
		}
		if info.Page != i+1 || info.PageCount != len(req.Pages) {
			t.Errorf("page %d metadata = %d/%d", i+1, info.Page, info.PageCount)
		}
	}
}

func TestSubmitRejectsMismatchedMetadata(t *testing.T) {
	s, _ := newTestState(t)
	p := &printer.Printer{
		ID:         "test",
		Connection: printer.Connection{Protocol: printer.ZPL, Transport: printer.TCP},
		Profile:    printer.PrinterProfile{DPI: 203},
	}
	s.SetActivePrinter(p)

	err := s.Submit(context.Background(), SubmitRequest{
		Pages:         [][]byte{{}, {}},
		PageWidthsUm:  []int64{1000},
		PageHeightsUm: []int64{1000, 1000},
		DPI:           203,
	})
	if err == nil {
		t.Fatal("expected error for mismatched metadata")
	}
}

func TestSubmitRejectsInvalidDPI(t *testing.T) {
	s, _ := newTestState(t)
	p := &printer.Printer{
		Connection: printer.Connection{Protocol: printer.ZPL, Transport: printer.TCP},
		Profile:    printer.PrinterProfile{DPI: 203},
	}
	s.SetActivePrinter(p)

	err := s.Submit(context.Background(), SubmitRequest{
		Pages:         [][]byte{{0xAA}},
		PageWidthsUm:  []int64{1000},
		PageHeightsUm: []int64{1000},
		DPI:           0,
	})
	if err == nil {
		t.Fatal("expected error for invalid DPI")
	}
}

// TestSubmitSinglePageHasNoSettleDelay proves the multi-page settle delay
// is not applied to single-page jobs. The test is deterministic: it relies
// on a context timeout that is strictly shorter than multiPageSettleDelay.
// If any delay were introduced before page 0 (or on the single-page path),
// the context would expire during that wait and Submit would return an
// error instead of success. No wall-clock measurement is used, so the
// result does not depend on how busy the test machine is.
func TestSubmitSinglePageHasNoSettleDelay(t *testing.T) {
	s, tr := newTestState(t)
	p := &printer.Printer{
		ID:         "test",
		Connection: printer.Connection{Protocol: printer.ZPL, Transport: printer.TCP},
		Profile:    printer.PrinterProfile{DPI: 203},
	}
	s.SetActivePrinter(p)

	ctx, cancel := context.WithTimeout(context.Background(),
		multiPageSettleDelay/5)
	defer cancel()

	err := s.Submit(ctx, SubmitRequest{
		Pages:         [][]byte{{0xAA}},
		PageWidthsUm:  []int64{10000},
		PageHeightsUm: []int64{10000},
		DPI:           203,
	})
	if err != nil {
		t.Fatalf("Submit: %v (single-page must not wait for settle delay)", err)
	}
	if len(tr.Payloads) != 1 {
		t.Fatalf("expected 1 payload, got %d", len(tr.Payloads))
	}
}

// TestSubmitMultiPageHonorsContextCancellation proves the settle delay
// between pages is cancellable: a two-page job with a context deadline
// shorter than multiPageSettleDelay must return DeadlineExceeded after
// sending only the first page. This also confirms the delay is actually
// awaited between pages, since without it both pages would be sent well
// within the deadline.
func TestSubmitMultiPageHonorsContextCancellation(t *testing.T) {
	s, tr := newTestState(t)
	p := &printer.Printer{
		ID:         "test",
		Connection: printer.Connection{Protocol: printer.ZPL, Transport: printer.TCP},
		Profile:    printer.PrinterProfile{DPI: 203},
	}
	s.SetActivePrinter(p)

	ctx, cancel := context.WithTimeout(context.Background(),
		multiPageSettleDelay/5)
	defer cancel()

	err := s.Submit(ctx, SubmitRequest{
		Pages:         [][]byte{{0xAA}, {0xBB}},
		PageWidthsUm:  []int64{10000, 10000},
		PageHeightsUm: []int64{10000, 10000},
		DPI:           203,
	})
	if err == nil {
		t.Fatal("expected context deadline error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected DeadlineExceeded, got %v", err)
	}
	if len(tr.Payloads) != 1 {
		t.Errorf("expected 1 payload sent before cancellation, got %d",
			len(tr.Payloads))
	}
}

func TestSubmitTrimsEachPDFPageIndependently(t *testing.T) {
	svc := printer.NewService()
	if err := svc.RegisterBackend(&escpos.Encoder{}); err != nil {
		t.Fatal(err)
	}
	transport := &printer.MemoryTransport{TransportKind: printer.TCP}
	if err := svc.RegisterTransport(transport); err != nil {
		t.Fatal(err)
	}
	state := NewState(svc, &basic.Renderer{})
	wantPrinter := printer.Printer{
		ID:         "trim-test-printer",
		Name:       "Trim test printer",
		Connection: printer.Connection{Protocol: printer.ESCPOS, Transport: printer.TCP, Endpoint: "test"},
		Profile: printer.PrinterProfile{
			DPI:                     254,
			SmartTrimEnabled:        true,
			SmartTrimBottomMarginUm: 100,
		},
	}
	cfg := config.Default()
	cfg.SavedPrinters = []config.SavedPrinter{{
		Printer:       wantPrinter,
		PaperWidthMm:  1,
		PaperHeightMm: 1,
	}}
	cfg.ActivePrinterID = wantPrinter.ID
	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := config.Save(configPath, cfg); err != nil {
		t.Fatal(err)
	}
	loadActivePrinterFromConfig(state, configPath)
	gotPrinter := state.ActivePrinter()
	if gotPrinter == nil || !gotPrinter.Profile.SmartTrimEnabled ||
		gotPrinter.Profile.SmartTrimBottomMarginUm != wantPrinter.Profile.SmartTrimBottomMarginUm {
		t.Fatalf("active profile loaded from config = %+v, want %+v", gotPrinter, wantPrinter.Profile)
	}

	pagePNG := func(width, height, lastInkRow int) []byte {
		t.Helper()
		img := image.NewGray(image.Rect(0, 0, width, height))
		for i := range img.Pix {
			img.Pix[i] = 255
		}
		img.SetGray(1, lastInkRow, color.Gray{Y: 0})
		img.SetGray(2, lastInkRow, color.Gray{Y: 0})
		var encoded bytes.Buffer
		if err := png.Encode(&encoded, img); err != nil {
			t.Fatal(err)
		}
		return encoded.Bytes()
	}
	err := state.Submit(context.Background(), SubmitRequest{
		Pages:         [][]byte{pagePNG(8, 10, 2), pagePNG(8, 12, 4)},
		PageWidthsUm:  []int64{800, 800},
		PageHeightsUm: []int64{1000, 1200},
		DPI:           254,
		JobID:         "smart-trim-multipage",
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if len(transport.Payloads) != 2 {
		t.Fatalf("payload count=%d, want 2", len(transport.Payloads))
	}
	wantHeights := []int{4, 6}
	for i, payload := range transport.Payloads {
		widthBytes, height := rasterDimensionsFromPayload(t, payload)
		if widthBytes != 1 || height != wantHeights[i] {
			t.Errorf("page %d raster header=%dx%d, want 1x%d", i+1, widthBytes, height, wantHeights[i])
		}
		if got, want := len(payload), 10+widthBytes*height; got != want {
			t.Errorf("page %d payload length=%d, header declares %d raster bytes plus 10-byte prefix", i+1, got, want-10)
		}
	}
}

func rasterDimensionsFromPayload(t *testing.T, payload []byte) (widthBytes, height int) {
	t.Helper()
	for i := 0; i+7 < len(payload); i++ {
		if payload[i] == 0x1D && payload[i+1] == 0x76 && payload[i+2] == 0x30 {
			widthBytes = int(payload[i+4]) | (int(payload[i+5]) << 8)
			height = int(payload[i+6]) | (int(payload[i+7]) << 8)
			return widthBytes, height
		}
	}
	t.Fatal("GS v 0 raster header not found")
	return 0, 0
}
