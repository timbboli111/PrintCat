// android/com/printcat/app/BluetoothDiscoveryHelper.java
package com.printcat.app;

import android.bluetooth.BluetoothAdapter;
import android.bluetooth.BluetoothDevice;
import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;
import android.content.IntentFilter;
import android.os.Build;

import java.io.IOException;
import java.util.ArrayList;
import java.util.Collections;
import java.util.Comparator;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;

public class BluetoothDiscoveryHelper {

    public static class DeviceInfo {
        public final String address;
        public final String name;

        public DeviceInfo(String address, String name) {
            this.address = address;
            this.name = name;
        }
    }

    private static final Object lock = new Object();

    private static volatile boolean discoveryRunning = false;
    private static volatile CountDownLatch currentLatch = null;

    private enum State {
        IDLE, STARTING, DISCOVERING, FINISHED
    }

    private static State state = State.IDLE;
    private static boolean cancellationRequested = false;

    public static DeviceInfo[] startDiscovery(Context context, long timeoutMs)
            throws SecurityException, IOException, IllegalStateException, IllegalArgumentException {

        if (context == null) {
            throw new IllegalArgumentException("Context is null");
        }
        if (timeoutMs <= 0) {
            throw new IllegalArgumentException("Timeout must be positive");
        }

        synchronized (lock) {
            if (discoveryRunning) {
                throw new IllegalStateException("Discovery already running");
            }
            discoveryRunning = true;
            cancellationRequested = false;
            state = State.STARTING;
        }

        BluetoothAdapter adapter = null;
        BroadcastReceiver receiver = null;
        CountDownLatch latch = new CountDownLatch(1);
        final Map<String, String> deviceMap = new HashMap<>();

        try {
            adapter = BluetoothAdapter.getDefaultAdapter();
            if (adapter == null) {
                throw new IOException("Bluetooth adapter not available");
            }
            if (!adapter.isEnabled()) {
                throw new IOException("Bluetooth is disabled");
            }

            receiver = new BroadcastReceiver() {
                @Override
                public void onReceive(Context context, Intent intent) {
                    String action = intent.getAction();
                    if (BluetoothDevice.ACTION_FOUND.equals(action)) {
                        BluetoothDevice device = intent.getParcelableExtra(BluetoothDevice.EXTRA_DEVICE);
                        if (device != null) {
                            String address = device.getAddress();
                            if (address != null && !address.isEmpty()) {
                                String name = device.getName();
                                if (name == null || name.isEmpty()) {
                                    name = "Unknown";
                                }
                                synchronized (deviceMap) {
                                    if (!deviceMap.containsKey(address)) {
                                        deviceMap.put(address, name);
                                    }
                                }
                            }
                        }
                    } else if (BluetoothAdapter.ACTION_DISCOVERY_FINISHED.equals(action)) {
                        latch.countDown();
                    }
                }
            };

            IntentFilter filter = new IntentFilter();
            filter.addAction(BluetoothDevice.ACTION_FOUND);
            filter.addAction(BluetoothAdapter.ACTION_DISCOVERY_FINISHED);

            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
                context.registerReceiver(receiver, filter, Context.RECEIVER_EXPORTED);
            } else {
                context.registerReceiver(receiver, filter);
            }

            // ATOMIC CHECK-AND-START
            boolean discoveryStarted;
            synchronized (lock) {
                if (cancellationRequested) {
                    throw new IOException("Discovery cancelled before start");
                }
                discoveryStarted = adapter.startDiscovery();
                if (!discoveryStarted) {
                    throw new IOException("Failed to start discovery (startDiscovery returned false)");
                }
                state = State.DISCOVERING;
                currentLatch = latch;
            }

            try {
                boolean finished = latch.await(timeoutMs, TimeUnit.MILLISECONDS);

                if (!finished) {
                    // Timeout: cancel discovery
                    if (adapter != null) {
                        adapter.cancelDiscovery();
                    }
                    // Wait a bit for cancellation broadcast
                    latch.await(500, TimeUnit.MILLISECONDS);
                }
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
                // Cancel discovery if still running
                if (adapter != null && adapter.isDiscovering()) {
                    adapter.cancelDiscovery();
                }
                // Let finally cleanup, then rethrow as IOException
                throw new IOException("Discovery interrupted", e);
            }

            // Build sorted result
            List<DeviceInfo> resultList = new ArrayList<>();
            synchronized (deviceMap) {
                for (Map.Entry<String, String> entry : deviceMap.entrySet()) {
                    resultList.add(new DeviceInfo(entry.getKey(), entry.getValue()));
                }
            }
            Collections.sort(resultList, new Comparator<DeviceInfo>() {
                @Override
                public int compare(DeviceInfo a, DeviceInfo b) {
                    return a.address.compareToIgnoreCase(b.address);
                }
            });
            return resultList.toArray(new DeviceInfo[0]);

        } finally {
            // Cleanup: unregister receiver
            if (receiver != null) {
                try {
                    context.unregisterReceiver(receiver);
                } catch (IllegalArgumentException e) {
                    // Already unregistered
                }
            }
            // Cancel discovery if still running, and wait for it to stop
            if (adapter != null && adapter.isDiscovering()) {
                adapter.cancelDiscovery();
                long deadline = System.currentTimeMillis() + 500;
                while (System.currentTimeMillis() < deadline && adapter.isDiscovering()) {
                    try {
                        Thread.sleep(50);
                    } catch (InterruptedException e) {
                        Thread.currentThread().interrupt();
                        break;
                    }
                }
            }
            // Reset state
            synchronized (lock) {
                state = State.FINISHED;
                currentLatch = null;
                discoveryRunning = false;
                cancellationRequested = false;
            }
        }
    }

    public static void cancelDiscovery(Context context) {
        BluetoothAdapter adapter = null;
        try {
            adapter = BluetoothAdapter.getDefaultAdapter();
        } catch (SecurityException e) {
            // Permission error: ignore and still try to wake up latch
        }

        synchronized (lock) {
            cancellationRequested = true;
            if (state == State.DISCOVERING) {
                if (adapter != null && adapter.isDiscovering()) {
                    adapter.cancelDiscovery();
                }
                if (currentLatch != null) {
                    currentLatch.countDown();
                }
            }
        }
    }

    public static boolean ensureDiscoveryStopped(Context context, long timeoutMs) {
        if (timeoutMs <= 0) {
            return false;
        }
        cancelDiscovery(context);

        BluetoothAdapter adapter = null;
        try {
            adapter = BluetoothAdapter.getDefaultAdapter();
        } catch (SecurityException e) {
            // Permission error: we still need to wait for discoveryRunning
        }

        long deadline = System.currentTimeMillis() + timeoutMs;
        while (System.currentTimeMillis() < deadline) {
            boolean running = discoveryRunning;
            boolean discovering = adapter != null && adapter.isDiscovering();
            if (!running && !discovering) {
                return true;
            }
            try {
                Thread.sleep(50);
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
                return !discoveryRunning && (adapter == null || !adapter.isDiscovering());
            }
        }
        return !discoveryRunning && (adapter == null || !adapter.isDiscovering());
    }

    public static boolean isDiscovering(Context context) {
        BluetoothAdapter adapter = BluetoothAdapter.getDefaultAdapter();
        return adapter != null && adapter.isDiscovering();
    }
}