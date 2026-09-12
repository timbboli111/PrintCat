package com.printcat.app;

import android.print.PrinterId;
import android.printservice.PrintJob;
import android.printservice.PrintService;
import android.printservice.PrinterDiscoverySession;

import java.util.List;

/**
 * Android Print Framework entry point for PrintCat.
 *
 * <p>M5-A intentionally only registers a valid service. Printer discovery and
 * print-job routing will be added in later milestones, after the shared
 * PDF/document and thermal-raster pipeline exists.</p>
 */
public final class PrintCatPrintService extends PrintService {
    @Override
    protected PrinterDiscoverySession onCreatePrinterDiscoverySession() {
        return new EmptyPrinterDiscoverySession();
    }

    @Override
    protected void onPrintJobQueued(PrintJob printJob) {
        // Do not send anything to the existing Bluetooth engine until M5-B
        // provides a complete framework-job-to-raster pipeline.
        printJob.fail("PrintCat print-job processing is not available yet.");
    }

    @Override
    protected void onRequestCancelPrintJob(PrintJob printJob) {
        printJob.cancel();
    }

    private static final class EmptyPrinterDiscoverySession extends PrinterDiscoverySession {
        @Override
        public void onStartPrinterDiscovery(List<PrinterId> priorityList) {
            // M5-A: registration only; no printer discovery is exposed yet.
        }

        @Override
        public void onStopPrinterDiscovery() {
            // Nothing to stop until printer discovery is implemented.
        }

        @Override
        public void onValidatePrinters(List<PrinterId> printerIds) {
            // There are no framework printers to validate during M5-A.
        }

        @Override
        public void onStartPrinterStateTracking(PrinterId printerId) {
            // State tracking is deferred with framework printer discovery.
        }

        @Override
        public void onStopPrinterStateTracking(PrinterId printerId) {
            // State tracking is deferred with framework printer discovery.
        }

        @Override
        public void onRequestCustomPrinterIcon(PrinterId printerId) {
            // No framework printers are advertised during M5-A.
        }

        @Override
        public void onDestroy() {
            // No resources are acquired by the registration-only session.
        }
    }
}
