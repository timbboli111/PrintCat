package printer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestAnalyzePayloadReportsESCPOSRasterHeader(t *testing.T) {
	payload := make([]byte, 10+4*288+3)
	copy(payload, []byte{0x1b, 0x40, 0x1d, 0x76, 0x30, 0x00, 0x04, 0x00, 0x20, 0x01})
	got := AnalyzePayload(ESCPOS, payload)
	wantDigest := sha256.Sum256(payload)
	if got.PayloadBytes != len(payload) {
		t.Errorf("PayloadBytes = %d, want %d", got.PayloadBytes, len(payload))
	}
	if got.SHA256 != hex.EncodeToString(wantDigest[:]) {
		t.Errorf("SHA256 = %s, want %s", got.SHA256, hex.EncodeToString(wantDigest[:]))
	}
	if !got.HasRasterHeader {
		t.Fatal("HasRasterHeader = false, want true")
	}
	if got.RasterWidthBytes != 4 || got.RasterWidthDotsPadded != 32 || got.RasterHeightDots != 288 {
		t.Errorf("raster dimensions = %d bytes, %d x %d dots; want 4 bytes, 32 x 288",
			got.RasterWidthBytes, got.RasterWidthDotsPadded, got.RasterHeightDots)
	}
	if got.RasterHeaderHex != "1d 76 30 00 04 00 20 01" {
		t.Errorf("RasterHeaderHex = %q", got.RasterHeaderHex)
	}
	if got.RasterBytesDeclared != int64(4*288) || got.BytesAfterRasterHeader != int64(4*288+3) ||
		got.TrailingBytesAfterRaster != 3 || got.RasterDataIncomplete {
		t.Errorf("raster byte accounting = %+v", got)
	}
}

func TestAnalyzePayloadDoesNotTreatOtherPayloadsAsRaster(t *testing.T) {
	payload := []byte{0x1b, 0x40, 0x1d, 0x76, 0x30, 0, 2, 0, 1, 0}
	if got := AnalyzePayload(ZPL, payload); got.HasRasterHeader {
		t.Fatal("ZPL payload unexpectedly parsed as ESC/POS raster")
	}
	if got := AnalyzePayload(ESCPOS, payload[:9]); got.HasRasterHeader {
		t.Fatal("truncated ESC/POS header unexpectedly parsed as raster")
	}
	truncated := []byte{0x1b, 0x40, 0x1d, 0x76, 0x30, 0, 2, 0, 8, 0, 0xaa}
	if got := AnalyzePayload(ESCPOS, truncated); !got.RasterDataIncomplete {
		t.Fatal("short raster payload was not flagged as incomplete")
	}
	maxHeader := []byte{0x1b, 0x40, 0x1d, 0x76, 0x30, 0, 0xff, 0xff, 0xff, 0xff}
	if got := AnalyzePayload(ESCPOS, maxHeader); got.RasterBytesDeclared != 0xFFFE0001 || !got.RasterDataIncomplete {
		t.Fatalf("maximum header byte count = %d, incomplete=%t", got.RasterBytesDeclared, got.RasterDataIncomplete)
	}
}

func TestJobDiagnosticsContextRoundTrip(t *testing.T) {
	ctx := WithJobDiagnostics(context.Background(), JobDiagnostics{
		JobID: "job-123", Source: SourceInternalEditor, Protocol: ESCPOS, PageCount: 2,
	})
	ctx = WithJobPage(ctx, 2, 2)
	got := JobDiagnosticsFromContext(ctx)
	if got.JobID != "job-123" || got.Source != SourceInternalEditor ||
		got.Protocol != ESCPOS || got.Page != 2 || got.PageCount != 2 {
		t.Fatalf("JobDiagnosticsFromContext() = %+v", got)
	}
}
