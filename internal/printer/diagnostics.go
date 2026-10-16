package printer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync/atomic"
	"time"
)

const (
	SourceInternalEditor        = "internal-editor"
	SourceAndroidPrintFramework = "android-print-framework"
)

type diagnosticContextKey struct{}

// JobDiagnostics carries non-content identifiers through the print pipeline.
type JobDiagnostics struct {
	JobID     string
	Source    string
	Protocol  Protocol
	Page      int
	PageCount int
}

var diagnosticJobSequence atomic.Uint64

// NewDiagnosticJobID returns a process-unique identifier without using document data.
func NewDiagnosticJobID(prefix string) string {
	if prefix == "" {
		prefix = "print"
	}
	return fmt.Sprintf("%s-%d-%d", prefix, time.Now().UnixNano(), diagnosticJobSequence.Add(1))
}

// WithJobDiagnostics attaches job metadata to a print context.
func WithJobDiagnostics(ctx context.Context, info JobDiagnostics) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, diagnosticContextKey{}, info)
}

// JobDiagnosticsFromContext returns job metadata attached to ctx, if any.
func JobDiagnosticsFromContext(ctx context.Context) JobDiagnostics {
	if ctx == nil {
		return JobDiagnostics{}
	}
	info, _ := ctx.Value(diagnosticContextKey{}).(JobDiagnostics)
	return info
}

// WithJobPage records a one-based page number for per-page transport logs.
func WithJobPage(ctx context.Context, page, pageCount int) context.Context {
	info := JobDiagnosticsFromContext(ctx)
	info.Page = page
	info.PageCount = pageCount
	return WithJobDiagnostics(ctx, info)
}

// PayloadDiagnostics contains a digest and raster metadata, never payload content.
type PayloadDiagnostics struct {
	PayloadBytes             int
	SHA256                   string
	HasRasterHeader          bool
	RasterWidthBytes         int
	RasterWidthDotsPadded    int
	RasterHeightDots         int
	RasterHeaderHex          string
	RasterBytesDeclared      int64
	BytesAfterRasterHeader   int64
	TrailingBytesAfterRaster int64
	RasterDataIncomplete     bool
}

// AnalyzePayload reports the payload digest and the standard ESC/POS GS v 0 raster header.
func AnalyzePayload(protocol Protocol, payload []byte) PayloadDiagnostics {
	digest := sha256.Sum256(payload)
	result := PayloadDiagnostics{
		PayloadBytes: len(payload),
		SHA256:       hex.EncodeToString(digest[:]),
	}
	if protocol != ESCPOS || len(payload) < 10 ||
		payload[0] != 0x1b || payload[1] != 0x40 ||
		payload[2] != 0x1d || payload[3] != 0x76 || payload[4] != 0x30 {
		return result
	}

	result.HasRasterHeader = true
	result.RasterWidthBytes = int(payload[6]) | int(payload[7])<<8
	result.RasterWidthDotsPadded = result.RasterWidthBytes * 8
	result.RasterHeightDots = int(payload[8]) | int(payload[9])<<8
	result.RasterHeaderHex = fmt.Sprintf("% x", payload[2:10])
	result.RasterBytesDeclared = int64(result.RasterWidthBytes) * int64(result.RasterHeightDots)
	result.BytesAfterRasterHeader = int64(len(payload) - 10)
	result.RasterDataIncomplete = result.BytesAfterRasterHeader < result.RasterBytesDeclared
	if !result.RasterDataIncomplete {
		result.TrailingBytesAfterRaster = result.BytesAfterRasterHeader - result.RasterBytesDeclared
	}
	return result
}
