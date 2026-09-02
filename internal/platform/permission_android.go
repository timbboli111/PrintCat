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

static int check_permission(JNIEnv* env, jobject context) {
    jclass contextClass = (*env)->FindClass(env, "android/content/Context");
    if (contextClass == NULL) return -1;
    jmethodID checkSelfPermission = (*env)->GetMethodID(env, contextClass, "checkSelfPermission", "(Ljava/lang/String;)I");
    if (checkSelfPermission == NULL) {
        (*env)->DeleteLocalRef(env, contextClass);
        return -1;
    }
    jstring perm = (*env)->NewStringUTF(env, "android.permission.BLUETOOTH_CONNECT");
    if (perm == NULL) {
        (*env)->DeleteLocalRef(env, contextClass);
        return -1;
    }
    jint result = (*env)->CallIntMethod(env, context, checkSelfPermission, perm);
    (*env)->DeleteLocalRef(env, perm);
    (*env)->DeleteLocalRef(env, contextClass);
    return result == 0 ? 1 : 0;
}

static int request_permission(JNIEnv* env, jobject context) {
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
    jstring perm = (*env)->NewStringUTF(env, "android.permission.BLUETOOTH_CONNECT");
    if (perm == NULL) {
        (*env)->DeleteLocalRef(env, permArray);
        (*env)->DeleteLocalRef(env, activityClass);
        return -1;
    }
    (*env)->SetObjectArrayElement(env, permArray, 0, perm);
    (*env)->DeleteLocalRef(env, perm);
    (*env)->CallVoidMethod(env, context, requestPermissions, permArray, 42);
    (*env)->DeleteLocalRef(env, permArray);
    (*env)->DeleteLocalRef(env, activityClass);
    if ((*env)->ExceptionCheck(env)) {
        (*env)->ExceptionClear(env);
        return -1;
    }
    return 0;
}
*/
import "C"

func checkBluetoothConnectPermission(ctx context.Context) (bool, error) {
	var granted bool
	var err error
	done := make(chan struct{})
	go func() {
		defer close(done)
		err = driver.RunNative(func(raw interface{}) error {
			ac, ok := raw.(*driver.AndroidContext)
			if !ok {
				return fmt.Errorf("failed to get Android context")
			}
			env := (*C.JNIEnv)(unsafe.Pointer(ac.Env))
			ctxObj := (C.jobject)(unsafe.Pointer(ac.Ctx))
			ret := C.check_permission(env, ctxObj)
			if ret == 1 {
				granted = true
			} else if ret == -1 {
				return fmt.Errorf("failed to check permission")
			}
			return nil
		})
	}()
	select {
	case <-done:
		return granted, err
	case <-ctx.Done():
		return false, ctx.Err()
	}
}

func requestBluetoothConnectPermission(ctx context.Context) error {
	done := make(chan struct{})
	var err error
	go func() {
		defer close(done)
		err = driver.RunNative(func(raw interface{}) error {
			ac, ok := raw.(*driver.AndroidContext)
			if !ok {
				return fmt.Errorf("failed to get Android context")
			}
			env := (*C.JNIEnv)(unsafe.Pointer(ac.Env))
			ctxObj := (C.jobject)(unsafe.Pointer(ac.Ctx))
			ret := C.request_permission(env, ctxObj)
			if ret == -1 {
				return fmt.Errorf("failed to request permission (context is not Activity?)")
			}
			return nil
		})
	}()
	select {
	case <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func ensureBluetoothConnectPermission(ctx context.Context) (bool, error) {
	granted, err := checkBluetoothConnectPermission(ctx)
	if err != nil {
		return false, err
	}
	if granted {
		return true, nil
	}
	if err := requestBluetoothConnectPermission(ctx); err != nil {
		return false, err
	}
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.After(10 * time.Second)
	for {
		select {
		case <-ticker.C:
			granted, err := checkBluetoothConnectPermission(ctx)
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
