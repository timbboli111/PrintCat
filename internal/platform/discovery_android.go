//go:build android

package platform

import (
	"context"
	"fmt"
	"time"
	"unsafe"

	"fyne.io/fyne/v2/driver"
	"github.com/timboli111/PrintCat/internal/printer"
)

/*
#include <jni.h>
#include <stdlib.h>
#include <string.h>

typedef struct {
    char* address;
    char* name;
} DeviceInfo;

static DeviceInfo* java_start_discovery(JNIEnv* env, jobject ctx, long timeoutMs, int* count, char** error_msg) {
    if (error_msg) *error_msg = NULL;
    *count = 0;

    jclass helperClass = (*env)->FindClass(env, "com/printcat/app/BluetoothDiscoveryHelper");
    if (helperClass == NULL) {
        if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env);
        if (error_msg) *error_msg = strdup("BluetoothDiscoveryHelper class not found");
        return NULL;
    }

    jmethodID startDiscovery = (*env)->GetStaticMethodID(env, helperClass, "startDiscovery",
        "(Landroid/content/Context;J)[Lcom/printcat/app/BluetoothDiscoveryHelper$DeviceInfo;");
    if (startDiscovery == NULL) {
        (*env)->DeleteLocalRef(env, helperClass);
        if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env);
        if (error_msg) *error_msg = strdup("startDiscovery method not found");
        return NULL;
    }

    jobjectArray resultArray = (*env)->CallStaticObjectMethod(env, helperClass, startDiscovery, ctx, (jlong)timeoutMs);
    if ((*env)->ExceptionCheck(env)) {
        jthrowable exc = (*env)->ExceptionOccurred(env);
        (*env)->ExceptionClear(env);
        jclass excClass = (*env)->GetObjectClass(env, exc);
        jmethodID getMessage = (*env)->GetMethodID(env, excClass, "getMessage", "()Ljava/lang/String;");
        if (getMessage != NULL) {
            jstring msg = (*env)->CallObjectMethod(env, exc, getMessage);
            const char* msgStr = (*env)->GetStringUTFChars(env, msg, NULL);
            if (error_msg) *error_msg = strdup(msgStr);
            (*env)->ReleaseStringUTFChars(env, msg, msgStr);
            (*env)->DeleteLocalRef(env, msg);
        } else {
            if (error_msg) *error_msg = strdup("Java exception in startDiscovery");
        }
        (*env)->DeleteLocalRef(env, excClass);
        (*env)->DeleteLocalRef(env, exc);
        (*env)->DeleteLocalRef(env, helperClass);
        return NULL;
    }

    if (resultArray == NULL) {
        (*env)->DeleteLocalRef(env, helperClass);
        *count = 0;
        return NULL;
    }

    jsize len = (*env)->GetArrayLength(env, resultArray);
    if (len == 0) {
        (*env)->DeleteLocalRef(env, resultArray);
        (*env)->DeleteLocalRef(env, helperClass);
        *count = 0;
        return NULL;
    }

    DeviceInfo* infos = (DeviceInfo*)malloc(sizeof(DeviceInfo) * len);
    if (infos == NULL) {
        (*env)->DeleteLocalRef(env, resultArray);
        (*env)->DeleteLocalRef(env, helperClass);
        if (error_msg) *error_msg = strdup("Failed to allocate memory");
        return NULL;
    }
    memset(infos, 0, sizeof(DeviceInfo) * len);

    jclass deviceInfoClass = (*env)->FindClass(env, "com/printcat/app/BluetoothDiscoveryHelper$DeviceInfo");
    if (deviceInfoClass == NULL) {
        free(infos);
        (*env)->DeleteLocalRef(env, resultArray);
        (*env)->DeleteLocalRef(env, helperClass);
        if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env);
        if (error_msg) *error_msg = strdup("DeviceInfo class not found");
        return NULL;
    }

    jfieldID addressField = (*env)->GetFieldID(env, deviceInfoClass, "address", "Ljava/lang/String;");
    jfieldID nameField = (*env)->GetFieldID(env, deviceInfoClass, "name", "Ljava/lang/String;");
    if (addressField == NULL || nameField == NULL) {
        (*env)->DeleteLocalRef(env, deviceInfoClass);
        free(infos);
        (*env)->DeleteLocalRef(env, resultArray);
        (*env)->DeleteLocalRef(env, helperClass);
        if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env);
        if (error_msg) *error_msg = strdup("DeviceInfo fields not found");
        return NULL;
    }

    int idx = 0;
    for (jsize i = 0; i < len; i++) {
        jobject deviceInfo = (*env)->GetObjectArrayElement(env, resultArray, i);
        if (deviceInfo == NULL) continue;

        jstring jAddress = (*env)->GetObjectField(env, deviceInfo, addressField);
        jstring jName = (*env)->GetObjectField(env, deviceInfo, nameField);

        const char* addrStr = NULL;
        const char* nameStr = NULL;
        if (jAddress != NULL) {
            addrStr = (*env)->GetStringUTFChars(env, jAddress, NULL);
        }
        if (jName != NULL) {
            nameStr = (*env)->GetStringUTFChars(env, jName, NULL);
        }

        if (addrStr != NULL && strlen(addrStr) > 0) {
            infos[idx].address = strdup(addrStr);
            if (nameStr != NULL && strlen(nameStr) > 0) {
                infos[idx].name = strdup(nameStr);
            } else {
                infos[idx].name = strdup("Unknown");
            }
            idx++;
        }

        if (addrStr != NULL) {
            (*env)->ReleaseStringUTFChars(env, jAddress, addrStr);
        }
        if (nameStr != NULL) {
            (*env)->ReleaseStringUTFChars(env, jName, nameStr);
        }
        if (jAddress != NULL) {
            (*env)->DeleteLocalRef(env, jAddress);
        }
        if (jName != NULL) {
            (*env)->DeleteLocalRef(env, jName);
        }
        (*env)->DeleteLocalRef(env, deviceInfo);
    }

    *count = idx;
    (*env)->DeleteLocalRef(env, deviceInfoClass);
    (*env)->DeleteLocalRef(env, resultArray);
    (*env)->DeleteLocalRef(env, helperClass);

    return infos;
}

static void free_device_infos(DeviceInfo* infos, int count) {
    if (infos == NULL) return;
    for (int i = 0; i < count; i++) {
        if (infos[i].address) free(infos[i].address);
        if (infos[i].name) free(infos[i].name);
    }
    free(infos);
}
*/
import "C"

const discoveryTimeout = 15 * time.Second

type discoveryAndroid struct{}

func (d *discoveryAndroid) Discover(ctx context.Context, kind printer.TransportKind) ([]Device, error) {
	if kind != printer.BluetoothClassic && kind != "" {
		return []Device{}, nil
	}

	var devices []Device
	var err error

	// RunNative is blocking; it will not return until Java discovery finishes or times out.
	// Since Discover is called from a goroutine in UI, this is safe.
	err = driver.RunNative(func(raw interface{}) error {
		ac, ok := raw.(*driver.AndroidContext)
		if !ok {
			return fmt.Errorf("failed to get Android context")
		}
		env := (*C.JNIEnv)(unsafe.Pointer(ac.Env))
		ctxObj := (C.jobject)(unsafe.Pointer(ac.Ctx))

		timeoutMs := C.long(discoveryTimeout.Milliseconds())

		var count C.int
		var errMsg *C.char
		infos := C.java_start_discovery(env, ctxObj, timeoutMs, &count, &errMsg)
		if errMsg != nil {
			defer C.free(unsafe.Pointer(errMsg))
			return fmt.Errorf("JNI error: %s", C.GoString(errMsg))
		}
		if infos == nil || count == 0 {
			return nil
		}
		defer C.free_device_infos(infos, count)

		cInfos := unsafe.Slice(infos, count)
		for _, info := range cInfos {
			if info.address == nil || C.strlen(info.address) == 0 {
				continue
			}
			address := C.GoString(info.address)
			name := C.GoString(info.name)
			if name == "" {
				name = "Unknown"
			}
			devices = append(devices, Device{
				ID:       address,
				Name:     name,
				Kind:     printer.BluetoothClassic,
				Endpoint: address,
				Profile: printer.PrinterProfile{
					SupportedProtocols:  []printer.Protocol{},
					SupportedTransports: []printer.TransportKind{printer.BluetoothClassic},
				},
			})
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return devices, nil
}

func (d *discoveryAndroid) RequestAccess(ctx context.Context, device Device) error {
	return nil
}

func newDiscovery() Integration {
	return &discoveryAndroid{}
}
