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

static void free_device_infos(DeviceInfo* infos, int count) {
    if (infos == NULL) return;
    for (int i = 0; i < count; i++) {
        if (infos[i].address) free(infos[i].address);
        if (infos[i].name) free(infos[i].name);
    }
    free(infos);
}

static DeviceInfo* bluetooth_discovery(JNIEnv* env, jobject context, long timeoutMs, int* count, char** error_msg) {
    if (error_msg) *error_msg = NULL;
    *count = 0;

    jclass contextClass = (*env)->GetObjectClass(env, context);
    if (contextClass == NULL) {
        char* msg = get_exception_message(env);
        if (msg) { *error_msg = msg; } else { *error_msg = strdup("failed to get context class"); }
        return NULL;
    }

    jmethodID getClassLoader = (*env)->GetMethodID(env, contextClass, "getClassLoader", "()Ljava/lang/ClassLoader;");
    if (getClassLoader == NULL) {
        char* msg = get_exception_message(env);
        if (msg) { *error_msg = msg; } else { *error_msg = strdup("failed to get class loader method"); }
        (*env)->DeleteLocalRef(env, contextClass);
        return NULL;
    }

    jobject classLoader = (*env)->CallObjectMethod(env, context, getClassLoader);
    (*env)->DeleteLocalRef(env, contextClass);
    if (classLoader == NULL) {
        char* msg = get_exception_message(env);
        if (msg) { *error_msg = msg; } else { *error_msg = strdup("failed to get class loader instance"); }
        return NULL;
    }

    jclass loaderClass = (*env)->FindClass(env, "java/lang/ClassLoader");
    if (loaderClass == NULL) {
        char* msg = get_exception_message(env);
        if (msg) { *error_msg = msg; } else { *error_msg = strdup("failed to find ClassLoader class"); }
        (*env)->DeleteLocalRef(env, classLoader);
        return NULL;
    }

    jmethodID loadClass = (*env)->GetMethodID(env, loaderClass, "loadClass", "(Ljava/lang/String;)Ljava/lang/Class;");
    if (loadClass == NULL) {
        char* msg = get_exception_message(env);
        if (msg) { *error_msg = msg; } else { *error_msg = strdup("failed to get loadClass method"); }
        (*env)->DeleteLocalRef(env, loaderClass);
        (*env)->DeleteLocalRef(env, classLoader);
        return NULL;
    }

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

    jmethodID startDiscovery = (*env)->GetStaticMethodID(env, helperClass, "startDiscovery",
        "(Landroid/content/Context;J)[Lorg/golang/app/BluetoothDiscoveryHelper$DeviceInfo;");
    if (startDiscovery == NULL) {
        char* msg = get_exception_message(env);
        if (msg) { *error_msg = msg; } else { *error_msg = strdup("failed to get startDiscovery method"); }
        (*env)->DeleteLocalRef(env, helperClass);
        return NULL;
    }

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

    jsize len = (*env)->GetArrayLength(env, resultArray);
    if (len == 0) {
        (*env)->DeleteLocalRef(env, resultArray);
        (*env)->DeleteLocalRef(env, helperClass);
        *count = 0;
        return NULL;
    }

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

// USB discovery: returns an array of USB device ids as int (device_id).
// The Go side converts each id to a string endpoint. We do not need to
// carry a name here because UsbPrinterHelper.listPrinters already filters
// to printable devices; the display name is fetched separately in the Go
// layer via productName() -- but to keep the JNI surface minimal we just
// return ids and a synthesized "USB printer <id>" name.
typedef struct {
    int device_id;
    char* name;
} UsbDeviceInfo;

static void free_usb_device_infos(UsbDeviceInfo* infos, int count) {
    if (infos == NULL) return;
    for (int i = 0; i < count; i++) {
        if (infos[i].name) free(infos[i].name);
    }
    free(infos);
}

static UsbDeviceInfo* usb_discovery(JNIEnv* env, jobject context, int* count, char** error_msg) {
    if (error_msg) *error_msg = NULL;
    *count = 0;

    jclass helperClass = (*env)->FindClass(env, "com/printcat/app/UsbPrinterHelper");
    if (helperClass == NULL) {
        char* msg = get_exception_message(env);
        if (msg) { *error_msg = msg; } else { *error_msg = strdup("UsbPrinterHelper class not found"); }
        return NULL;
    }

    jmethodID listPrinters = (*env)->GetStaticMethodID(env, helperClass, "listPrinters",
        "(Landroid/content/Context;)[Lcom/printcat/app/UsbPrinterHelper$DeviceInfo;");
    if (listPrinters == NULL) {
        char* msg = get_exception_message(env);
        if (msg) { *error_msg = msg; } else { *error_msg = strdup("UsbPrinterHelper.listPrinters not found"); }
        (*env)->DeleteLocalRef(env, helperClass);
        return NULL;
    }

    jobjectArray resultArray = (*env)->CallStaticObjectMethod(env, helperClass, listPrinters, context);
    if ((*env)->ExceptionCheck(env)) {
        char* msg = get_exception_message(env);
        if (msg) { *error_msg = msg; } else { *error_msg = strdup("Java exception in listPrinters"); }
        (*env)->DeleteLocalRef(env, helperClass);
        return NULL;
    }
    if (resultArray == NULL) {
        (*env)->DeleteLocalRef(env, helperClass);
        return NULL;
    }

    jsize len = (*env)->GetArrayLength(env, resultArray);
    if (len == 0) {
        (*env)->DeleteLocalRef(env, resultArray);
        (*env)->DeleteLocalRef(env, helperClass);
        return NULL;
    }

    jobject firstElement = (*env)->GetObjectArrayElement(env, resultArray, 0);
    if (firstElement == NULL) {
        (*env)->DeleteLocalRef(env, resultArray);
        (*env)->DeleteLocalRef(env, helperClass);
        return NULL;
    }
    jclass infoClass = (*env)->GetObjectClass(env, firstElement);
    (*env)->DeleteLocalRef(env, firstElement);
    if (infoClass == NULL) {
        (*env)->DeleteLocalRef(env, resultArray);
        (*env)->DeleteLocalRef(env, helperClass);
        return NULL;
    }

    jfieldID idField = (*env)->GetFieldID(env, infoClass, "deviceId", "I");
    jfieldID nameField = (*env)->GetFieldID(env, infoClass, "name", "Ljava/lang/String;");
    if (idField == NULL || nameField == NULL) {
        char* msg = get_exception_message(env);
        if (msg) { *error_msg = msg; } else { *error_msg = strdup("UsbPrinterHelper.DeviceInfo fields not found"); }
        (*env)->DeleteLocalRef(env, infoClass);
        (*env)->DeleteLocalRef(env, resultArray);
        (*env)->DeleteLocalRef(env, helperClass);
        return NULL;
    }

    UsbDeviceInfo* infos = (UsbDeviceInfo*)malloc(sizeof(UsbDeviceInfo) * len);
    if (infos == NULL) {
        (*env)->DeleteLocalRef(env, infoClass);
        (*env)->DeleteLocalRef(env, resultArray);
        (*env)->DeleteLocalRef(env, helperClass);
        return NULL;
    }
    memset(infos, 0, sizeof(UsbDeviceInfo) * len);

    int idx = 0;
    for (jsize i = 0; i < len; i++) {
        jobject info = (*env)->GetObjectArrayElement(env, resultArray, i);
        if (info == NULL) continue;

        jint devId = (*env)->GetIntField(env, info, idField);
        jstring jName = (*env)->GetObjectField(env, info, nameField);
        const char* nameStr = NULL;
        if (jName != NULL) {
            nameStr = (*env)->GetStringUTFChars(env, jName, NULL);
        }
        infos[idx].device_id = (int)devId;
        if (nameStr != NULL && strlen(nameStr) > 0) {
            infos[idx].name = strdup(nameStr);
        } else {
            infos[idx].name = strdup("USB printer");
        }
        idx++;

        if (nameStr != NULL) {
            (*env)->ReleaseStringUTFChars(env, jName, nameStr);
        }
        if (jName != NULL) {
            (*env)->DeleteLocalRef(env, jName);
        }
        (*env)->DeleteLocalRef(env, info);
    }

    *count = idx;
    (*env)->DeleteLocalRef(env, infoClass);
    (*env)->DeleteLocalRef(env, resultArray);
    (*env)->DeleteLocalRef(env, helperClass);

    return infos;
}
*/
import "C"

const discoveryTimeout = 15 * time.Second

type discoveryAndroid struct{}

func (d *discoveryAndroid) Discover(ctx context.Context, kind printer.TransportKind) ([]Device, error) {
	switch kind {
	case printer.USB:
		return d.discoverUSB(ctx)
	case printer.BluetoothClassic, "":
		return d.discoverBluetooth(ctx)
	default:
		return []Device{}, nil
	}
}

func (d *discoveryAndroid) discoverBluetooth(ctx context.Context) ([]Device, error) {
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

func (d *discoveryAndroid) discoverUSB(ctx context.Context) ([]Device, error) {
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
			infos := C.usb_discovery(env, ctxObj, &count, &errMsg)
			if errMsg != nil {
				defer C.free(unsafe.Pointer(errMsg))
				return fmt.Errorf("JNI error: %s", C.GoString(errMsg))
			}
			if infos == nil || count == 0 {
				return nil
			}
			defer C.free_usb_device_infos(infos, count)

			cInfos := unsafe.Slice(infos, count)
			for _, info := range cInfos {
				idStr := fmt.Sprintf("%d", int(info.device_id))
				name := C.GoString(info.name)
				if name == "" {
					name = "USB printer"
				}
				devices = append(devices, Device{
					ID:       idStr,
					Name:     name,
					Kind:     printer.USB,
					Endpoint: idStr,
					Profile: printer.PrinterProfile{
						SupportedProtocols:  []printer.Protocol{},
						SupportedTransports: []printer.TransportKind{printer.USB},
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
