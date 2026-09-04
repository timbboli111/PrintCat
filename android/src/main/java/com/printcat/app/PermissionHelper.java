package com.printcat.app;

import android.app.Activity;
import android.content.pm.PackageManager;
import android.util.Log;

public class PermissionHelper {
    private static final String TAG = "PrintCatPermission";

    public static void requestPermission(Activity activity, String permission, int requestCode) {
        Log.d(TAG, "requestPermission called: activity=" + activity + ", permission=" + permission + ", requestCode=" + requestCode);
        if (activity == null) {
            Log.e(TAG, "activity is null");
            return;
        }
        activity.runOnUiThread(new Runnable() {
            @Override
            public void run() {
                Log.d(TAG, "runOnUiThread: requesting permission " + permission);
                activity.requestPermissions(new String[]{permission}, requestCode);
                Log.d(TAG, "requestPermissions() called");
            }
        });
    }
}