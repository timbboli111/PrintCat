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

static char* get_exception_message(JNIEnv* env) {
    jthrowable exc = (*env)->ExceptionOccurred(env);
    if (exc == NULL) return NULL;
    (*env)->ExceptionClear(env);

    jclass excClass = (*env)->GetObjectClass(env, exc);
    jmethodID getMessage = (*env)->GetMethodID(env, excClass, "getMessage", "()Ljava/lang/String;");
    if (getMessage == NULL) {
        (*env)->DeleteLocalRef(env, excClass);
        (*env)->DeleteLocalRef(env, exc);
        return strdup("unknown exception");
    }
    jstring msg = (*env)->CallObjectMethod(env, exc, getMessage);
    const char* msgStr = (*env)->GetStringUTFChars(env, msg, NULL);
    char* result = strdup(msgStr);
    (*env)->ReleaseStringUTFChars(env, msg, msgStr);
    (*env)->DeleteLocalRef(env, msg);
    (*env)->DeleteLocalRef(env, excClass);
    (*env)->DeleteLocalRef(env, exc);
    return result;
}

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

static int request_permission(JNIEnv* env, jobject context, const char* perm, char** error_msg) {
    if (error_msg) *error_msg = NULL;

    // 1. Get Context.getClassLoader()
    jclass contextClass = (*env)->GetObjectClass(env, context);
    if (contextClass == NULL) {
        char* msg = get_exception_message(env);
        if (msg) { *error_msg = msg; } else { *error_msg = strdup("failed to get context class"); }
        return -1;
    }
    jmethodID getClassLoader = (*env)->GetMethodID(env, contextClass, "getClassLoader", "()Ljava/lang/ClassLoader;");
    if (getClassLoader == NULL) {
        char* msg = get_exception_message(env);
        if (msg) { *error_msg = msg; } else { *error_msg = strdup("failed to get class loader method"); }
        (*env)->DeleteLocalRef(env, contextClass);
        return -1;
    }
    jobject classLoader = (*env)->CallObjectMethod(env, context, getClassLoader);
    (*env)->DeleteLocalRef(env, contextClass);
    if (classLoader == NULL) {
        char* msg = get_exception_message(env);
        if (msg) { *error_msg = msg; } else { *error_msg = strdup("failed to get class loader instance"); }
        return -1;
    }

    // 2. ClassLoader.loadClass("com.printcat.app.PermissionHelper")
    jclass loaderClass = (*env)->FindClass(env, "java/lang/ClassLoader");
    if (loaderClass == NULL) {
        char* msg = get_exception_message(env);
        if (msg) { *error_msg = msg; } else { *error_msg = strdup("failed to find ClassLoader class"); }
        (*env)->DeleteLocalRef(env, classLoader);
        return -1;
    }
    jmethodID loadClass = (*env)->GetMethodID(env, loaderClass, "loadClass", "(Ljava/lang/String;)Ljava/lang/Class;");
    if (loadClass == NULL) {
        char* msg = get_exception_message(env);
        if (msg) { *error_msg = msg; } else { *error_msg = strdup("failed to get loadClass method"); }
        (*env)->DeleteLocalRef(env, loaderClass);
        (*env)->DeleteLocalRef(env, classLoader);
        return -1;
    }

    jstring className = (*env)->NewStringUTF(env, "com.printcat.app.PermissionHelper");
    if (className == NULL) {
        char* msg = get_exception_message(env);
        if (msg) { *error_msg = msg; } else { *error_msg = strdup("failed to create class name string"); }
        (*env)->DeleteLocalRef(env, loaderClass);
        (*env)->DeleteLocalRef(env, classLoader);
        return -1;
    }
    jclass helperClass = (*env)->CallObjectMethod(env, classLoader, loadClass, className);
    (*env)->DeleteLocalRef(env, className);
    (*env)->DeleteLocalRef(env, classLoader);
    (*env)->DeleteLocalRef(env, loaderClass);

    if (helperClass == NULL) {
        char* msg = get_exception_message(env);
        if (msg) { *error_msg = msg; } else { *error_msg = strdup("failed to load PermissionHelper class"); }
        return -1;
    }

    // 3. Get static method ID
    jmethodID requestPermMethod = (*env)->GetStaticMethodID(
        env,
        helperClass,
        "requestPermission",
        "(Landroid/app/Activity;Ljava/lang/String;I)V"
    );
    if (requestPermMethod == NULL) {
        char* msg = get_exception_message(env);
        if (msg) { *error_msg = msg; } else { *error_msg = strdup("failed to get requestPermission method"); }
        (*env)->DeleteLocalRef(env, helperClass);
        return -1;
    }

    // 4. Create permission string
    jstring jperm = (*env)->NewStringUTF(env, perm);
    if (jperm == NULL) {
        char* msg = get_exception_message(env);
        if (msg) { *error_msg = msg; } else { *error_msg = strdup("failed to create permission string"); }
        (*env)->DeleteLocalRef(env, helperClass);
        return -1;
    }

    // 5. Call static method
    (*env)->CallStaticVoidMethod(env, helperClass, requestPermMethod, context, jperm, 42);
    (*env)->DeleteLocalRef(env, jperm);
    (*env)->DeleteLocalRef(env, helperClass);

    if ((*env)->ExceptionCheck(env)) {
        char* msg = get_exception_message(env);
        if (msg) { *error_msg = msg; } else { *error_msg = strdup("Java exception while calling PermissionHelper"); }
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

func checkBluetoothConnectPermission(ctx context.Context) (bool, error) {
	return checkPermissionSync(ctx, "android.permission.BLUETOOTH_CONNECT")
}

func ensureBluetoothConnectPermission(ctx context.Context) (bool, error) {
	return ensurePermissionSync(ctx, "android.permission.BLUETOOTH_CONNECT")
}

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

		var errMsg *C.char
		ret := C.request_permission(env, ctxObj, cperm, &errMsg)
		if ret == -1 {
			if errMsg != nil {
				defer C.free(unsafe.Pointer(errMsg))
				return fmt.Errorf("JNI request_permission: %s", C.GoString(errMsg))
			}
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
