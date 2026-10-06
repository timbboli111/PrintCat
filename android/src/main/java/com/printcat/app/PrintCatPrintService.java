package com.printcat.app;

import android.content.Context;
import android.os.Handler;
import android.os.Looper;
import android.os.ParcelFileDescriptor;
import android.print.PrintAttributes;
import android.print.PrintAttributes.MediaSize;
import android.print.PrinterCapabilitiesInfo;
import android.print.PrinterId;
import android.print.PrinterInfo;
import android.printservice.PrintJob;
import android.printservice.PrintService;
import android.printservice.PrinterDiscoverySession;
import android.util.Log;

import java.io.File;
import java.util.Collections;
import java.util.List;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

/**
 * Android Print Framework entry point for PrintCat.
 *
 * <p>Step 0 proved the JNI round-trip. Step 1B wired the PDF pipeline. Step
 * 1C ensures the process-wide bridge state exists and the persisted active
 * printer is loaded so that jobs arriving through the PrintService reach
 * the existing PrintCat engine even when the Fyne UI has not been opened in
 * this process. This revision also advertises the active printer's
 * user-chosen paper dimensions to the Android Print Framework, replacing
 * the previous hard-coded 80x200 mm default.</p>
 *
 * <p><strong>Threading boundary.</strong> Every {@code
 * android.printservice.PrintJob} method (getId, getInfo, getDocument,
 * complete, fail, cancel) must be called from the main thread; the
 * framework enforces this with
 * {@code PrintService.throwIfNotCalledOnMainThread()}. Heavy work (PDF
 * rasterisation, PNG compression, JNI submission) runs on a single-thread
 * worker executor. The worker never sees a {@code PrintJob}: it receives an
 * immutable snapshot (jobId, ParcelFileDescriptor, PrintAttributes,
 * localId), a Context for staging, and a {@link PrintCompletion} handle
 * whose methods marshal complete()/fail() back onto the main thread.</p>
 *
 * <p>{@code PROBE_MODE} is preserved for regression. When true, only the
 * Step 0 probe path runs; when false, the real submit path runs.</p>
 */
public final class PrintCatPrintService extends PrintService {

    private static final String TAG = "PrintCatService";
    private static final String PROBE_TAG = "PrintCatProbe";

    /**
     * When true, only Step 0 probe behaviour runs and every job is failed
     * after the JNI round-trip. Set to false for Step 1B and later.
     */
    private static final boolean PROBE_MODE = false;

    /** Fallback DPI used only when PrintAttributes does not carry one. */
    private static final int FALLBACK_DPI = 203;

    /** Fallback paper size (mm) when the bridge has no active paper size. */
    private static final int FALLBACK_PAPER_WIDTH_MM = 80;
    private static final int FALLBACK_PAPER_HEIGHT_MM = 200;

    /**
     * Canonical config path relative to the Android app files directory.
     * Fyne on Android maps {@code application.Storage().RootURI()} to
     * {@code <app-files>/fyne/}, so the persisted config lives at
     * {@code <app-files>/fyne/config.json}. Using the same relative path
     * here guarantees the PrintService bootstrap reads the exact file the
     * Fyne UI writes.
     */
    private static final String CONFIG_RELATIVE_PATH = "fyne/config.json";

    // Step 0 probe entry point (kept for regression).
    private static native int nativeProbe(int marker);

    // Step 1C bootstrap: ensures the process-wide bridge state exists and
    // the persisted active printer is loaded. Returns:
    //   0 = ready
    //  -1 = bootstrap failure
    //  -2 = no active printer configured
    private static native int nativeEnsureBridgeInitialized(String configPath);

    // Step 1C paper propagation: returns the active printer's user-chosen
    // paper width or height in mils (1 mm = 5000/127 mils), or -1 if not
    // configured.
    private static native int nativeGetActivePaperWidthMils();
    private static native int nativeGetActivePaperHeightMils();

    // Step 1B submit entry point. See cmd/printcat/bridge_android.go for the
    // full return-code table.
    private static native int nativeSubmitJob(
            byte[][] pagesPng,
            int[] widthsUm,
            int[] heightsUm,
            int dpi,
            String printerIDLocal);

    // Single-thread executor used to move PDF rendering and JNI submission
    // off the PrintService main looper, avoiding ANR on large jobs.
    private final ExecutorService worker = Executors.newSingleThreadExecutor();

    // Handler bound to the main looper. Used by PrintCompletion to post
    // complete()/fail() onto the main thread.
    private final Handler mainHandler = new Handler(Looper.getMainLooper());

    static {
        try {
            System.loadLibrary("main");
            Log.d(TAG, "System.loadLibrary(\"main\") OK");
        } catch (UnsatisfiedLinkError e) {
            Log.e(TAG, "System.loadLibrary(\"main\") FAILED: " + e.getMessage());
        }
    }

    @Override
    public void onCreate() {
        super.onCreate();
        Log.d(TAG, "PrintCatPrintService.onCreate");
        ensureBridgeInitialized();
    }

    /**
     * Best-effort initialization of the Go bridge. Failures are logged, not
     * thrown, so that the PrintService remains usable even in edge cases.
     * The Fyne UI path additionally calls bridge.Bootstrap when it starts;
     * Bootstrap is idempotent.
     */
    private void ensureBridgeInitialized() {
        String configPath = "";
        try {
            File filesDir = getFilesDir();
            if (filesDir != null) {
                configPath = new File(filesDir, CONFIG_RELATIVE_PATH).getAbsolutePath();
            }
        } catch (Throwable t) {
            Log.e(TAG, "compute config path failed", t);
        }
        try {
            int rc = nativeEnsureBridgeInitialized(configPath);
            Log.d(TAG, "nativeEnsureBridgeInitialized(" + configPath + ") => " + rc);
        } catch (UnsatisfiedLinkError e) {
            Log.e(TAG, "nativeEnsureBridgeInitialized: library not loaded: "
                    + e.getMessage());
        } catch (Throwable t) {
            Log.e(TAG, "nativeEnsureBridgeInitialized failed", t);
        }
    }

    @Override
    protected PrinterDiscoverySession onCreatePrinterDiscoverySession() {
        return new ProbePrinterDiscoverySession();
    }

    @Override
    protected void onPrintJobQueued(final PrintJob printJob) {
        if (printJob == null) {
            Log.e(TAG, "onPrintJobQueued received null job");
            return;
        }

        if (PROBE_MODE) {
            Log.d(TAG, "onPrintJobQueued START id=" + printJob.getId());
            runProbePath(printJob);
            return;
        }

        // ---- MAIN THREAD: extract every value the worker needs. ----
        //
        // All PrintJob API calls happen here, on the main thread. After this
        // block, the PrintJob reference is stored only inside the
        // PrintCompletion handle; the worker never receives it.

        final String jobId;
        try {
            jobId = String.valueOf(printJob.getId());
        } catch (Throwable t) {
            Log.e(TAG, "printJob.getId() failed on main thread", t);
            safeFailJob(printJob, "getId failed: " + t.getMessage());
            return;
        }
        Log.d(TAG, "onPrintJobQueued START id=" + jobId);

        final ParcelFileDescriptor pfd;
        try {
            pfd = printJob.getDocument().getData();
        } catch (Throwable t) {
            Log.e(TAG, "getDocument().getData() failed", t);
            safeFailJob(printJob, "cannot access print document: " + t.getMessage());
            return;
        }
        if (pfd == null) {
            safeFailJob(printJob, "PrintDocument.getData() returned null");
            return;
        }

        final PrintAttributes attrs;
        try {
            if (printJob.getInfo() == null) {
                closeQuietly(pfd);
                safeFailJob(printJob, "PrintJobInfo is null");
                return;
            }
            attrs = printJob.getInfo().getAttributes();
        } catch (Throwable t) {
            Log.e(TAG, "getInfo().getAttributes() failed", t);
            closeQuietly(pfd);
            safeFailJob(printJob, "cannot access attributes: " + t.getMessage());
            return;
        }
        if (attrs == null) {
            closeQuietly(pfd);
            safeFailJob(printJob, "PrintAttributes is null");
            return;
        }

        String localIdTmp = "";
        try {
            if (printJob.getInfo() != null
                    && printJob.getInfo().getPrinterId() != null) {
                localIdTmp = printJob.getInfo().getPrinterId().getLocalId();
            }
        } catch (Throwable t) {
            Log.w(TAG, "read printerId local id failed: " + t.getMessage());
        }
        final String localId = localIdTmp;

        final PrintCompletion completion =
                new PrintCompletion(printJob, mainHandler, jobId);

        final Context context = PrintCatPrintService.this;

        // Best-effort re-bootstrap right before dispatching. If the UI or
        // onCreate already initialized the bridge, this is a fast no-op.
        ensureBridgeInitialized();

        Log.d(TAG, "onPrintJobQueued: dispatching to worker, jobId=" + jobId
                + " localId=" + localId);

        worker.execute(new Runnable() {
            @Override
            public void run() {
                runSubmitWorker(context, pfd, attrs, localId, completion);
            }
        });
    }

    @Override
    protected void onRequestCancelPrintJob(PrintJob printJob) {
        if (printJob == null) {
            return;
        }
        try {
            printJob.cancel();
        } catch (Throwable t) {
            Log.e(TAG, "printJob.cancel threw", t);
        }
    }

    @Override
    public void onDestroy() {
        Log.d(TAG, "PrintCatPrintService.onDestroy");
        worker.shutdownNow();
        super.onDestroy();
    }

    // ---------------------------------------------------------------------
    // Step 0 probe path
    // ---------------------------------------------------------------------

    private void runProbePath(PrintJob printJob) {
        final int pid = android.os.Process.myPid();
        final String threadName = Thread.currentThread().getName();
        Log.d(PROBE_TAG, "onPrintJobQueued START pid=" + pid
                + " thread=" + threadName
                + " jobId=" + printJob.getId());

        try {
            Log.d(PROBE_TAG, "info=" + String.valueOf(printJob.getInfo()));
        } catch (Throwable t) {
            Log.e(PROBE_TAG, "read job info failed", t);
        }

        try {
            Object attrs = printJob.getClass().getMethod("getAttributes").invoke(printJob);
            Log.d(PROBE_TAG, "attrs (reflection)=" + String.valueOf(attrs));
        } catch (Throwable t) {
            Log.w(PROBE_TAG, "getAttributes not accessible on this SDK: " + t);
        }

        ParcelFileDescriptor pfd = null;
        try {
            pfd = printJob.getDocument().getData();
            Log.d(PROBE_TAG, "PFD acquired=" + (pfd != null));
            if (pfd != null) {
                Log.d(PROBE_TAG, "PFD fd=" + pfd.getFd()
                        + " statSize=" + pfd.getStatSize());
            }
        } catch (Throwable t) {
            Log.e(PROBE_TAG, "read document/PFD failed", t);
        } finally {
            if (pfd != null) {
                try {
                    pfd.close();
                } catch (Throwable ignored) {
                    // probe only
                }
            }
        }

        final int probeMarker = pid;
        try {
            final int result = nativeProbe(probeMarker);
            Log.d(PROBE_TAG, "nativeProbe(" + probeMarker + ") => " + result
                    + " (expected 42)");
        } catch (UnsatisfiedLinkError e) {
            Log.e(PROBE_TAG, "nativeProbe FAILED (UnsatisfiedLinkError): "
                    + e.getMessage());
        } catch (Throwable t) {
            Log.e(PROBE_TAG, "nativeProbe FAILED", t);
        }

        try {
            printJob.getClass()
                    .getMethod("fail", String.class)
                    .invoke(printJob,
                            "PrintCat Step 0 probe only; printing not implemented yet.");
        } catch (Throwable t) {
            Log.e(PROBE_TAG, "printJob.fail via reflection failed", t);
        }

        Log.d(PROBE_TAG, "onPrintJobQueued END");
    }

    // ---------------------------------------------------------------------
    // Step 1B submit path
    // ---------------------------------------------------------------------

    private void runSubmitWorker(final Context context,
                                 final ParcelFileDescriptor pfd,
                                 final PrintAttributes attrs,
                                 final String localId,
                                 final PrintCompletion completion) {
        int rc;
        String failReason = null;
        try {
            PrintJobDispatcher.Result result;
            try {
                result = PrintJobDispatcher.dispatch(context, pfd, attrs, FALLBACK_DPI);
            } catch (PrintJobDispatcher.DispatchException de) {
                rc = -100;
                failReason = "PDF dispatch failed: " + de.getMessage();
                Log.e(TAG, "runSubmitWorker: " + failReason);
                completion.reportFailure(failReason);
                return;
            }

            Log.d(TAG, "runSubmitWorker: submitting " + result.pagesPng.length
                    + " page(s) dpi=" + result.dpi + " localId=" + localId);
            rc = nativeSubmitJob(
                    result.pagesPng,
                    result.widthsUm,
                    result.heightsUm,
                    result.dpi,
                    localId);
            if (rc != 0) {
                failReason = "nativeSubmitJob returned rc=" + rc;
                Log.e(TAG, "runSubmitWorker: " + failReason);
            } else {
                Log.d(TAG, "runSubmitWorker: nativeSubmitJob OK");
            }
        } catch (Throwable t) {
            Log.e(TAG, "runSubmitWorker unexpected error", t);
            rc = -101;
            failReason = "unexpected: " + t.getMessage();
        } finally {
            closeQuietly(pfd);
        }

        if (rc == 0) {
            completion.reportSuccess();
        } else {
            completion.reportFailure(failReason != null ? failReason : "unknown error");
        }
    }

    // ---------------------------------------------------------------------
    // Helpers
    // ---------------------------------------------------------------------

    private static void safeFailJob(PrintJob printJob, String reason) {
        Log.e(TAG, "failJob: " + reason);
        try {
            printJob.fail(reason);
        } catch (Throwable t) {
            Log.e(TAG, "printJob.fail threw", t);
        }
    }

    private static void closeQuietly(ParcelFileDescriptor pfd) {
        if (pfd == null) {
            return;
        }
        try {
            pfd.close();
        } catch (Throwable ignored) {
            // best effort
        }
    }

    /**
     * Reads the active printer's user-chosen paper dimensions from the Go
     * bridge, falling back to the historical 80x200 mm default when none is
     * configured. Returns mils, matching the units used by
     * {@link MediaSize}.
     */
    private int[] readActivePaperMils() {
        int widthMils = -1;
        int heightMils = -1;
        try {
            widthMils = nativeGetActivePaperWidthMils();
            heightMils = nativeGetActivePaperHeightMils();
        } catch (UnsatisfiedLinkError e) {
            Log.w(TAG, "nativeGetActivePaper*Mils: library not loaded: "
                    + e.getMessage());
        } catch (Throwable t) {
            Log.w(TAG, "nativeGetActivePaper*Mils failed", t);
        }
        if (widthMils <= 0 || heightMils <= 0) {
            widthMils = mmToMils(FALLBACK_PAPER_WIDTH_MM);
            heightMils = mmToMils(FALLBACK_PAPER_HEIGHT_MM);
        }
        return new int[]{widthMils, heightMils};
    }

    private static int mmToMils(int mm) {
        // 1 mm = 1000/25.4 mils = 5000/127 mils.
        return mm * 5000 / 127;
    }

    private static final class PrintCompletion {
        private final PrintJob printJob;
        private final Handler mainHandler;
        private final String jobId;

        PrintCompletion(PrintJob printJob, Handler mainHandler, String jobId) {
            this.printJob = printJob;
            this.mainHandler = mainHandler;
            this.jobId = jobId;
        }

        void reportSuccess() {
            mainHandler.post(new Runnable() {
                @Override
                public void run() {
                    try {
                        printJob.complete();
                        Log.d(TAG, "printJob.complete() OK for job " + jobId);
                    } catch (Throwable t) {
                        Log.e(TAG, "printJob.complete() failed", t);
                    }
                }
            });
        }

        void reportFailure(final String reason) {
            mainHandler.post(new Runnable() {
                @Override
                public void run() {
                    try {
                        printJob.fail(reason);
                        Log.d(TAG, "printJob.fail() posted for job " + jobId
                                + " reason=" + reason);
                    } catch (Throwable t) {
                        Log.e(TAG, "printJob.fail() threw", t);
                    }
                }
            });
        }
    }

    // ---------------------------------------------------------------------
    // Discovery session
    // ---------------------------------------------------------------------

    /**
     * Advertises the single active PrintCat printer to the Android Print
     * Framework. The MediaSize is read from the Go bridge on each discovery
     * cycle, so changes to the active printer's paper dimensions propagate
     * to the framework's print dialog.
     */
    private final class ProbePrinterDiscoverySession extends PrinterDiscoverySession {

        @Override
        public void onStartPrinterDiscovery(List<PrinterId> priorityList) {
            try {
                PrinterId pid = PrintCatPrintService.this.generatePrinterId("printcat-probe");

                int[] paperMils = readActivePaperMils();
                int widthMils = paperMils[0];
                int heightMils = paperMils[1];
                // Convert back to mm for the human-readable label; mils may
                // be an approximation of the user-chosen mm value.
                int widthMm = (int) Math.round(widthMils * 25.4 / 1000.0);
                int heightMm = (int) Math.round(heightMils * 25.4 / 1000.0);

                String sizeId = "receipt-" + widthMm + "x" + heightMm;
                String sizeLabel = widthMm + "mm x " + heightMm + "mm";

                PrinterCapabilitiesInfo.Builder caps =
                        new PrinterCapabilitiesInfo.Builder(pid);
                caps.setMinMargins(PrintAttributes.Margins.NO_MARGINS);
                caps.setColorModes(
                        PrintAttributes.COLOR_MODE_MONOCHROME,
                        PrintAttributes.COLOR_MODE_MONOCHROME);
                caps.setDuplexModes(
                        PrintAttributes.DUPLEX_MODE_NONE,
                        PrintAttributes.DUPLEX_MODE_NONE);
                caps.addResolution(
                        new PrintAttributes.Resolution("203", "203dpi", 203, 203),
                        true);
                caps.addMediaSize(
                        new MediaSize(sizeId, sizeLabel, widthMils, heightMils),
                        true);

                PrinterInfo info = new PrinterInfo.Builder(
                        pid, "PrintCat Probe", PrinterInfo.STATUS_IDLE)
                        .setCapabilities(caps.build())
                        .build();

                addPrinters(Collections.singletonList(info));
                Log.d(TAG, "ProbePrinterDiscoverySession advertised printcat-probe size="
                        + sizeLabel + " (" + widthMils + "x" + heightMils + " mils)");
            } catch (Throwable t) {
                Log.e(TAG, "onStartPrinterDiscovery failed", t);
            }
        }

        @Override
        public void onStopPrinterDiscovery() {
        }

        @Override
        public void onValidatePrinters(List<PrinterId> printerIds) {
        }

        @Override
        public void onStartPrinterStateTracking(PrinterId printerId) {
        }

        @Override
        public void onStopPrinterStateTracking(PrinterId printerId) {
        }

        @Override
        public void onDestroy() {
        }
    }
}