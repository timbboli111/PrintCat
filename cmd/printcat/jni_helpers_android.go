//go:build android

// Package main: JNI C helpers for the Android print bridge.
//
// This file exists only to hold the definitions of the C helper functions
// used by bridge_android.go. bridge_android.go uses //export, whose cgo
// preamble may contain only declarations (not definitions), so the
// definitions live here in a separate cgo file that does not use //export.
//
// The helpers abstract away the JNIEnv function-pointer style, exposing
// plain C functions that bridge_android.go can call without touching
// JNIEnv directly.

package main

/*
#include <jni.h>
#include <stdlib.h>
#include <string.h>

// printcat_jni_object_array_length returns the length of a Java object
// array as int, or -1 on error.
int printcat_jni_object_array_length(JNIEnv* env, void* arr);

// printcat_jni_object_array_element returns the i-th element of a Java
// object array as a raw pointer. The returned reference is a JNI local
// reference, which is released by the JVM when the calling native method
// returns. Returns NULL on error.
void* printcat_jni_object_array_element(JNIEnv* env, void* arr, int i);

// printcat_jni_byte_array_to_c copies a Java byte[] into a freshly
// allocated C buffer of the same length. On success it writes the length
// to *out_len and returns a pointer the caller must release with free().
// Returns NULL on error.
unsigned char* printcat_jni_byte_array_to_c(JNIEnv* env, void* arr, size_t* out_len);

// printcat_jni_int_array_to_c copies a Java int[] into a freshly allocated
// C buffer. On success it writes the element count to *out_count and
// returns a pointer the caller must release with free(). Returns NULL on
// error or for an empty array.
int* printcat_jni_int_array_to_c(JNIEnv* env, void* arr, size_t* out_count);

// printcat_jni_string_to_c copies a Java String into a freshly allocated
// UTF-8 C string. The caller must release the result with free(). Returns
// NULL if s is NULL or on error.
char* printcat_jni_string_to_c(JNIEnv* env, void* s);

// ---- Definitions ----

int printcat_jni_object_array_length(JNIEnv* env, void* arr) {
	if (env == NULL || arr == NULL) {
		return -1;
	}
	return (int)(*env)->GetArrayLength(env, (jarray)arr);
}

void* printcat_jni_object_array_element(JNIEnv* env, void* arr, int i) {
	if (env == NULL || arr == NULL || i < 0) {
		return NULL;
	}
	return (void*)(*env)->GetObjectArrayElement(env, (jobjectArray)arr, (jsize)i);
}

unsigned char* printcat_jni_byte_array_to_c(JNIEnv* env, void* arr, size_t* out_len) {
	if (env == NULL || arr == NULL) {
		return NULL;
	}
	jsize len = (*env)->GetArrayLength(env, (jarray)arr);
	if (len <= 0) {
		return NULL;
	}
	unsigned char* buf = (unsigned char*)malloc((size_t)len);
	if (buf == NULL) {
		return NULL;
	}
	(*env)->GetByteArrayRegion(env, (jbyteArray)arr, 0, len, (jbyte*)buf);
	if ((*env)->ExceptionCheck(env)) {
		(*env)->ExceptionClear(env);
		free(buf);
		return NULL;
	}
	if (out_len != NULL) {
		*out_len = (size_t)len;
	}
	return buf;
}

int* printcat_jni_int_array_to_c(JNIEnv* env, void* arr, size_t* out_count) {
	if (env == NULL || arr == NULL) {
		return NULL;
	}
	jsize len = (*env)->GetArrayLength(env, (jarray)arr);
	if (len <= 0) {
		if (out_count != NULL) {
			*out_count = 0;
		}
		return NULL;
	}
	int* buf = (int*)malloc((size_t)len * sizeof(int));
	if (buf == NULL) {
		return NULL;
	}
	(*env)->GetIntArrayRegion(env, (jintArray)arr, 0, len, (jint*)buf);
	if ((*env)->ExceptionCheck(env)) {
		(*env)->ExceptionClear(env);
		free(buf);
		return NULL;
	}
	if (out_count != NULL) {
		*out_count = (size_t)len;
	}
	return buf;
}

char* printcat_jni_string_to_c(JNIEnv* env, void* s) {
	if (env == NULL || s == NULL) {
		return NULL;
	}
	const char* cstr = (*env)->GetStringUTFChars(env, (jstring)s, NULL);
	if (cstr == NULL) {
		return NULL;
	}
	size_t len = strlen(cstr);
	char* buf = (char*)malloc(len + 1);
	if (buf != NULL) {
		memcpy(buf, cstr, len + 1);
	}
	(*env)->ReleaseStringUTFChars(env, (jstring)s, cstr);
	return buf;
}
*/
import "C"

// No Go code is required in this file. The C definitions above are compiled
// by cgo and linked into the same package as bridge_android.go.
