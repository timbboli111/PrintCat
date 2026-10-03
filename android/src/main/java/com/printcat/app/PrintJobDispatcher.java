package com.printcat.app;

import android.content.Context;
import android.graphics.Bitmap;
import android.graphics.Color;
import android.graphics.pdf.PdfRenderer;
import android.os.ParcelFileDescriptor;
import android.print.PrintAttributes;
import android.print.PrintAttributes.MediaSize;
import android.print.PrintAttributes.Resolution;
import android.util.Log;

import java.io.ByteArrayOutputStream;
import java.io.File;
import java.io.FileInputStream;
import java.io.FileOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;
import java.util.ArrayList;
import java.util.List;

/**
 * Turns a PDF {@link ParcelFileDescriptor} into a list of per-page PNG
 * byte arrays, using {@link PdfRenderer} (API 21+).
 *
 * <p>PdfRenderer requires a <em>seekable</em> file descriptor. Print
 * Framework's {@code PrintDocument.getData()} does not guarantee this; the
 * spooler may hand us a pipe-backed descriptor, in which case PdfRenderer
 * fails with "file descriptor not seekable". This dispatcher detects that
 * condition and stages the PDF into a temp file inside the app cache
 * directory before opening the renderer.</p>
 *
 * <p>Data flow:</p>
 * <pre>
 *   input pfd (possibly non-seekable)
 *        │
 *        ├─ statSize &gt; 0 → use directly (fast path)
 *        │
 *        └─ statSize ≤ 0 → stage to temp file, open a new read-only pfd
 *             │
 *             ▼
 *        SeekablePdf.rendererPfd  ← the descriptor actually handed to PdfRenderer
 * </pre>
 *
 * <p>The dispatcher is memory-only for page output (PNGs stay in memory),
 * but file-backed for the raw PDF when staging is required. It does not
 * close the input ParcelFileDescriptor; the caller owns that.</p>
 */
public final class PrintJobDispatcher {

    private static final String TAG = "PrintCatDispatch";

    /** Hard cap on the number of PDF pages we will accept in one job. */
    public static final int MAX_PAGES = 20;

    /** Hard cap on the number of pixels for a single page (~20 MP). */
    public static final int MAX_PIXELS_PER_PAGE = 20_000_000;

    /** Hard cap on the PDF size we will stage to disk when needed (100 MB). */
    private static final long MAX_PDF_BYTES = 100L * 1024L * 1024L;

    /** Upper bound for a plausible DPI value from PrintAttributes. */
    private static final int MAX_DPI = 2400;

    private PrintJobDispatcher() {
    }

    public static final class Result {
        public final byte[][] pagesPng;
        public final int[] widthsUm;
        public final int[] heightsUm;
        public final int dpi;

        Result(byte[][] pagesPng, int[] widthsUm, int[] heightsUm, int dpi) {
            this.pagesPng = pagesPng;
            this.widthsUm = widthsUm;
            this.heightsUm = heightsUm;
            this.dpi = dpi;
        }
    }

    public static final class DispatchException extends Exception {
        public DispatchException(String message) {
            super(message);
        }

        public DispatchException(String message, Throwable cause) {
            super(message, cause);
        }
    }

    /**
     * Render every page of the PDF behind {@code pfd} into a PNG, using the
     * media size and DPI described by {@code attrs}. A {@code fallbackDpi} is
     * used only when {@code attrs} does not carry a valid resolution.
     *
     * @param context used to locate the cache directory for staging
     *                non-seekable PDFs.
     */
    public static Result dispatch(
            Context context,
            ParcelFileDescriptor pfd,
            PrintAttributes attrs,
            int fallbackDpi) throws DispatchException {

        if (context == null) {
            throw new DispatchException("Context is null");
        }
        if (pfd == null) {
            throw new DispatchException("ParcelFileDescriptor is null");
        }
        if (attrs == null) {
            throw new DispatchException("PrintAttributes is null");
        }

        int dpi = resolveDpi(attrs, fallbackDpi);

        MediaSize media = attrs.getMediaSize();
        if (media == null) {
            throw new DispatchException("PrintAttributes has no MediaSize");
        }
        int widthMils = media.getWidthMils();
        int heightMils = media.getHeightMils();
        if (widthMils <= 0 || heightMils <= 0) {
            throw new DispatchException(
                    "Invalid MediaSize: " + widthMils + "x" + heightMils + " mils");
        }

        long widthUm = milsToMicrometers(widthMils);
        long heightUm = milsToMicrometers(heightMils);
        if (widthUm <= 0 || heightUm <= 0) {
            throw new DispatchException(
                    "Computed media size in micrometers is non-positive: "
                            + widthUm + "x" + heightUm);
        }

        int targetWidthPx = micrometersToPixels(widthUm, dpi);
        int targetHeightPx = micrometersToPixels(heightUm, dpi);
        if (targetWidthPx <= 0 || targetHeightPx <= 0) {
            throw new DispatchException(
                    "Computed bitmap size is zero: "
                            + targetWidthPx + "x" + targetHeightPx);
        }

        long totalPixels = (long) targetWidthPx * (long) targetHeightPx;
        if (totalPixels > MAX_PIXELS_PER_PAGE) {
            throw new DispatchException(
                    "Target bitmap too large: " + targetWidthPx + "x" + targetHeightPx
                            + " = " + totalPixels + " pixels (max "
                            + MAX_PIXELS_PER_PAGE + ")");
        }

        SeekablePdf seekable = ensureSeekablePdf(context, pfd);

        List<byte[]> pageList = new ArrayList<>();
        PdfRenderer renderer = null;
        try {
            try {
                renderer = new PdfRenderer(seekable.rendererPfd);
            } catch (IllegalArgumentException iae) {
                // Some devices report a non-negative getStatSize() for a
                // descriptor that PdfRenderer nevertheless rejects as
                // non-seekable. Retry once by forcing a staged copy.
                String msg = iae.getMessage();
                boolean rejectedAsNonSeekable =
                        msg != null && msg.contains("seekable");
                if (rejectedAsNonSeekable && seekable.copyPfd == null) {
                    Log.w(TAG, "PdfRenderer rejected the original descriptor as "
                            + "non-seekable despite getStatSize > 0; retrying "
                            + "with a staged temp file");
                    seekable.close();
                    seekable = stagePdfToTemp(context, pfd);
                    renderer = new PdfRenderer(seekable.rendererPfd);
                } else {
                    throw iae;
                }
            }

            int pageCount = renderer.getPageCount();
            if (pageCount <= 0) {
                throw new DispatchException("PDF has no pages");
            }
            if (pageCount > MAX_PAGES) {
                throw new DispatchException(
                        "PDF has too many pages: " + pageCount
                                + " (max " + MAX_PAGES + ")");
            }

            for (int i = 0; i < pageCount; i++) {
                PdfRenderer.Page page = null;
                Bitmap bitmap = null;
                try {
                    page = renderer.openPage(i);
                    bitmap = Bitmap.createBitmap(
                            targetWidthPx, targetHeightPx, Bitmap.Config.ARGB_8888);
                    bitmap.eraseColor(Color.WHITE);
                    page.render(bitmap, null, null, PdfRenderer.Page.RENDER_MODE_FOR_PRINT);
                    pageList.add(bitmapToPng(bitmap));
                } catch (OutOfMemoryError oom) {
                    throw new DispatchException(
                            "Out of memory rendering page " + i, oom);
                } catch (Throwable t) {
                    throw new DispatchException(
                            "Failed to render page " + i + ": " + t.getMessage(), t);
                } finally {
                    if (page != null) {
                        try {
                            page.close();
                        } catch (Throwable ignored) {
                            // best effort
                        }
                    }
                    if (bitmap != null) {
                        bitmap.recycle();
                    }
                }
            }
        } catch (DispatchException de) {
            throw de;
        } catch (Throwable t) {
            throw new DispatchException(
                    "Failed to open PDF: " + t.getMessage(), t);
        } finally {
            if (renderer != null) {
                try {
                    renderer.close();
                } catch (Throwable ignored) {
                    // best effort
                }
            }
            seekable.close();
        }

        int n = pageList.size();
        byte[][] pagesPng = new byte[n][];
        int[] widthsUmArr = new int[n];
        int[] heightsUmArr = new int[n];
        for (int i = 0; i < n; i++) {
            pagesPng[i] = pageList.get(i);
            widthsUmArr[i] = (int) widthUm;
            heightsUmArr[i] = (int) heightUm;
        }

        Log.d(TAG, "dispatch success: pages=" + n
                + " dpi=" + dpi
                + " pageUm=" + widthUm + "x" + heightUm);
        return new Result(pagesPng, widthsUmArr, heightsUmArr, dpi);
    }

    // ---------------------------------------------------------------------
    // Seekable PDF handling
    // ---------------------------------------------------------------------

    /**
     * Holds the descriptor that {@link PdfRenderer} must actually open,
     * plus an optional staged copy and temp file that need cleanup.
     *
     * <p>The original input descriptor is <strong>not</strong> owned by this
     * class; the caller closes it. Only the staged copy (if any) is closed
     * and the temp file (if any) is deleted by {@link #close()}.</p>
     */
    private static final class SeekablePdf {
        /**
         * The descriptor to pass to {@code new PdfRenderer(...)}. Either the
         * caller-owned original (fast path) or a staged read-only copy
         * (staging path).
         */
        final ParcelFileDescriptor rendererPfd;

        /** Non-null only when the PDF was staged to a temp file. */
        final ParcelFileDescriptor copyPfd;

        /** Non-null only when the PDF was staged to a temp file. */
        final File tempFile;

        SeekablePdf(ParcelFileDescriptor rendererPfd,
                    ParcelFileDescriptor copyPfd,
                    File tempFile) {
            this.rendererPfd = rendererPfd;
            this.copyPfd = copyPfd;
            this.tempFile = tempFile;
        }

        void close() {
            if (copyPfd != null) {
                try {
                    copyPfd.close();
                } catch (Throwable ignored) {
                    // best effort
                }
            }
            if (tempFile != null) {
                //noinspection ResultOfMethodCallIgnored
                tempFile.delete();
            }
        }
    }

    /**
     * Returns a {@link SeekablePdf} whose {@code rendererPfd} is intended to
     * be seekable for {@link PdfRenderer}. If the input reports a positive
     * size via {@link ParcelFileDescriptor#getStatSize()} it is used
     * directly; otherwise the PDF is streamed into a temp file in the app
     * cache directory and the temp file's descriptor is returned.
     */
    private static SeekablePdf ensureSeekablePdf(Context context, ParcelFileDescriptor pfd)
            throws DispatchException {

        long statSize;
        try {
            statSize = pfd.getStatSize();
        } catch (Throwable t) {
            statSize = -1;
        }

        if (statSize > 0) {
            // Looks like a real file on disk: use directly.
            return new SeekablePdf(pfd, null, null);
        }

        Log.d(TAG, "ensureSeekablePdf: input not seekable (statSize="
                + statSize + "), staging to temp file");
        return stagePdfToTemp(context, pfd);
    }

    /**
     * Unconditionally streams the content of {@code source} into a temp file
     * inside {@code context.getCacheDir()} and returns a {@link SeekablePdf}
     * whose {@code rendererPfd} is a read-only descriptor on that file.
     */
    private static SeekablePdf stagePdfToTemp(Context context, ParcelFileDescriptor source)
            throws DispatchException {

        File tempFile;
        try {
            File cacheDir = context.getCacheDir();
            if (cacheDir == null) {
                throw new DispatchException(
                        "Cache directory is null; cannot stage non-seekable PDF");
            }
            tempFile = File.createTempFile("printcat-job-", ".pdf", cacheDir);
        } catch (IOException e) {
            throw new DispatchException(
                    "Failed to create temporary PDF file: " + e.getMessage(), e);
        }

        ParcelFileDescriptor copyPfd;
        try {
            copyPfd = copyPfdToFile(source, tempFile);
        } catch (DispatchException de) {
            //noinspection ResultOfMethodCallIgnored
            tempFile.delete();
            throw de;
        } catch (IOException e) {
            //noinspection ResultOfMethodCallIgnored
            tempFile.delete();
            throw new DispatchException(
                    "Failed to copy PDF to temporary file: " + e.getMessage(), e);
        } catch (Throwable t) {
            //noinspection ResultOfMethodCallIgnored
            tempFile.delete();
            throw new DispatchException(
                    "Unexpected error while staging PDF: " + t.getMessage(), t);
        }

        return new SeekablePdf(copyPfd, copyPfd, tempFile);
    }

    /**
     * Streams the entire content of {@code source} into {@code dest} and
     * returns a read-only ParcelFileDescriptor on {@code dest}. Enforces
     * {@link #MAX_PDF_BYTES}. Does not close the source.
     */
    private static ParcelFileDescriptor copyPfdToFile(
            ParcelFileDescriptor source, File dest)
            throws IOException, DispatchException {

        InputStream in = null;
        OutputStream out = null;
        try {
            in = new FileInputStream(source.getFileDescriptor());
            out = new FileOutputStream(dest);

            byte[] buffer = new byte[64 * 1024];
            long total = 0L;
            int read;
            while ((read = in.read(buffer)) != -1) {
                total += read;
                if (total > MAX_PDF_BYTES) {
                    throw new DispatchException(
                            "PDF exceeds maximum supported size of "
                                    + MAX_PDF_BYTES + " bytes while staging");
                }
                out.write(buffer, 0, read);
            }
            out.flush();
        } finally {
            if (in != null) {
                try {
                    in.close();
                } catch (Throwable ignored) {
                    // best effort
                }
            }
            if (out != null) {
                try {
                    out.close();
                } catch (Throwable ignored) {
                    // best effort
                }
            }
        }

        try {
            return ParcelFileDescriptor.open(dest, ParcelFileDescriptor.MODE_READ_ONLY);
        } catch (IOException e) {
            throw new DispatchException(
                    "Failed to open staged PDF: " + e.getMessage(), e);
        }
    }

    // ---------------------------------------------------------------------
    // Attribute handling
    // ---------------------------------------------------------------------

    private static int resolveDpi(PrintAttributes attrs, int fallbackDpi)
            throws DispatchException {
        Resolution res = attrs.getResolution();
        if (res != null) {
            int h = res.getHorizontalDpi();
            if (h > 0 && h <= MAX_DPI) {
                return h;
            }
            Log.w(TAG, "PrintAttributes resolution out of range: " + h);
        }
        if (fallbackDpi > 0 && fallbackDpi <= MAX_DPI) {
            Log.w(TAG, "Falling back to DPI " + fallbackDpi
                    + " because PrintAttributes provided no valid resolution");
            return fallbackDpi;
        }
        throw new DispatchException(
                "No valid DPI available (attributes missing, no usable fallback)");
    }

    private static long milsToMicrometers(int mils) {
        // 1 mil = 25.4 micrometers = 254 / 10 micrometers.
        return (long) mils * 254L / 10L;
    }

    private static int micrometersToPixels(long um, int dpi) {
        // pixels = um / 25_400 * dpi, rounded to nearest.
        long numerator = um * (long) dpi;
        long denominator = 25_400L;
        if (numerator <= 0 || denominator <= 0) {
            return 0;
        }
        return (int) ((numerator + denominator / 2) / denominator);
    }

    private static byte[] bitmapToPng(Bitmap bitmap) throws IOException {
        ByteArrayOutputStream out = new ByteArrayOutputStream();
        try {
            boolean ok = bitmap.compress(Bitmap.CompressFormat.PNG, 100, out);
            if (!ok) {
                throw new IOException("Bitmap.compress returned false");
            }
        } finally {
            out.close();
        }
        return out.toByteArray();
    }
}