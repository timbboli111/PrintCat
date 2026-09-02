#ifndef BLUETOOTH_H
#define BLUETOOTH_H

#include <jni.h>
#include <stddef.h>

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
);

#endif