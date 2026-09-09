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
