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

// Struct to hold device info
typedef struct {
    char* address;
    char* name;
} DeviceInfo;

// Helper to get exception message
static char* get_exception_message(JNIEnv* env) {
    jthrowable exc = (*env)->ExceptionOccurred(env);
    if (exc == NULL) return NULL;
    (*env)->ExceptionClear(env);

    jclass excClass = (*env)->GetObjectClass(env, exc);
    jmethodID getMessage = (*env)->GetMethodID(env, excClass, "getMessage", "()Ljava/lang/String;");
    if (getMessage == NULL) {
        (*env)->DeleteLocalRef(env, excClass);
        (*env)->DeleteLocalRef(env, exc);
        char* unknown = (char*)malloc(18);
        if (unknown) memcpy(unknown, "unknown exception", 18);
        return unknown;
    }
    jstring msg = (*env)->CallObjectMethod(env, exc, getMessage);
    const char* msgStr = (*env)->GetStringUTFChars(env, msg, NULL);
    size_t len = strlen(msgStr);
    char* result = (char*)malloc(len + 1);
    if (result) memcpy(result, msgStr, len + 1);
    (*env)->ReleaseStringUTFChars(env, msg, msgStr);
    (*env)->DeleteLocalRef(env, msg);
    (*env)->DeleteLocalRef(env, excClass);
    (*env)->DeleteLocalRef(env, exc);
    return result;
}

// Free DeviceInfo array
static void free_device_infos(DeviceInfo* infos, int count) {
    if (infos == NULL) return;
    for (int i = 0; i < count; i++) {
        if (infos[i].address) free(infos[i].address);
        if (infos[i].name) free(infos[i].name);
    }
    free(infos);
}

// Main discovery function: load helper, call startDiscovery, parse DeviceInfo
static DeviceInfo* bluetooth_discovery(JNIEnv* env, jobject context, long timeoutMs, int* count, char** error_msg) {
    if (error_msg) *error_msg = NULL;
    *count = 0;

    // 1. Get Context class
    jclass contextClass = (*env)->GetObjectClass(env, context);
    if (contextClass == NULL) {
        char* msg = get_exception_message(env);
        if (msg) { *error_msg = msg; } else { *error_msg = strdup("failed to get context class"); }
        return NULL;
    }

    // 2. Get getClassLoader method
    jmethodID getClassLoader = (*env)->GetMethodID(env, contextClass, "getClassLoader", "()Ljava/lang/ClassLoader;");
    if (getClassLoader == NULL) {
        char* msg = get_exception_message(env);
        if (msg) { *error_msg = msg; } else { *error_msg = strdup("failed to get class loader method"); }
        (*env)->DeleteLocalRef(env, contextClass);
        return NULL;
    }

    // 3. Get class loader instance
    jobject classLoader = (*env)->CallObjectMethod(env, context, getClassLoader);
    (*env)->DeleteLocalRef(env, contextClass);
    if (classLoader == NULL) {
        char* msg = get_exception_message(env);
        if (msg) { *error_msg = msg; } else { *error_msg = strdup("failed to get class loader instance"); }
        return NULL;
    }

    // 4. Get ClassLoader class
    jclass loaderClass = (*env)->FindClass(env, "java/lang/ClassLoader");
    if (loaderClass == NULL) {
        char* msg = get_exception_message(env);
        if (msg) { *error_msg = msg; } else { *error_msg = strdup("failed to find ClassLoader class"); }
        (*env)->DeleteLocalRef(env, classLoader);
        return NULL;
    }

    // 5. Get loadClass method
    jmethodID loadClass = (*env)->GetMethodID(env, loaderClass, "loadClass", "(Ljava/lang/String;)Ljava/lang/Class;");
    if (loadClass == NULL) {
        char* msg = get_exception_message(env);
        if (msg) { *error_msg = msg; } else { *error_msg = strdup("failed to get loadClass method"); }
        (*env)->DeleteLocalRef(env, loaderClass);
        (*env)->DeleteLocalRef(env, classLoader);
        return NULL;
    }

    // 6. Load BluetoothDiscoveryHelper class
    jstring className = (*env)->NewStringUTF(env, "org.golang.app.BluetoothDiscoveryHelper");
    if (className == NULL) {
        char* msg = get_exception_message(env);
        if (msg) { *error_msg = msg; } else { *error_msg = strdup("failed to create class name string"); }
        (*env)->DeleteLocalRef(env, loaderClass);
        (*env)->DeleteLocalRef(env, classLoader);
        return NULL;
    }
    jclass helperClass = (*env)->CallObjectMethod(env, classLoader, loadClass, className);
    (*env)->DeleteLocalRef(env, className);
    (*env)->DeleteLocalRef(env, classLoader);
    (*env)->DeleteLocalRef(env, loaderClass);

    if (helperClass == NULL) {
        char* msg = get_exception_message(env);
        if (msg) { *error_msg = msg; } else { *error_msg = strdup("failed to load BluetoothDiscoveryHelper"); }
        return NULL;
    }

    // 7. Get startDiscovery method
    jmethodID startDiscovery = (*env)->GetStaticMethodID(env, helperClass, "startDiscovery",
        "(Landroid/content/Context;J)[Lorg/golang/app/BluetoothDiscoveryHelper$DeviceInfo;");
    if (startDiscovery == NULL) {
        char* msg = get_exception_message(env);
        if (msg) { *error_msg = msg; } else { *error_msg = strdup("failed to get startDiscovery method"); }
        (*env)->DeleteLocalRef(env, helperClass);
        return NULL;
    }

    // 8. Call startDiscovery with cast to jlong
    jobjectArray resultArray = (*env)->CallStaticObjectMethod(env, helperClass, startDiscovery, context, (jlong)timeoutMs);
    if ((*env)->ExceptionCheck(env)) {
        char* msg = get_exception_message(env);
        if (msg) { *error_msg = msg; } else { *error_msg = strdup("Java exception in startDiscovery"); }
        (*env)->DeleteLocalRef(env, helperClass);
        return NULL;
    }

    if (resultArray == NULL) {
        (*env)->DeleteLocalRef(env, helperClass);
        *count = 0;
        return NULL;
    }

    // 9. Get array length
    jsize len = (*env)->GetArrayLength(env, resultArray);
    if (len == 0) {
        (*env)->DeleteLocalRef(env, resultArray);
        (*env)->DeleteLocalRef(env, helperClass);
        *count = 0;
        return NULL;
    }

    // 10. Get DeviceInfo class from first element (instead of FindClass)
    jobject firstElement = (*env)->GetObjectArrayElement(env, resultArray, 0);
    if (firstElement == NULL) {
        char* msg = get_exception_message(env);
        if (msg) { *error_msg = msg; } else { *error_msg = strdup("failed to get first DeviceInfo element"); }
        (*env)->DeleteLocalRef(env, resultArray);
        (*env)->DeleteLocalRef(env, helperClass);
        return NULL;
    }
    jclass deviceInfoClass = (*env)->GetObjectClass(env, firstElement);
    (*env)->DeleteLocalRef(env, firstElement);
    if (deviceInfoClass == NULL) {
        char* msg = get_exception_message(env);
        if (msg) { *error_msg = msg; } else { *error_msg = strdup("failed to get DeviceInfo class"); }
        (*env)->DeleteLocalRef(env, resultArray);
        (*env)->DeleteLocalRef(env, helperClass);
        return NULL;
    }

    // 11. Get field IDs
    jfieldID addressField = (*env)->GetFieldID(env, deviceInfoClass, "address", "Ljava/lang/String;");
    jfieldID nameField = (*env)->GetFieldID(env, deviceInfoClass, "name", "Ljava/lang/String;");
    if (addressField == NULL || nameField == NULL) {
        char* msg = get_exception_message(env);
        if (msg) { *error_msg = msg; } else { *error_msg = strdup("failed to get DeviceInfo fields"); }
        (*env)->DeleteLocalRef(env, deviceInfoClass);
        (*env)->DeleteLocalRef(env, resultArray);
        (*env)->DeleteLocalRef(env, helperClass);
        return NULL;
    }

    // 12. Allocate DeviceInfo array
    DeviceInfo* infos = (DeviceInfo*)malloc(sizeof(DeviceInfo) * len);
    if (infos == NULL) {
        char* msg = get_exception_message(env);
        if (msg) { *error_msg = msg; } else { *error_msg = strdup("failed to allocate memory"); }
        (*env)->DeleteLocalRef(env, deviceInfoClass);
        (*env)->DeleteLocalRef(env, resultArray);
        (*env)->DeleteLocalRef(env, helperClass);
        return NULL;
    }
    memset(infos, 0, sizeof(DeviceInfo) * len);

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

			var count C.int
			var errMsg *C.char
			infos := C.bluetooth_discovery(env, ctxObj, C.long(discoveryTimeout.Milliseconds()), &count, &errMsg)
			if errMsg != nil {
				defer C.free(unsafe.Pointer(errMsg))
				return fmt.Errorf("JNI error: %s", C.GoString(errMsg))
			}
			if infos == nil || count == 0 {
				return nil
			}
			defer C.free_device_infos(infos, count)

			// Convert C array to Go slice
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
	}()
	select {
	case <-done:
		return devices, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (d *discoveryAndroid) RequestAccess(ctx context.Context, device Device) error {
	return nil
}

func newDiscovery() Integration {
	return &discoveryAndroid{}
}
