//go:build android

package platform

import (
	"context"
	"fmt"
	"unsafe"

	"fyne.io/fyne/v2/driver"
	"github.com/timboli111/PrintCat/internal/printer"
)

/*
#include <jni.h>
#include <stdlib.h>
#include <string.h>

typedef struct {
    char* mac;
    char* name;
} DeviceInfo;

static void free_device_infos(DeviceInfo* infos, int count) {
    if (infos == NULL) return;
    for (int i = 0; i < count; i++) {
        if (infos[i].mac) free(infos[i].mac);
        if (infos[i].name) free(infos[i].name);
    }
    free(infos);
}

static DeviceInfo* bluetooth_get_bonded_devices(JNIEnv* env, jobject ctx, int* count, char** error_msg) {
    if (error_msg) *error_msg = NULL;
    *count = 0;

    jclass bluetoothManagerClass = (*env)->FindClass(env, "android/bluetooth/BluetoothManager");
    if (bluetoothManagerClass == NULL) {
        if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env);
        if (error_msg) *error_msg = strdup("BluetoothManager class not found");
        return NULL;
    }

    jclass contextClass = (*env)->FindClass(env, "android/content/Context");
    if (contextClass == NULL) {
        (*env)->DeleteLocalRef(env, bluetoothManagerClass);
        if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env);
        if (error_msg) *error_msg = strdup("Context class not found");
        return NULL;
    }

    jmethodID getSystemService = (*env)->GetMethodID(env, contextClass, "getSystemService", "(Ljava/lang/String;)Ljava/lang/Object;");
    if (getSystemService == NULL) {
        (*env)->DeleteLocalRef(env, contextClass);
        (*env)->DeleteLocalRef(env, bluetoothManagerClass);
        if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env);
        if (error_msg) *error_msg = strdup("getSystemService method not found");
        return NULL;
    }

    jstring serviceName = (*env)->NewStringUTF(env, "bluetooth");
    if (serviceName == NULL) {
        (*env)->DeleteLocalRef(env, contextClass);
        (*env)->DeleteLocalRef(env, bluetoothManagerClass);
        if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env);
        if (error_msg) *error_msg = strdup("failed to create service name string");
        return NULL;
    }

    jobject manager = (*env)->CallObjectMethod(env, ctx, getSystemService, serviceName);
    (*env)->DeleteLocalRef(env, serviceName);
    (*env)->DeleteLocalRef(env, contextClass);
    if (manager == NULL) {
        (*env)->DeleteLocalRef(env, bluetoothManagerClass);
        if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env);
        if (error_msg) *error_msg = strdup("failed to get BluetoothManager");
        return NULL;
    }

    jmethodID getAdapter = (*env)->GetMethodID(env, bluetoothManagerClass, "getAdapter", "()Landroid/bluetooth/BluetoothAdapter;");
    if (getAdapter == NULL) {
        (*env)->DeleteLocalRef(env, manager);
        (*env)->DeleteLocalRef(env, bluetoothManagerClass);
        if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env);
        if (error_msg) *error_msg = strdup("getAdapter method not found");
        return NULL;
    }

    jobject adapter = (*env)->CallObjectMethod(env, manager, getAdapter);
    (*env)->DeleteLocalRef(env, manager);
    (*env)->DeleteLocalRef(env, bluetoothManagerClass);
    if (adapter == NULL) {
        if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env);
        *count = 0;
        return NULL;
    }

    jclass adapterClass = (*env)->GetObjectClass(env, adapter);
    if (adapterClass == NULL) {
        (*env)->DeleteLocalRef(env, adapter);
        if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env);
        if (error_msg) *error_msg = strdup("BluetoothAdapter class not found");
        return NULL;
    }

    jmethodID getBondedDevices = (*env)->GetMethodID(env, adapterClass, "getBondedDevices", "()Ljava/util/Set;");
    if (getBondedDevices == NULL) {
        (*env)->DeleteLocalRef(env, adapterClass);
        (*env)->DeleteLocalRef(env, adapter);
        if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env);
        if (error_msg) *error_msg = strdup("getBondedDevices method not found");
        return NULL;
    }

    jobject bondedSet = (*env)->CallObjectMethod(env, adapter, getBondedDevices);
    (*env)->DeleteLocalRef(env, adapterClass);
    (*env)->DeleteLocalRef(env, adapter);
    if (bondedSet == NULL) {
        if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env);
        *count = 0;
        return NULL;
    }

    jclass setClass = (*env)->GetObjectClass(env, bondedSet);
    if (setClass == NULL) {
        (*env)->DeleteLocalRef(env, bondedSet);
        if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env);
        if (error_msg) *error_msg = strdup("Set class not found");
        return NULL;
    }

    jmethodID iterator = (*env)->GetMethodID(env, setClass, "iterator", "()Ljava/util/Iterator;");
    if (iterator == NULL) {
        (*env)->DeleteLocalRef(env, setClass);
        (*env)->DeleteLocalRef(env, bondedSet);
        if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env);
        if (error_msg) *error_msg = strdup("iterator method not found");
        return NULL;
    }

    jobject iter = (*env)->CallObjectMethod(env, bondedSet, iterator);
    (*env)->DeleteLocalRef(env, setClass);
    if (iter == NULL) {
        (*env)->DeleteLocalRef(env, bondedSet);
        if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env);
        *count = 0;
        return NULL;
    }

    jclass iterClass = (*env)->GetObjectClass(env, iter);
    if (iterClass == NULL) {
        (*env)->DeleteLocalRef(env, iter);
        (*env)->DeleteLocalRef(env, bondedSet);
        if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env);
        if (error_msg) *error_msg = strdup("Iterator class not found");
        return NULL;
    }

    jmethodID hasNext = (*env)->GetMethodID(env, iterClass, "hasNext", "()Z");
    jmethodID next = (*env)->GetMethodID(env, iterClass, "next", "()Ljava/lang/Object;");
    if (hasNext == NULL || next == NULL) {
        (*env)->DeleteLocalRef(env, iterClass);
        (*env)->DeleteLocalRef(env, iter);
        (*env)->DeleteLocalRef(env, bondedSet);
        if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env);
        if (error_msg) *error_msg = strdup("Iterator methods not found");
        return NULL;
    }

    int deviceCount = 0;
    jboolean hasMore = (*env)->CallBooleanMethod(env, iter, hasNext);
    while (hasMore == JNI_TRUE) {
        jobject device = (*env)->CallObjectMethod(env, iter, next);
        if (device != NULL) {
            deviceCount++;
            (*env)->DeleteLocalRef(env, device);
        }
        hasMore = (*env)->CallBooleanMethod(env, iter, hasNext);
    }

    (*env)->DeleteLocalRef(env, iter);
    iter = (*env)->CallObjectMethod(env, bondedSet, iterator);
    if (iter == NULL) {
        (*env)->DeleteLocalRef(env, bondedSet);
        if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env);
        *count = 0;
        return NULL;
    }

    DeviceInfo* infos = (DeviceInfo*)malloc(sizeof(DeviceInfo) * deviceCount);
    if (infos == NULL) {
        (*env)->DeleteLocalRef(env, iter);
        (*env)->DeleteLocalRef(env, bondedSet);
        if ((*env)->ExceptionCheck(env)) (*env)->ExceptionClear(env);
        if (error_msg) *error_msg = strdup("failed to allocate memory");
        return NULL;
    }
    memset(infos, 0, sizeof(DeviceInfo) * deviceCount);

    int idx = 0;
    hasMore = (*env)->CallBooleanMethod(env, iter, hasNext);
    while (hasMore == JNI_TRUE && idx < deviceCount) {
        jobject device = (*env)->CallObjectMethod(env, iter, next);
        if (device == NULL) {
            hasMore = (*env)->CallBooleanMethod(env, iter, hasNext);
            continue;
        }

        jclass deviceClass = (*env)->GetObjectClass(env, device);
        if (deviceClass != NULL) {
            jmethodID getName = (*env)->GetMethodID(env, deviceClass, "getName", "()Ljava/lang/String;");
            jmethodID getAddress = (*env)->GetMethodID(env, deviceClass, "getAddress", "()Ljava/lang/String;");

            if (getName != NULL && getAddress != NULL) {
                jstring jName = (*env)->CallObjectMethod(env, device, getName);
                jstring jAddress = (*env)->CallObjectMethod(env, device, getAddress);

                const char* nameStr = NULL;
                const char* addrStr = NULL;
                if (jName != NULL) {
                    nameStr = (*env)->GetStringUTFChars(env, jName, NULL);
                }
                if (jAddress != NULL) {
                    addrStr = (*env)->GetStringUTFChars(env, jAddress, NULL);
                }

                if (addrStr != NULL && strlen(addrStr) > 0) {
                    infos[idx].mac = strdup(addrStr);
                    if (nameStr != NULL && strlen(nameStr) > 0) {
                        infos[idx].name = strdup(nameStr);
                    } else {
                        infos[idx].name = strdup("Unknown");
                    }
                    idx++;
                }

                if (nameStr != NULL) {
                    (*env)->ReleaseStringUTFChars(env, jName, nameStr);
                }
                if (addrStr != NULL) {
                    (*env)->ReleaseStringUTFChars(env, jAddress, addrStr);
                }
                if (jName != NULL) {
                    (*env)->DeleteLocalRef(env, jName);
                }
                if (jAddress != NULL) {
                    (*env)->DeleteLocalRef(env, jAddress);
                }
            }
            (*env)->DeleteLocalRef(env, deviceClass);
        }
        (*env)->DeleteLocalRef(env, device);
        hasMore = (*env)->CallBooleanMethod(env, iter, hasNext);
    }

    *count = idx;
    (*env)->DeleteLocalRef(env, iter);
    (*env)->DeleteLocalRef(env, bondedSet);

    if ((*env)->ExceptionCheck(env)) {
        (*env)->ExceptionClear(env);
        free_device_infos(infos, idx);
        if (error_msg) *error_msg = strdup("exception during bonded devices iteration");
        *count = 0;
        return NULL;
    }

    return infos;
}
*/
import "C"

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
			infos := C.bluetooth_get_bonded_devices(env, ctxObj, &count, &errMsg)
			if errMsg != nil {
				defer C.free(unsafe.Pointer(errMsg))
				return fmt.Errorf("JNI error: %s", C.GoString(errMsg))
			}
			if infos == nil || count == 0 {
				return nil
			}
			defer C.free_device_infos(infos, count)

			// Convert C array to Go slice without giant array literal
			cInfos := unsafe.Slice(infos, count)
			for _, info := range cInfos {
				if info.mac == nil || C.strlen(info.mac) == 0 {
					continue
				}
				mac := C.GoString(info.mac)
				name := C.GoString(info.name)
				if name == "" {
					name = "Unknown"
				}
				devices = append(devices, Device{
					ID:       mac,
					Name:     name,
					Kind:     printer.BluetoothClassic,
					Endpoint: mac,
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
