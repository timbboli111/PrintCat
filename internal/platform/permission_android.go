//go:build android

package platform

import (
	"context"
	"fmt"
	"time"
	"unsafe"

	"fyne.io/fyne/v2/driver"
)

/*
#include <jni.h>
#include <stdlib.h>
#include <string.h>

static int check_permission(JNIEnv* env, jobject context, const char* perm) {
    jclass contextClass = (*env)->FindClass(env, "android/content/Context");
    if (contextClass == NULL) return -1;
    jmethodID checkSelfPermission = (*env)->GetMethodID(env, contextClass, "checkSelfPermission", "(Ljava/lang/String;)I");
    if (checkSelfPermission == NULL) {
        (*env)->DeleteLocalRef(env, contextClass);
        return -1;
    }
    jstring jperm = (*env)->NewStringUTF(env, perm);
    if (jperm == NULL) {
        (*env)->DeleteLocalRef(env, contextClass);
        return -1;
    }
    jint result = (*env)->CallIntMethod(env, context, checkSelfPermission, jperm);
    (*env)->DeleteLocalRef(env, jperm);
    (*env)->DeleteLocalRef(env, contextClass);
    return result == 0 ? 1 : 0;
}

static int request_permission(JNIEnv* env, jobject context, const char* perm) {
    jclass activityClass = (*env)->FindClass(env, "android/app/Activity");
    if (activityClass == NULL) return -1;
    if (!(*env)->IsInstanceOf(env, context, activityClass)) {
        (*env)->DeleteLocalRef(env, activityClass);
        return -1;
    }
    jmethodID requestPermissions = (*env)->GetMethodID(env, activityClass, "requestPermissions", "([Ljava/lang/String;I)V");
    if (requestPermissions == NULL) {
        (*env)->DeleteLocalRef(env, activityClass);
        return -1;
    }
    jclass stringClass = (*env)->FindClass(env, "java/lang/String");
    if (stringClass == NULL) {
        (*env)->DeleteLocalRef(env, activityClass);
        return -1;
    }
    jobjectArray permArray = (*env)->NewObjectArray(env, 1, stringClass, NULL);
    (*env)->DeleteLocalRef(env, stringClass);
    if (permArray == NULL) {
        (*env)->DeleteLocalRef(env, activityClass);
        return -1;
    }
    jstring jperm = (*env)->NewStringUTF(env, perm);
    if (jperm == NULL) {
        (*env)->DeleteLocalRef(env, permArray);
        (*env)->DeleteLocalRef(env, activityClass);
        return -1;
    }
    (*env)->SetObjectArrayElement(env, permArray, 0, jperm);
    (*env)->DeleteLocalRef(env, jperm);
    (*env)->CallVoidMethod(env, context, requestPermissions, permArray, 42);
    (*env)->DeleteLocalRef(env, permArray);
    (*env)->DeleteLocalRef(env, activityClass);
    if ((*env)->ExceptionCheck(env)) {
        (*env)->ExceptionClear(env);
        return -1;
    }
    return 0;
}

// request_permissions_pair requests two permissions in a single
// Activity.requestPermissions() call. Android 12+ requires FINE and COARSE
// to be requested together so the platform can present the
// "Precise vs Approximate" choice correctly; issuing two separate requests
// does not work as intended.
static int request_permissions_pair(JNIEnv* env, jobject context, const char* perm1, const char* perm2) {
    jclass activityClass = (*env)->FindClass(env, "android/app/Activity");
    if (activityClass == NULL) return -1;
    if (!(*env)->IsInstanceOf(env, context, activityClass)) {
        (*env)->DeleteLocalRef(env, activityClass);
        return -1;
    }
    jmethodID requestPermissions = (*env)->GetMethodID(env, activityClass, "requestPermissions", "([Ljava/lang/String;I)V");
    if (requestPermissions == NULL) {
        (*env)->DeleteLocalRef(env, activityClass);
        return -1;
    }
    jclass stringClass = (*env)->FindClass(env, "java/lang/String");
    if (stringClass == NULL) {
        (*env)->DeleteLocalRef(env, activityClass);
        return -1;
    }
    jobjectArray permArray = (*env)->NewObjectArray(env, 2, stringClass, NULL);
    (*env)->DeleteLocalRef(env, stringClass);
    if (permArray == NULL) {
        (*env)->DeleteLocalRef(env, activityClass);
        return -1;
    }

    jstring jperm1 = (*env)->NewStringUTF(env, perm1);
    if (jperm1 == NULL) {
        (*env)->DeleteLocalRef(env, permArray);
        (*env)->DeleteLocalRef(env, activityClass);
        return -1;
    }
    (*env)->SetObjectArrayElement(env, permArray, 0, jperm1);
    (*env)->DeleteLocalRef(env, jperm1);

    jstring jperm2 = (*env)->NewStringUTF(env, perm2);
    if (jperm2 == NULL) {
        (*env)->DeleteLocalRef(env, permArray);
        (*env)->DeleteLocalRef(env, activityClass);
        return -1;
    }
    (*env)->SetObjectArrayElement(env, permArray, 1, jperm2);
    (*env)->DeleteLocalRef(env, jperm2);

    (*env)->CallVoidMethod(env, context, requestPermissions, permArray, 42);
    (*env)->DeleteLocalRef(env, permArray);
    (*env)->DeleteLocalRef(env, activityClass);
    if ((*env)->ExceptionCheck(env)) {
        (*env)->ExceptionClear(env);
        return -1;
    }
    return 0;
}

static int get_api_level(JNIEnv* env) {
    jclass buildClass = (*env)->FindClass(env, "android/os/Build$VERSION");
    if (buildClass == NULL) return -1;
    jfieldID sdkIntField = (*env)->GetStaticFieldID(env, buildClass, "SDK_INT", "I");
    if (sdkIntField == NULL) {
        (*env)->DeleteLocalRef(env, buildClass);
        return -1;
    }
    jint sdkInt = (*env)->GetStaticIntField(env, buildClass, sdkIntField);
    (*env)->DeleteLocalRef(env, buildClass);
    return (int)sdkInt;
}
*/
import "C"

func getAndroidAPIVersion() int {
	var apiLevel int
	_ = driver.RunNative(func(raw interface{}) error {
		ac, ok := raw.(*driver.AndroidContext)
		if !ok {
			return fmt.Errorf("failed to get Android context")
		}
		env := (*C.JNIEnv)(unsafe.Pointer(ac.Env))
		apiLevel = int(C.get_api_level(env))
		return nil
	})
	return apiLevel
}

// ---------- BLUETOOTH_CONNECT ----------

func checkBluetoothConnectPermission(ctx context.Context) (bool, error) {
	if getAndroidAPIVersion() < 31 {
		return true, nil
	}
	return checkPermissionSync(ctx, "android.permission.BLUETOOTH_CONNECT")
}

func ensureBluetoothConnectPermission(ctx context.Context) (bool, error) {
	if getAndroidAPIVersion() < 31 {
		return true, nil
	}
	return ensurePermissionSync(ctx, "android.permission.BLUETOOTH_CONNECT")
}

// ---------- BLUETOOTH_SCAN / ACCESS_FINE_LOCATION ----------

func checkBluetoothScanPermission(ctx context.Context) (bool, error) {
	perm, err := getScanPermissionNameSync(ctx)
	if err != nil {
		return false, err
	}
	return checkPermissionSync(ctx, perm)
}

func ensureBluetoothScanPermission(ctx context.Context) (bool, error) {
	perm, err := getScanPermissionNameSync(ctx)
	if err != nil {
		return false, err
	}
	return ensurePermissionSync(ctx, perm)
}

// ---------- ACCESS_FINE_LOCATION (API 31+ OEM path) ----------

// checkFineLocationPermission reports whether ACCESS_FINE_LOCATION is
// granted. On API < 31 the caller is expected to have already obtained it
// via EnsureBluetoothScanPermission (which maps to ACCESS_FINE_LOCATION on
// those API levels); this function returns true there to avoid a redundant
// check.
func checkFineLocationPermission(ctx context.Context) (bool, error) {
	if getAndroidAPIVersion() < 31 {
		return true, nil
	}
	return checkPermissionSync(ctx, "android.permission.ACCESS_FINE_LOCATION")
}

// ensureFineLocationPermission makes sure ACCESS_FINE_LOCATION is granted.
//
// On API < 31 it is a no-op returning (true, nil): the existing
// EnsureBluetoothScanPermission already requests ACCESS_FINE_LOCATION on
// those API levels, so a second request would produce a duplicate dialog.
//
// On API 31+ it:
//  1. returns true immediately if FINE is already granted;
//  2. returns false if COARSE is granted without FINE (the user previously
//     chose "Approximate" — a second dialog would not change that);
//  3. otherwise requests FINE and COARSE together in a single
//     Activity.requestPermissions call (required by Android 12+ for the
//     "Precise vs Approximate" dialog), then waits for a result:
//     - FINE granted      -> (true, nil)
//     - COARSE only       -> (false, nil)
//     - no response       -> (false, error) on timeout
func ensureFineLocationPermission(ctx context.Context) (bool, error) {
	if getAndroidAPIVersion() < 31 {
		// ACCESS_FINE_LOCATION for Classic discovery is already handled by
		// EnsureBluetoothScanPermission on these API levels.
		return true, nil
	}

	const finePerm = "android.permission.ACCESS_FINE_LOCATION"
	const coarsePerm = "android.permission.ACCESS_COARSE_LOCATION"

	fineGranted, err := checkPermissionSync(ctx, finePerm)
	if err != nil {
		return false, err
	}
	if fineGranted {
		return true, nil
	}

	coarseGranted, err := checkPermissionSync(ctx, coarsePerm)
	if err != nil {
		return false, err
	}
	if coarseGranted && !fineGranted {
		// The user previously selected "Approximate" and the platform
		// will not re-prompt without user-initiated Settings change.
		// Report the current state honestly instead of looping.
		return false, nil
	}

	if err := requestFineAndCoarseLocationSync(ctx, finePerm, coarsePerm); err != nil {
		return false, err
	}

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.After(10 * time.Second)
	for {
		select {
		case <-ticker.C:
			fineNow, err := checkPermissionSync(ctx, finePerm)
			if err != nil {
				return false, err
			}
			if fineNow {
				return true, nil
			}
			coarseNow, err := checkPermissionSync(ctx, coarsePerm)
			if err != nil {
				return false, err
			}
			if coarseNow {
				// User selected "Approximate" in the runtime dialog.
				return false, nil
			}
		case <-ctx.Done():
			return false, ctx.Err()
		case <-timeout:
			return false, fmt.Errorf("fine location permission not granted within timeout")
		}
	}
}

func requestFineAndCoarseLocationSync(ctx context.Context, finePerm, coarsePerm string) error {
	var err error
	err = driver.RunNative(func(raw interface{}) error {
		ac, ok := raw.(*driver.AndroidContext)
		if !ok {
			return fmt.Errorf("failed to get Android context")
		}
		env := (*C.JNIEnv)(unsafe.Pointer(ac.Env))
		ctxObj := (C.jobject)(unsafe.Pointer(ac.Ctx))
		cfine := C.CString(finePerm)
		defer C.free(unsafe.Pointer(cfine))
		ccoarse := C.CString(coarsePerm)
		defer C.free(unsafe.Pointer(ccoarse))
		ret := C.request_permissions_pair(env, ctxObj, cfine, ccoarse)
		if ret == -1 {
			return fmt.Errorf("failed to request fine/coarse location permissions")
		}
		return nil
	})
	return err
}

// ---------- SYNC HELPERS ----------

func getScanPermissionNameSync(ctx context.Context) (string, error) {
	var perm string
	var err error
	err = driver.RunNative(func(raw interface{}) error {
		ac, ok := raw.(*driver.AndroidContext)
		if !ok {
			return fmt.Errorf("failed to get Android context")
		}
		env := (*C.JNIEnv)(unsafe.Pointer(ac.Env))
		apiLevel := int(C.get_api_level(env))
		if apiLevel < 0 {
			return fmt.Errorf("failed to get API level")
		}
		if apiLevel >= 31 {
			perm = "android.permission.BLUETOOTH_SCAN"
		} else {
			perm = "android.permission.ACCESS_FINE_LOCATION"
		}
		return nil
	})
	return perm, err
}

func checkPermissionSync(ctx context.Context, perm string) (bool, error) {
	var granted bool
	var err error
	err = driver.RunNative(func(raw interface{}) error {
		ac, ok := raw.(*driver.AndroidContext)
		if !ok {
			return fmt.Errorf("failed to get Android context")
		}
		env := (*C.JNIEnv)(unsafe.Pointer(ac.Env))
		ctxObj := (C.jobject)(unsafe.Pointer(ac.Ctx))
		cperm := C.CString(perm)
		defer C.free(unsafe.Pointer(cperm))
		ret := C.check_permission(env, ctxObj, cperm)
		if ret == 1 {
			granted = true
		} else if ret == -1 {
			return fmt.Errorf("failed to check permission")
		}
		return nil
	})
	return granted, err
}

func requestPermissionSync(ctx context.Context, perm string) error {
	var err error
	err = driver.RunNative(func(raw interface{}) error {
		ac, ok := raw.(*driver.AndroidContext)
		if !ok {
			return fmt.Errorf("failed to get Android context")
		}
		env := (*C.JNIEnv)(unsafe.Pointer(ac.Env))
		ctxObj := (C.jobject)(unsafe.Pointer(ac.Ctx))
		cperm := C.CString(perm)
		defer C.free(unsafe.Pointer(cperm))
		ret := C.request_permission(env, ctxObj, cperm)
		if ret == -1 {
			return fmt.Errorf("failed to request permission")
		}
		return nil
	})
	return err
}

func ensurePermissionSync(ctx context.Context, perm string) (bool, error) {
	granted, err := checkPermissionSync(ctx, perm)
	if err != nil {
		return false, err
	}
	if granted {
		return true, nil
	}
	if err := requestPermissionSync(ctx, perm); err != nil {
		return false, err
	}
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.After(10 * time.Second)
	for {
		select {
		case <-ticker.C:
			granted, err := checkPermissionSync(ctx, perm)
			if err != nil {
				return false, err
			}
			if granted {
				return true, nil
			}
		case <-ctx.Done():
			return false, ctx.Err()
		case <-timeout:
			return false, fmt.Errorf("permission not granted within timeout")
		}
	}
}
