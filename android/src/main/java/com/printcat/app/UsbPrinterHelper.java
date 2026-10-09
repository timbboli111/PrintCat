package com.printcat.app;

import android.app.PendingIntent;
import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;
import android.content.IntentFilter;
import android.hardware.usb.UsbConstants;
import android.hardware.usb.UsbDevice;
import android.hardware.usb.UsbDeviceConnection;
import android.hardware.usb.UsbEndpoint;
import android.hardware.usb.UsbInterface;
import android.hardware.usb.UsbManager;
import android.os.Build;
import android.util.Log;

import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;

/**
 * USB thermal printer helper backed by the Android USB Host API.
 *
 * <p>Devices are selected at discovery time when they expose at least one
 * interface with a bulk OUT endpoint. Printer-class interfaces are
 * preferred; vendor-specific interfaces with a bulk OUT endpoint are also
 * accepted, because many thermal printers do not set the printer-class
 * descriptor bit.</p>
 *
 * <p>This helper deliberately does NOT handle USB-serial/CDC devices: those
 * are a different transport (Serial) and use different endpoint types.</p>
 */
public final class UsbPrinterHelper {

    private static final String TAG = "PrintCatUsb";
    public static final String ACTION_USB_PERMISSION =
            "com.printcat.app.USB_PERMISSION";

    private UsbPrinterHelper() {
    }

    public static final class DeviceInfo {
        public final int deviceId;
        public final String name;
        public final int vendorId;
        public final int productId;
        public final int interfaceCount;

        public DeviceInfo(int deviceId, String name, int vendorId, int productId, int interfaceCount) {
            this.deviceId = deviceId;
            this.name = name;
            this.vendorId = vendorId;
            this.productId = productId;
            this.interfaceCount = interfaceCount;
        }
    }

    private static UsbManager getUsbManager(Context context) {
        if (context == null) {
            return null;
        }
        return (UsbManager) context.getSystemService(Context.USB_SERVICE);
    }

    /**
     * Returns printable USB devices. A device is considered printable if it
     * has at least one interface with a bulk OUT endpoint. Interfaces of
     * printer class are preferred and, when present, may be the only ones
     * considered, but the filter ultimately falls back to any bulk OUT
     * interface so that vendor-specific printers are detected.
     */
    public static DeviceInfo[] listPrinters(Context context) {
        UsbManager manager = getUsbManager(context);
        if (manager == null) {
            return new DeviceInfo[0];
        }
        HashMap<String, UsbDevice> deviceList = manager.getDeviceList();
        if (deviceList == null || deviceList.isEmpty()) {
            return new DeviceInfo[0];
        }
        List<DeviceInfo> out = new ArrayList<>();
        for (UsbDevice dev : deviceList.values()) {
            if (!isLikelyPrinter(dev)) {
                continue;
            }
            String name = dev.getProductName() != null
                    ? dev.getProductName()
                    : dev.getDeviceName();
            out.add(new DeviceInfo(
                    dev.getDeviceId(),
                    name,
                    dev.getVendorId(),
                    dev.getProductId(),
                    dev.getInterfaceCount()));
        }
        return out.toArray(new DeviceInfo[0]);
    }

    /**
     * Returns true when the device has at least one interface with a bulk
     * OUT endpoint. Printer-class interfaces are checked first, then any
     * interface. USB-serial CDC devices typically expose bulk IN + bulk OUT
     * on an interface whose class is CDC_DATA (0x0A); those are NOT
     * considered printable here and should be handled by the Serial
     * transport instead.
     */
    private static boolean isLikelyPrinter(UsbDevice dev) {
        // Explicit printer-class interface.
        for (int i = 0; i < dev.getInterfaceCount(); i++) {
            UsbInterface itf = dev.getInterface(i);
            if (itf.getInterfaceClass() == UsbConstants.USB_CLASS_PRINTER
                    && hasBulkOut(itf)) {
                return true;
            }
        }
        // Vendor-specific or unknown interface with a bulk OUT endpoint.
        for (int i = 0; i < dev.getInterfaceCount(); i++) {
            UsbInterface itf = dev.getInterface(i);
            if (itf.getInterfaceClass() == UsbConstants.USB_CLASS_CDC_DATA) {
                // CDC data interface — that is USB-serial territory.
                continue;
            }
            if (hasBulkOut(itf)) {
                return true;
            }
        }
        return false;
    }

    private static boolean hasBulkOut(UsbInterface itf) {
        for (int e = 0; e < itf.getEndpointCount(); e++) {
            UsbEndpoint ep = itf.getEndpoint(e);
            if (ep.getType() == UsbConstants.USB_ENDPOINT_XFER_BULK
                    && ep.getDirection() == UsbConstants.USB_DIR_OUT) {
                return true;
            }
        }
        return false;
    }

    private static UsbDevice findDeviceById(Context context, int deviceId) {
        UsbManager manager = getUsbManager(context);
        if (manager == null) {
            return null;
        }
        HashMap<String, UsbDevice> list = manager.getDeviceList();
        if (list == null) {
            return null;
        }
        for (UsbDevice dev : list.values()) {
            if (dev.getDeviceId() == deviceId) {
                return dev;
            }
        }
        return null;
    }

    private static boolean hasPermission(Context context, UsbDevice dev) {
        UsbManager manager = getUsbManager(context);
        return manager != null && manager.hasPermission(dev);
    }

    private static PendingIntent buildPermissionIntent(Context context) {
        Intent intent = new Intent(ACTION_USB_PERMISSION);
        intent.setPackage(context.getPackageName());
        int flags = 0;
        if (Build.VERSION.SDK_INT >= 31) {
            flags = PendingIntent.FLAG_MUTABLE;
        }
        return PendingIntent.getBroadcast(context, 0, intent, flags);
    }

    /**
     * Requests USB permission for the given device id and blocks until the
     * result arrives or the timeout expires. Returns true only when
     * permission is granted. Returns false on denial, timeout, or any
     * platform error.
     */
    public static boolean requestPermissionSync(Context context, int deviceId, long timeoutMs) {
        UsbDevice dev = findDeviceById(context, deviceId);
        if (dev == null) {
            Log.w(TAG, "requestPermissionSync: device " + deviceId + " not present");
            return false;
        }
        if (hasPermission(context, dev)) {
            return true;
        }

        final CountDownLatch latch = new CountDownLatch(1);
        final boolean[] granted = new boolean[]{false};

        BroadcastReceiver receiver = new BroadcastReceiver() {
            @Override
            public void onReceive(Context ctx, Intent intent) {
                if (!ACTION_USB_PERMISSION.equals(intent.getAction())) {
                    return;
                }
                UsbDevice received = intent.getParcelableExtra(UsbManager.EXTRA_DEVICE);
                boolean ok = intent.getBooleanExtra(UsbManager.EXTRA_PERMISSION_GRANTED, false);
                if (received != null && received.getDeviceId() == deviceId && ok) {
                    granted[0] = true;
                }
                latch.countDown();
            }
        };

        IntentFilter filter = new IntentFilter(ACTION_USB_PERMISSION);
        if (Build.VERSION.SDK_INT >= 33) {
            context.registerReceiver(receiver, filter, Context.RECEIVER_NOT_EXPORTED);
        } else {
            context.registerReceiver(receiver, filter);
        }

        try {
            UsbManager manager = getUsbManager(context);
            if (manager == null) {
                return false;
            }
            manager.requestPermission(dev, buildPermissionIntent(context));
            boolean completed = latch.await(timeoutMs, TimeUnit.MILLISECONDS);
            if (!completed) {
                Log.w(TAG, "requestPermissionSync: timeout for device " + deviceId);
            }
            return granted[0];
        } catch (InterruptedException ie) {
            Thread.currentThread().interrupt();
            return false;
        } catch (Throwable t) {
            Log.e(TAG, "requestPermissionSync failed", t);
            return false;
        } finally {
            try {
                context.unregisterReceiver(receiver);
            } catch (Throwable ignored) {
            }
        }
    }

    /**
     * Sends payload to the given USB device. Requests permission if not
     * already granted. Claims the first interface with a bulk OUT endpoint,
     * writes the payload, then releases the interface.
     *
     * Return codes:
     *   0  success
     *  -1  context/manager error
     *  -2  device not found
     *  -3  permission denied or timed out
     *  -4  openDevice failed
     *  -5  no interface with bulk OUT endpoint
     *  -6  claimInterface failed
     *  -7  bulkTransfer failed or returned wrong byte count
     */
    public static int sendBytes(Context context, int deviceId, byte[] payload, long timeoutMs) {
        if (payload == null || payload.length == 0) {
            return 0;
        }
        if (timeoutMs <= 0) {
            timeoutMs = 5000;
        }

        UsbManager manager = getUsbManager(context);
        if (manager == null) {
            Log.e(TAG, "sendBytes: UsbManager unavailable");
            return -1;
        }
        UsbDevice dev = findDeviceById(context, deviceId);
        if (dev == null) {
            Log.e(TAG, "sendBytes: device " + deviceId + " not present");
            return -2;
        }
        if (!hasPermission(context, dev)) {
            if (!requestPermissionSync(context, deviceId, timeoutMs)) {
                Log.e(TAG, "sendBytes: permission denied for device " + deviceId);
                return -3;
            }
        }

        UsbInterface targetInterface = null;
        UsbEndpoint targetOut = null;

        // Prefer printer-class interface with bulk OUT.
        for (int i = 0; i < dev.getInterfaceCount(); i++) {
            UsbInterface itf = dev.getInterface(i);
            if (itf.getInterfaceClass() != UsbConstants.USB_CLASS_PRINTER) {
                continue;
            }
            UsbEndpoint ep = findBulkOut(itf);
            if (ep != null) {
                targetInterface = itf;
                targetOut = ep;
                break;
            }
        }
        // Fallback: any interface with bulk OUT, excluding CDC_DATA.
        if (targetInterface == null) {
            for (int i = 0; i < dev.getInterfaceCount(); i++) {
                UsbInterface itf = dev.getInterface(i);
                if (itf.getInterfaceClass() == UsbConstants.USB_CLASS_CDC_DATA) {
                    continue;
                }
                UsbEndpoint ep = findBulkOut(itf);
                if (ep != null) {
                    targetInterface = itf;
                    targetOut = ep;
                    break;
                }
            }
        }
        if (targetInterface == null || targetOut == null) {
            Log.e(TAG, "sendBytes: no interface with bulk OUT endpoint");
            return -5;
        }

        UsbDeviceConnection connection = manager.openDevice(dev);
        if (connection == null) {
            Log.e(TAG, "sendBytes: openDevice failed for " + deviceId);
            return -4;
        }

        try {
            if (!connection.claimInterface(targetInterface, true)) {
                Log.e(TAG, "sendBytes: claimInterface failed");
                return -6;
            }
            try {
                int written = connection.bulkTransfer(targetOut, payload, payload.length, (int) timeoutMs);
                if (written < 0) {
                    Log.e(TAG, "sendBytes: bulkTransfer returned " + written);
                    return -7;
                }
                if (written != payload.length) {
                    Log.e(TAG, "sendBytes: short write " + written + "/" + payload.length);
                    return -7;
                }
                return 0;
            } finally {
                try {
                    connection.releaseInterface(targetInterface);
                } catch (Throwable ignored) {
                }
            }
        } finally {
            try {
                connection.close();
            } catch (Throwable ignored) {
            }
        }
    }

    private static UsbEndpoint findBulkOut(UsbInterface itf) {
        for (int i = 0; i < itf.getEndpointCount(); i++) {
            UsbEndpoint ep = itf.getEndpoint(i);
            if (ep.getType() == UsbConstants.USB_ENDPOINT_XFER_BULK
                    && ep.getDirection() == UsbConstants.USB_DIR_OUT) {
                return ep;
            }
        }
        return null;
    }
}