package bridge

import (
	"context"
	"fmt"
	"testing"

	"github.com/timboli111/PrintCat/internal/document"
	"github.com/timboli111/PrintCat/internal/printer"
	"github.com/timboli111/PrintCat/internal/render"
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
	}
	if err := s.Submit(context.Background(), req); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if len(tr.Payloads) != 3 {
		t.Fatalf("expected 3 payloads, got %d", len(tr.Payloads))
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
