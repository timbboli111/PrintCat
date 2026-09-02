//go:build android

package bluetooth

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

// SPP UUID: 00001101-0000-1000-8000-00805F9B34FB
static const char* SPP_UUID = "00001101-0000-1000-8000-00805F9B34FB";

// Helper untuk mengecek exception dan menyiapkan pesan error.
// Mengembalikan 0 jika tidak ada exception, -1 jika ada.
// error_msg akan diisi dengan pesan exception (dialokasikan dengan malloc).
static int check_exception(JNIEnv* env, const char* context, char** error_msg) {
    if (!(*env)->ExceptionCheck(env)) {
        return 0;
    }
    jthrowable exc = (*env)->ExceptionOccurred(env);
    (*env)->ExceptionClear(env);

    jclass excClass = (*env)->GetObjectClass(env, exc);
    jmethodID getMessage = (*env)->GetMethodID(env, excClass, "getMessage", "()Ljava/lang/String;");
    if (getMessage == NULL) {
        (*env)->DeleteLocalRef(env, excClass);
        (*env)->DeleteLocalRef(env, exc);
        if (error_msg) {
            *error_msg = malloc(64);
            if (*error_msg) {
                snprintf(*error_msg, 64, "%s: unknown exception", context);
            }
        }
        return -1;
    }
    jstring msg = (*env)->CallObjectMethod(env, exc, getMessage);
    const char* msgStr = (*env)->GetStringUTFChars(env, msg, NULL);
    size_t len = strlen(msgStr) + strlen(context) + 4;
    if (error_msg) {
        *error_msg = malloc(len);
        if (*error_msg) {
            snprintf(*error_msg, len, "%s: %s", context, msgStr);
        }
    }
    (*env)->ReleaseStringUTFChars(env, msg, msgStr);
    (*env)->DeleteLocalRef(env, msg);
    (*env)->DeleteLocalRef(env, excClass);
    (*env)->DeleteLocalRef(env, exc);
    return -1;
}

// Helper untuk menutup socket dengan aman (best-effort).
static void close_socket_safe(JNIEnv* env, jobject socket) {
    if (socket == NULL) return;
    jclass socketClass = (*env)->GetObjectClass(env, socket);
    if (socketClass == NULL) {
        // Exception mungkin terjadi; bersihkan dan abaikan.
        if ((*env)->ExceptionCheck(env)) {
            (*env)->ExceptionClear(env);
        }
        return;
    }
    jmethodID close = (*env)->GetMethodID(env, socketClass, "close", "()V");
    if (close == NULL) {
        (*env)->DeleteLocalRef(env, socketClass);
        if ((*env)->ExceptionCheck(env)) {
            (*env)->ExceptionClear(env);
        }
        return;
    }
    (*env)->CallVoidMethod(env, socket, close);
    if ((*env)->ExceptionCheck(env)) {
        (*env)->ExceptionClear(env);
    }
    (*env)->DeleteLocalRef(env, socketClass);
}

// Fungsi utama untuk mengirim data via Bluetooth Classic RFCOMM/SPP.
// Mengembalikan 0 sukses, -1 error.
// error_msg diisi dengan pesan error jika terjadi.
static int bluetooth_send(
    JNIEnv* env,
    jobject ctx,
    const char* mac,
    const unsigned char* data,
    size_t len,
    char** error_msg
) {
    if (error_msg) *error_msg = NULL;

    // 1. Context.getSystemService("bluetooth")
    jclass contextClass = (*env)->FindClass(env, "android/content/Context");
    if (contextClass == NULL) {
        if (check_exception(env, "FindClass(Context)", error_msg) != 0) return -1;
        if (error_msg) *error_msg = strdup("FindClass(Context) failed");
        return -1;
    }
    jmethodID getSystemService = (*env)->GetMethodID(env, contextClass, "getSystemService", "(Ljava/lang/String;)Ljava/lang/Object;");
    if (getSystemService == NULL) {
        if (check_exception(env, "GetMethodID(getSystemService)", error_msg) != 0) {
            (*env)->DeleteLocalRef(env, contextClass);
            return -1;
        }
        (*env)->DeleteLocalRef(env, contextClass);
        if (error_msg) *error_msg = strdup("getSystemService method not found");
        return -1;
    }
    jstring serviceName = (*env)->NewStringUTF(env, "bluetooth");
    if (serviceName == NULL) {
        if (check_exception(env, "NewStringUTF(serviceName)", error_msg) != 0) {
            (*env)->DeleteLocalRef(env, contextClass);
            return -1;
        }
        (*env)->DeleteLocalRef(env, contextClass);
        if (error_msg) *error_msg = strdup("failed to create service name string");
        return -1;
    }
    jobject bluetoothManager = (*env)->CallObjectMethod(env, ctx, getSystemService, serviceName);
    (*env)->DeleteLocalRef(env, serviceName);
    if (check_exception(env, "CallObjectMethod(getSystemService)", error_msg) != 0) {
        (*env)->DeleteLocalRef(env, contextClass);
        return -1;
    }
    (*env)->DeleteLocalRef(env, contextClass);
    if (bluetoothManager == NULL) {
        if (error_msg) *error_msg = strdup("failed to get BluetoothManager");
        return -1;
    }

    // 2. BluetoothManager.getAdapter() -> BluetoothAdapter
    jclass bluetoothManagerClass = (*env)->GetObjectClass(env, bluetoothManager);
    if (bluetoothManagerClass == NULL) {
        if (check_exception(env, "GetObjectClass(BluetoothManager)", error_msg) != 0) {
            (*env)->DeleteLocalRef(env, bluetoothManager);
            return -1;
        }
        (*env)->DeleteLocalRef(env, bluetoothManager);
        if (error_msg) *error_msg = strdup("BluetoothManager class not found");
        return -1;
    }
    jmethodID getAdapter = (*env)->GetMethodID(env, bluetoothManagerClass, "getAdapter", "()Landroid/bluetooth/BluetoothAdapter;");
    if (getAdapter == NULL) {
        if (check_exception(env, "GetMethodID(getAdapter)", error_msg) != 0) {
            (*env)->DeleteLocalRef(env, bluetoothManagerClass);
            (*env)->DeleteLocalRef(env, bluetoothManager);
            return -1;
        }
        (*env)->DeleteLocalRef(env, bluetoothManagerClass);
        (*env)->DeleteLocalRef(env, bluetoothManager);
        if (error_msg) *error_msg = strdup("getAdapter method not found");
        return -1;
    }
    jobject adapter = (*env)->CallObjectMethod(env, bluetoothManager, getAdapter);
    (*env)->DeleteLocalRef(env, bluetoothManagerClass);
    (*env)->DeleteLocalRef(env, bluetoothManager);
    if (check_exception(env, "CallObjectMethod(getAdapter)", error_msg) != 0) {
        return -1;
    }
    if (adapter == NULL) {
        if (error_msg) *error_msg = strdup("failed to get BluetoothAdapter");
        return -1;
    }

    // 3. BluetoothAdapter.getRemoteDevice(mac) -> BluetoothDevice
    jclass adapterClass = (*env)->GetObjectClass(env, adapter);
    if (adapterClass == NULL) {
        if (check_exception(env, "GetObjectClass(BluetoothAdapter)", error_msg) != 0) {
            (*env)->DeleteLocalRef(env, adapter);
            return -1;
        }
        (*env)->DeleteLocalRef(env, adapter);
        if (error_msg) *error_msg = strdup("BluetoothAdapter class not found");
        return -1;
    }
    jmethodID getRemoteDevice = (*env)->GetMethodID(env, adapterClass, "getRemoteDevice", "(Ljava/lang/String;)Landroid/bluetooth/BluetoothDevice;");
    if (getRemoteDevice == NULL) {
        if (check_exception(env, "GetMethodID(getRemoteDevice)", error_msg) != 0) {
            (*env)->DeleteLocalRef(env, adapterClass);
            (*env)->DeleteLocalRef(env, adapter);
            return -1;
        }
        (*env)->DeleteLocalRef(env, adapterClass);
        (*env)->DeleteLocalRef(env, adapter);
        if (error_msg) *error_msg = strdup("getRemoteDevice method not found");
        return -1;
    }
    jstring jMac = (*env)->NewStringUTF(env, mac);
    if (jMac == NULL) {
        if (check_exception(env, "NewStringUTF(mac)", error_msg) != 0) {
            (*env)->DeleteLocalRef(env, adapterClass);
            (*env)->DeleteLocalRef(env, adapter);
            return -1;
        }
        (*env)->DeleteLocalRef(env, adapterClass);
        (*env)->DeleteLocalRef(env, adapter);
        if (error_msg) *error_msg = strdup("failed to create MAC string");
        return -1;
    }
    jobject device = (*env)->CallObjectMethod(env, adapter, getRemoteDevice, jMac);
    (*env)->DeleteLocalRef(env, jMac);
    (*env)->DeleteLocalRef(env, adapterClass);
    (*env)->DeleteLocalRef(env, adapter);
    if (check_exception(env, "CallObjectMethod(getRemoteDevice)", error_msg) != 0) {
        return -1;
    }
    if (device == NULL) {
        if (error_msg) *error_msg = strdup("failed to get BluetoothDevice");
        return -1;
    }

    // 4. UUID.fromString(SPP_UUID)
    jclass uuidClass = (*env)->FindClass(env, "java/util/UUID");
    if (uuidClass == NULL) {
        if (check_exception(env, "FindClass(UUID)", error_msg) != 0) {
            (*env)->DeleteLocalRef(env, device);
            return -1;
        }
        (*env)->DeleteLocalRef(env, device);
        if (error_msg) *error_msg = strdup("UUID class not found");
        return -1;
    }
    jmethodID fromString = (*env)->GetStaticMethodID(env, uuidClass, "fromString", "(Ljava/lang/String;)Ljava/util/UUID;");
    if (fromString == NULL) {
        if (check_exception(env, "GetStaticMethodID(fromString)", error_msg) != 0) {
            (*env)->DeleteLocalRef(env, uuidClass);
            (*env)->DeleteLocalRef(env, device);
            return -1;
        }
        (*env)->DeleteLocalRef(env, uuidClass);
        (*env)->DeleteLocalRef(env, device);
        if (error_msg) *error_msg = strdup("UUID.fromString method not found");
        return -1;
    }
    jstring jUUIDStr = (*env)->NewStringUTF(env, SPP_UUID);
    if (jUUIDStr == NULL) {
        if (check_exception(env, "NewStringUTF(UUID)", error_msg) != 0) {
            (*env)->DeleteLocalRef(env, uuidClass);
            (*env)->DeleteLocalRef(env, device);
            return -1;
        }
        (*env)->DeleteLocalRef(env, uuidClass);
        (*env)->DeleteLocalRef(env, device);
        if (error_msg) *error_msg = strdup("failed to create UUID string");
        return -1;
    }
    jobject uuid = (*env)->CallStaticObjectMethod(env, uuidClass, fromString, jUUIDStr);
    (*env)->DeleteLocalRef(env, jUUIDStr);
    (*env)->DeleteLocalRef(env, uuidClass);
    if (check_exception(env, "CallStaticObjectMethod(fromString)", error_msg) != 0) {
        (*env)->DeleteLocalRef(env, device);
        return -1;
    }
    if (uuid == NULL) {
        (*env)->DeleteLocalRef(env, device);
        if (error_msg) *error_msg = strdup("failed to create UUID");
        return -1;
    }

    // 5. BluetoothDevice.createRfcommSocketToServiceRecord(UUID) -> BluetoothSocket
    jclass deviceClass = (*env)->GetObjectClass(env, device);
    if (deviceClass == NULL) {
        if (check_exception(env, "GetObjectClass(BluetoothDevice)", error_msg) != 0) {
            (*env)->DeleteLocalRef(env, uuid);
            (*env)->DeleteLocalRef(env, device);
            return -1;
        }
        (*env)->DeleteLocalRef(env, uuid);
        (*env)->DeleteLocalRef(env, device);
        if (error_msg) *error_msg = strdup("BluetoothDevice class not found");
        return -1;
    }
    jmethodID createSocket = (*env)->GetMethodID(env, deviceClass, "createRfcommSocketToServiceRecord", "(Ljava/util/UUID;)Landroid/bluetooth/BluetoothSocket;");
    if (createSocket == NULL) {
        // Fallback: createInsecureRfcommSocketToServiceRecord
        createSocket = (*env)->GetMethodID(env, deviceClass, "createInsecureRfcommSocketToServiceRecord", "(Ljava/util/UUID;)Landroid/bluetooth/BluetoothSocket;");
        if (createSocket == NULL) {
            if (check_exception(env, "GetMethodID(createRfcommSocketToServiceRecord)", error_msg) != 0) {
                (*env)->DeleteLocalRef(env, deviceClass);
                (*env)->DeleteLocalRef(env, uuid);
                (*env)->DeleteLocalRef(env, device);
                return -1;
            }
            (*env)->DeleteLocalRef(env, deviceClass);
            (*env)->DeleteLocalRef(env, uuid);
            (*env)->DeleteLocalRef(env, device);
            if (error_msg) *error_msg = strdup("createRfcommSocketToServiceRecord method not found");
            return -1;
        }
    }
    jobject socket = (*env)->CallObjectMethod(env, device, createSocket, uuid);
    (*env)->DeleteLocalRef(env, uuid);
    (*env)->DeleteLocalRef(env, deviceClass);
    (*env)->DeleteLocalRef(env, device);
    if (check_exception(env, "CallObjectMethod(createSocket)", error_msg) != 0) {
        return -1;
    }
    if (socket == NULL) {
        if (error_msg) *error_msg = strdup("failed to create BluetoothSocket");
        return -1;
    }

    // 6. BluetoothSocket.connect()
    jclass socketClass = (*env)->GetObjectClass(env, socket);
    if (socketClass == NULL) {
        if (check_exception(env, "GetObjectClass(BluetoothSocket)", error_msg) != 0) {
            close_socket_safe(env, socket);
            (*env)->DeleteLocalRef(env, socket);
            return -1;
        }
        close_socket_safe(env, socket);
        (*env)->DeleteLocalRef(env, socket);
        if (error_msg) *error_msg = strdup("BluetoothSocket class not found");
        return -1;
    }
    jmethodID connect = (*env)->GetMethodID(env, socketClass, "connect", "()V");
    if (connect == NULL) {
        if (check_exception(env, "GetMethodID(connect)", error_msg) != 0) {
            close_socket_safe(env, socket);
            (*env)->DeleteLocalRef(env, socketClass);
            (*env)->DeleteLocalRef(env, socket);
            return -1;
        }
        close_socket_safe(env, socket);
        (*env)->DeleteLocalRef(env, socketClass);
        (*env)->DeleteLocalRef(env, socket);
        if (error_msg) *error_msg = strdup("connect method not found");
        return -1;
    }
    (*env)->CallVoidMethod(env, socket, connect);
    if (check_exception(env, "CallVoidMethod(connect)", error_msg) != 0) {
        close_socket_safe(env, socket);
        (*env)->DeleteLocalRef(env, socketClass);
        (*env)->DeleteLocalRef(env, socket);
        return -1;
    }

    // 7. BluetoothSocket.getOutputStream() -> OutputStream
    jmethodID getOutputStream = (*env)->GetMethodID(env, socketClass, "getOutputStream", "()Ljava/io/OutputStream;");
    if (getOutputStream == NULL) {
        if (check_exception(env, "GetMethodID(getOutputStream)", error_msg) != 0) {
            close_socket_safe(env, socket);
            (*env)->DeleteLocalRef(env, socketClass);
            (*env)->DeleteLocalRef(env, socket);
            return -1;
        }
        close_socket_safe(env, socket);
        (*env)->DeleteLocalRef(env, socketClass);
        (*env)->DeleteLocalRef(env, socket);
        if (error_msg) *error_msg = strdup("getOutputStream method not found");
        return -1;
    }
    jobject outputStream = (*env)->CallObjectMethod(env, socket, getOutputStream);
    if (check_exception(env, "CallObjectMethod(getOutputStream)", error_msg) != 0) {
        close_socket_safe(env, socket);
        (*env)->DeleteLocalRef(env, socketClass);
        (*env)->DeleteLocalRef(env, socket);
        return -1;
    }
    if (outputStream == NULL) {
        close_socket_safe(env, socket);
        (*env)->DeleteLocalRef(env, socketClass);
        (*env)->DeleteLocalRef(env, socket);
        if (error_msg) *error_msg = strdup("failed to get OutputStream");
        return -1;
    }

    // 8. OutputStream.write(byte[])
    jclass outputStreamClass = (*env)->GetObjectClass(env, outputStream);
    if (outputStreamClass == NULL) {
        if (check_exception(env, "GetObjectClass(OutputStream)", error_msg) != 0) {
            close_socket_safe(env, socket);
            (*env)->DeleteLocalRef(env, outputStream);
            (*env)->DeleteLocalRef(env, socketClass);
            (*env)->DeleteLocalRef(env, socket);
            return -1;
        }
        close_socket_safe(env, socket);
        (*env)->DeleteLocalRef(env, outputStream);
        (*env)->DeleteLocalRef(env, socketClass);
        (*env)->DeleteLocalRef(env, socket);
        if (error_msg) *error_msg = strdup("OutputStream class not found");
        return -1;
    }
    jmethodID write = (*env)->GetMethodID(env, outputStreamClass, "write", "([B)V");
    if (write == NULL) {
        if (check_exception(env, "GetMethodID(write)", error_msg) != 0) {
            close_socket_safe(env, socket);
            (*env)->DeleteLocalRef(env, outputStreamClass);
            (*env)->DeleteLocalRef(env, outputStream);
            (*env)->DeleteLocalRef(env, socketClass);
            (*env)->DeleteLocalRef(env, socket);
            return -1;
        }
        close_socket_safe(env, socket);
        (*env)->DeleteLocalRef(env, outputStreamClass);
        (*env)->DeleteLocalRef(env, outputStream);
        (*env)->DeleteLocalRef(env, socketClass);
        (*env)->DeleteLocalRef(env, socket);
        if (error_msg) *error_msg = strdup("write method not found");
        return -1;
    }

    if (len > 0) {
        jbyteArray jPayload = (*env)->NewByteArray(env, (jsize)len);
        if (jPayload == NULL) {
            if (check_exception(env, "NewByteArray", error_msg) != 0) {
                close_socket_safe(env, socket);
                (*env)->DeleteLocalRef(env, outputStreamClass);
                (*env)->DeleteLocalRef(env, outputStream);
                (*env)->DeleteLocalRef(env, socketClass);
                (*env)->DeleteLocalRef(env, socket);
                return -1;
            }
            close_socket_safe(env, socket);
            (*env)->DeleteLocalRef(env, outputStreamClass);
            (*env)->DeleteLocalRef(env, outputStream);
            (*env)->DeleteLocalRef(env, socketClass);
            (*env)->DeleteLocalRef(env, socket);
            if (error_msg) *error_msg = strdup("failed to create byte array");
            return -1;
        }
        (*env)->SetByteArrayRegion(env, jPayload, 0, (jsize)len, (const jbyte*)data);
        if (check_exception(env, "SetByteArrayRegion", error_msg) != 0) {
            (*env)->DeleteLocalRef(env, jPayload);
            close_socket_safe(env, socket);
            (*env)->DeleteLocalRef(env, outputStreamClass);
            (*env)->DeleteLocalRef(env, outputStream);
            (*env)->DeleteLocalRef(env, socketClass);
            (*env)->DeleteLocalRef(env, socket);
            return -1;
        }
        (*env)->CallVoidMethod(env, outputStream, write, jPayload);
        if (check_exception(env, "CallVoidMethod(write)", error_msg) != 0) {
            (*env)->DeleteLocalRef(env, jPayload);
            close_socket_safe(env, socket);
            (*env)->DeleteLocalRef(env, outputStreamClass);
            (*env)->DeleteLocalRef(env, outputStream);
            (*env)->DeleteLocalRef(env, socketClass);
            (*env)->DeleteLocalRef(env, socket);
            return -1;
        }
        (*env)->DeleteLocalRef(env, jPayload);
    }

    // 9. Tutup socket dan bersihkan resources
    close_socket_safe(env, socket);
    (*env)->DeleteLocalRef(env, outputStreamClass);
    (*env)->DeleteLocalRef(env, outputStream);
    (*env)->DeleteLocalRef(env, socketClass);
    (*env)->DeleteLocalRef(env, socket);
    return 0;
}
*/
import "C"

func (t *BluetoothTransport) send(ctx context.Context, endpoint string, payload []byte, options map[string]string) error {
	if endpoint == "" {
		return fmt.Errorf("bluetooth endpoint required")
	}
	if !isMACFormat(endpoint) {
		return fmt.Errorf("invalid MAC address: %s", endpoint)
	}
	if len(payload) == 0 {
		return nil
	}

	var result error
	done := make(chan struct{})
	go func() {
		defer close(done)
		err := driver.RunNative(func(raw interface{}) error {
			ac, ok := raw.(*driver.AndroidContext)
			if !ok {
				return fmt.Errorf("failed to get Android context")
			}
			env := (*C.JNIEnv)(unsafe.Pointer(ac.Env))
			ctxObj := (C.jobject)(unsafe.Pointer(ac.Ctx))
			cMac := C.CString(endpoint)
			defer C.free(unsafe.Pointer(cMac))
			var errMsg *C.char
			ret := C.bluetooth_send(env, ctxObj, cMac, (*C.uchar)(unsafe.Pointer(&payload[0])), C.size_t(len(payload)), &errMsg)
			if ret != 0 {
				if errMsg != nil {
					defer C.free(unsafe.Pointer(errMsg))
					return fmt.Errorf("%s", C.GoString(errMsg))
				}
				return fmt.Errorf("bluetooth send failed")
			}
			return nil
		})
		result = err
	}()

	select {
	case <-done:
		return result
	case <-ctx.Done():
		return ctx.Err()
	}
}
