#include "bluetooth.h"
#include <string.h>
#include <stdlib.h>

static void set_error(char** error_msg, const char* msg) {
    if (error_msg) {
        *error_msg = strdup(msg);
    }
}

static void clear_exception(JNIEnv* env) {
    if ((*env)->ExceptionCheck(env)) {
        (*env)->ExceptionClear(env);
    }
}

static jobject get_bluetooth_adapter(JNIEnv* env, jobject context) {
    jclass bluetoothManagerClass = (*env)->FindClass(env, "android/bluetooth/BluetoothManager");
    if (bluetoothManagerClass == NULL) {
        clear_exception(env);
        return NULL;
    }

    jclass contextClass = (*env)->FindClass(env, "android/content/Context");
    jmethodID getSystemService = (*env)->GetMethodID(env, contextClass, "getSystemService", "(Ljava/lang/String;)Ljava/lang/Object;");
    if (getSystemService == NULL) {
        (*env)->DeleteLocalRef(env, bluetoothManagerClass);
        clear_exception(env);
        return NULL;
    }

    jstring serviceName = (*env)->NewStringUTF(env, "bluetooth");
    jobject manager = (*env)->CallObjectMethod(env, context, getSystemService, serviceName);
    (*env)->DeleteLocalRef(env, serviceName);
    (*env)->DeleteLocalRef(env, contextClass);
    if (manager == NULL) {
        (*env)->DeleteLocalRef(env, bluetoothManagerClass);
        clear_exception(env);
        return NULL;
    }

    jmethodID getAdapter = (*env)->GetMethodID(env, bluetoothManagerClass, "getAdapter", "()Landroid/bluetooth/BluetoothAdapter;");
    jobject adapter = (*env)->CallObjectMethod(env, manager, getAdapter);
    (*env)->DeleteLocalRef(env, manager);
    (*env)->DeleteLocalRef(env, bluetoothManagerClass);
    if (adapter == NULL) {
        clear_exception(env);
        return NULL;
    }
    return adapter;
}

static jobject get_remote_device(JNIEnv* env, jobject adapter, const char* mac) {
    jclass adapterClass = (*env)->GetObjectClass(env, adapter);
    jmethodID getRemoteDevice = (*env)->GetMethodID(env, adapterClass, "getRemoteDevice", "(Ljava/lang/String;)Landroid/bluetooth/BluetoothDevice;");
    if (getRemoteDevice == NULL) {
        (*env)->DeleteLocalRef(env, adapterClass);
        clear_exception(env);
        return NULL;
    }
    jstring jmac = (*env)->NewStringUTF(env, mac);
    jobject device = (*env)->CallObjectMethod(env, adapter, getRemoteDevice, jmac);
    (*env)->DeleteLocalRef(env, jmac);
    (*env)->DeleteLocalRef(env, adapterClass);
    if (device == NULL) {
        clear_exception(env);
        return NULL;
    }
    return device;
}

static jobject create_rfcomm_socket(JNIEnv* env, jobject device) {
    jclass deviceClass = (*env)->GetObjectClass(env, device);

    // SPP UUID: 00001101-0000-1000-8000-00805F9B34FB
    jstring uuidStr = (*env)->NewStringUTF(env, "00001101-0000-1000-8000-00805F9B34FB");
    jclass uuidClass = (*env)->FindClass(env, "java/util/UUID");
    if (uuidClass == NULL) {
        (*env)->DeleteLocalRef(env, uuidStr);
        (*env)->DeleteLocalRef(env, deviceClass);
        clear_exception(env);
        return NULL;
    }
    jmethodID fromString = (*env)->GetStaticMethodID(env, uuidClass, "fromString", "(Ljava/lang/String;)Ljava/util/UUID;");
    jobject uuid = (*env)->CallStaticObjectMethod(env, uuidClass, fromString, uuidStr);
    (*env)->DeleteLocalRef(env, uuidStr);
    (*env)->DeleteLocalRef(env, uuidClass);
    if (uuid == NULL) {
        (*env)->DeleteLocalRef(env, deviceClass);
        clear_exception(env);
        return NULL;
    }

    jmethodID createSocket = (*env)->GetMethodID(env, deviceClass, "createRfcommSocketToServiceRecord", "(Ljava/util/UUID;)Landroid/bluetooth/BluetoothSocket;");
    if (createSocket == NULL) {
        (*env)->DeleteLocalRef(env, uuid);
        (*env)->DeleteLocalRef(env, deviceClass);
        clear_exception(env);
        return NULL;
    }
    jobject socket = (*env)->CallObjectMethod(env, device, createSocket, uuid);
    (*env)->DeleteLocalRef(env, uuid);
    (*env)->DeleteLocalRef(env, deviceClass);
    if (socket == NULL) {
        clear_exception(env);
        return NULL;
    }
    return socket;
}

static jobject create_fallback_socket(JNIEnv* env, jobject device, int channel) {
    jclass deviceClass = (*env)->GetObjectClass(env, device);
    jmethodID createSocket = (*env)->GetMethodID(env, deviceClass, "createInsecureRfcommSocket", "(I)Landroid/bluetooth/BluetoothSocket;");
    if (createSocket == NULL) {
        clear_exception(env);
        (*env)->DeleteLocalRef(env, deviceClass);
        return NULL;
    }
    jobject socket = (*env)->CallObjectMethod(env, device, createSocket, (jint)channel);
    (*env)->DeleteLocalRef(env, deviceClass);
    if (socket == NULL) {
        clear_exception(env);
        return NULL;
    }
    return socket;
}

int bluetooth_send(
    uintptr_t java_vm,
    uintptr_t jni_env,
    uintptr_t ctx,
    const char* mac,
    const unsigned char* data,
    size_t len,
    int channel,
    int timeout_sec,
    char** error_msg
) {
    JNIEnv* env = (JNIEnv*)jni_env;
    jobject context = (jobject)ctx;

    (void)timeout_sec;

    if (error_msg) {
        *error_msg = NULL;
    }

    if (mac == NULL || data == NULL || len == 0) {
        set_error(error_msg, "invalid parameters");
        return -1;
    }

    // Dapatkan BluetoothAdapter
    jobject adapter = get_bluetooth_adapter(env, context);
    if (adapter == NULL) {
        set_error(error_msg, "failed to get BluetoothAdapter");
        return -2;
    }

    // Dapatkan BluetoothDevice dari MAC
    jobject device = get_remote_device(env, adapter, mac);
    (*env)->DeleteLocalRef(env, adapter);
    if (device == NULL) {
        set_error(error_msg, "failed to get BluetoothDevice");
        return -3;
    }

    // Buat RFCOMM socket dengan SPP UUID (prioritas utama)
    jobject socket = create_rfcomm_socket(env, device);
    if (socket == NULL) {
        // Fallback: gunakan channel manual
        socket = create_fallback_socket(env, device, channel);
        if (socket == NULL) {
            (*env)->DeleteLocalRef(env, device);
            set_error(error_msg, "failed to create RFCOMM socket");
            return -4;
        }
    }
    (*env)->DeleteLocalRef(env, device);

    // Connect
    jclass socketClass = (*env)->GetObjectClass(env, socket);
    jmethodID connect = (*env)->GetMethodID(env, socketClass, "connect", "()V");
    if (connect == NULL) {
        (*env)->DeleteLocalRef(env, socketClass);
        (*env)->DeleteLocalRef(env, socket);
        set_error(error_msg, "connect method not found");
        return -5;
    }
    (*env)->CallVoidMethod(env, socket, connect);

    if ((*env)->ExceptionCheck(env)) {
        jthrowable exc = (*env)->ExceptionOccurred(env);
        (*env)->ExceptionClear(env);
        jclass excClass = (*env)->GetObjectClass(env, exc);
        jmethodID getMessage = (*env)->GetMethodID(env, excClass, "getMessage", "()Ljava/lang/String;");
        if (getMessage != NULL) {
            jstring msg = (*env)->CallObjectMethod(env, exc, getMessage);
            const char* msgStr = (*env)->GetStringUTFChars(env, msg, NULL);
            set_error(error_msg, msgStr);
            (*env)->ReleaseStringUTFChars(env, msg, msgStr);
            (*env)->DeleteLocalRef(env, msg);
        }
        (*env)->DeleteLocalRef(env, excClass);
        (*env)->DeleteLocalRef(env, exc);

        // Close socket
        jmethodID close = (*env)->GetMethodID(env, socketClass, "close", "()V");
        if (close != NULL) {
            (*env)->CallVoidMethod(env, socket, close);
        }
        (*env)->DeleteLocalRef(env, socketClass);
        (*env)->DeleteLocalRef(env, socket);
        return -6;
    }

    // Get OutputStream
    jmethodID getOutputStream = (*env)->GetMethodID(env, socketClass, "getOutputStream", "()Ljava/io/OutputStream;");
    if (getOutputStream == NULL) {
        (*env)->DeleteLocalRef(env, socketClass);
        (*env)->DeleteLocalRef(env, socket);
        set_error(error_msg, "getOutputStream method not found");
        return -7;
    }
    jobject outputStream = (*env)->CallObjectMethod(env, socket, getOutputStream);
    if (outputStream == NULL) {
        (*env)->DeleteLocalRef(env, socketClass);
        (*env)->DeleteLocalRef(env, socket);
        set_error(error_msg, "failed to get OutputStream");
        return -8;
    }

    // Write data
    jclass outputStreamClass = (*env)->GetObjectClass(env, outputStream);
    jmethodID write = (*env)->GetMethodID(env, outputStreamClass, "write", "([B)V");
    if (write == NULL) {
        (*env)->DeleteLocalRef(env, outputStreamClass);
        (*env)->DeleteLocalRef(env, outputStream);
        (*env)->DeleteLocalRef(env, socketClass);
        (*env)->DeleteLocalRef(env, socket);
        set_error(error_msg, "write method not found");
        return -9;
    }

    jbyteArray arr = (*env)->NewByteArray(env, (jsize)len);
    if (arr == NULL) {
        (*env)->DeleteLocalRef(env, outputStreamClass);
        (*env)->DeleteLocalRef(env, outputStream);
        (*env)->DeleteLocalRef(env, socketClass);
        (*env)->DeleteLocalRef(env, socket);
        set_error(error_msg, "failed to allocate byte array");
        return -10;
    }
    (*env)->SetByteArrayRegion(env, arr, 0, (jsize)len, (const jbyte*)data);
    (*env)->CallVoidMethod(env, outputStream, write, arr);

    if ((*env)->ExceptionCheck(env)) {
        jthrowable exc = (*env)->ExceptionOccurred(env);
        (*env)->ExceptionClear(env);
        jclass excClass = (*env)->GetObjectClass(env, exc);
        jmethodID getMessage = (*env)->GetMethodID(env, excClass, "getMessage", "()Ljava/lang/String;");
        if (getMessage != NULL) {
            jstring msg = (*env)->CallObjectMethod(env, exc, getMessage);
            const char* msgStr = (*env)->GetStringUTFChars(env, msg, NULL);
            set_error(error_msg, msgStr);
            (*env)->ReleaseStringUTFChars(env, msg, msgStr);
            (*env)->DeleteLocalRef(env, msg);
        }
        (*env)->DeleteLocalRef(env, excClass);
        (*env)->DeleteLocalRef(env, exc);
        (*env)->DeleteLocalRef(env, arr);
        (*env)->DeleteLocalRef(env, outputStreamClass);
        (*env)->DeleteLocalRef(env, outputStream);
        // Close socket
        jmethodID close = (*env)->GetMethodID(env, socketClass, "close", "()V");
        if (close != NULL) {
            (*env)->CallVoidMethod(env, socket, close);
        }
        (*env)->DeleteLocalRef(env, socketClass);
        (*env)->DeleteLocalRef(env, socket);
        return -11;
    }

    // Cleanup
    (*env)->DeleteLocalRef(env, arr);
    (*env)->DeleteLocalRef(env, outputStreamClass);
    (*env)->DeleteLocalRef(env, outputStream);

    // Close socket
    jmethodID close = (*env)->GetMethodID(env, socketClass, "close", "()V");
    if (close != NULL) {
        (*env)->CallVoidMethod(env, socket, close);
    }
    (*env)->DeleteLocalRef(env, socketClass);
    (*env)->DeleteLocalRef(env, socket);

    return 0;
}