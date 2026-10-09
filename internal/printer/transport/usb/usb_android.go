//go:build android

package usb

import (
	"context"
	"fmt"
	"unsafe"

	"fyne.io/fyne/v2/driver"
)

/*
#include <jni.h>
#include <stdlib.h>
#include <string.h>

// printcat_usb_send: opens the USB device, claims its printer (or vendor)
// interface, finds a bulk OUT endpoint, and writes the payload. Returns 0
// on success, negative values for structured failures (see the Go wrapper
// for the mapping).
static int printcat_usb_send(JNIEnv* env, jobject context, int device_id,
                             const unsigned char* payload, size_t payload_len,
                             long timeout_ms, char** error_msg) {
    if (error_msg) *error_msg = NULL;

    // Get UsbManager
    jclass contextClass = (*env)->FindClass(env, "android/content/Context");
    if (contextClass == NULL) {
        if (error_msg) *error_msg = strdup("FindClass(Context) failed");
        return -1;
    }
    jmethodID getSystemService = (*env)->GetMethodID(env, contextClass,
        "getSystemService", "(Ljava/lang/String;)Ljava/lang/Object;");
    if (getSystemService == NULL) {
        (*env)->DeleteLocalRef(env, contextClass);
        if (error_msg) *error_msg = strdup("getSystemService method not found");
        return -1;
    }
    jstring serviceName = (*env)->NewStringUTF(env, "usb");
    jobject usbManager = (*env)->CallObjectMethod(env, context, getSystemService, serviceName);
    (*env)->DeleteLocalRef(env, serviceName);
    (*env)->DeleteLocalRef(env, contextClass);
    if ((*env)->ExceptionCheck(env)) {
        (*env)->ExceptionClear(env);
        if (error_msg) *error_msg = strdup("getSystemService(\"usb\") threw");
        return -1;
    }
    if (usbManager == NULL) {
        if (error_msg) *error_msg = strdup("UsbManager is null");
        return -1;
    }

    // Invoke UsbPrinterHelper.sendBytes(Context, int, byte[], long)
    jclass helperClass = (*env)->FindClass(env, "com/printcat/app/UsbPrinterHelper");
    if (helperClass == NULL) {
        (*env)->DeleteLocalRef(env, usbManager);
        if (error_msg) *error_msg = strdup("UsbPrinterHelper class not found");
        return -1;
    }
    jmethodID sendBytes = (*env)->GetStaticMethodID(env, helperClass, "sendBytes",
        "(Landroid/content/Context;I[BJ)I");
    if (sendBytes == NULL) {
        (*env)->DeleteLocalRef(env, helperClass);
        (*env)->DeleteLocalRef(env, usbManager);
        if (error_msg) *error_msg = strdup("UsbPrinterHelper.sendBytes not found");
        return -1;
    }

    jbyteArray arr = (*env)->NewByteArray(env, (jsize)payload_len);
    if (arr == NULL) {
        (*env)->DeleteLocalRef(env, helperClass);
        (*env)->DeleteLocalRef(env, usbManager);
        if (error_msg) *error_msg = strdup("NewByteArray failed");
        return -1;
    }
    (*env)->SetByteArrayRegion(env, arr, 0, (jsize)payload_len, (const jbyte*)payload);

    jint rc = (*env)->CallStaticIntMethod(env, helperClass, sendBytes,
        context, (jint)device_id, arr, (jlong)timeout_ms);
    if ((*env)->ExceptionCheck(env)) {
        (*env)->ExceptionClear(env);
        (*env)->DeleteLocalRef(env, arr);
        (*env)->DeleteLocalRef(env, helperClass);
        (*env)->DeleteLocalRef(env, usbManager);
        if (error_msg) *error_msg = strdup("UsbPrinterHelper.sendBytes threw");
        return -1;
    }
    (*env)->DeleteLocalRef(env, arr);
    (*env)->DeleteLocalRef(env, helperClass);
    (*env)->DeleteLocalRef(env, usbManager);
    return (int)rc;
}
*/
import "C"

func (t *USBTransport) send(ctx context.Context, endpoint string, payload []byte, options map[string]string) error {
	if len(payload) == 0 {
		return nil
	}

	// endpoint is the decimal representation of the Android USB device id.
	var deviceID int
	if _, err := fmt.Sscanf(endpoint, "%d", &deviceID); err != nil {
		return fmt.Errorf("invalid usb endpoint %q: expected numeric device id", endpoint)
	}

	timeoutMs := 5000
	if v, ok := options["timeout_ms"]; ok {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
			timeoutMs = n
		}
	}

	var result error
	done := make(chan struct{})
	go func() {
		defer close(done)
		result = driver.RunNative(func(raw interface{}) error {
			ac, ok := raw.(*driver.AndroidContext)
			if !ok {
				return fmt.Errorf("failed to get Android context")
			}
			env := (*C.JNIEnv)(unsafe.Pointer(ac.Env))
			ctxObj := (C.jobject)(unsafe.Pointer(ac.Ctx))

			var errMsg *C.char
			ret := C.printcat_usb_send(
				env,
				ctxObj,
				C.int(deviceID),
				(*C.uchar)(unsafe.Pointer(&payload[0])),
				C.size_t(len(payload)),
				C.long(timeoutMs),
				&errMsg,
			)
			if errMsg != nil {
				defer C.free(unsafe.Pointer(errMsg))
			}
			if ret != 0 {
				if errMsg != nil {
					return fmt.Errorf("usb send failed (rc=%d): %s", int(ret), C.GoString(errMsg))
				}
				return fmt.Errorf("usb send failed (rc=%d)", int(ret))
			}
			return nil
		})
	}()

	select {
	case <-done:
		return result
	case <-ctx.Done():
		return ctx.Err()
	}
}
