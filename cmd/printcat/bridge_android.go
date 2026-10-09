//go:build android

package main

/*
#include <jni.h>
#include <stdlib.h>

int printcat_jni_object_array_length(JNIEnv* env, void* arr);
void* printcat_jni_object_array_element(JNIEnv* env, void* arr, int i);
unsigned char* printcat_jni_byte_array_to_c(JNIEnv* env, void* arr, size_t* out_len);
int* printcat_jni_int_array_to_c(JNIEnv* env, void* arr, size_t* out_count);
char* printcat_jni_string_to_c(JNIEnv* env, void* s);
*/
import "C"

import (
	"context"
	"log"
	"runtime"
	"unsafe"

	"github.com/timboli111/PrintCat/internal/bridge"
)

//export Java_com_printcat_app_PrintCatPrintService_nativeProbe
func Java_com_printcat_app_PrintCatPrintService_nativeProbe(env *C.JNIEnv, clazz C.jclass, marker C.jint) C.jint {
	log.Printf("[PrintCatProbe][go] nativeProbe reached: marker=%d runtime=%s/%s goroutines=%d",
		int32(marker), runtime.GOOS, runtime.GOARCH, runtime.NumGoroutine())
	return C.jint(42)
}

//export Java_com_printcat_app_PrintCatPrintService_nativeEnsureBridgeInitialized
func Java_com_printcat_app_PrintCatPrintService_nativeEnsureBridgeInitialized(
	env *C.JNIEnv,
	clazz C.jclass,
	configPath C.jstring,
) C.jint {
	path := ""
	if unsafe.Pointer(configPath) != nil {
		cPath := C.printcat_jni_string_to_c(env, unsafe.Pointer(configPath))
		if cPath != nil {
			path = C.GoString(cPath)
			C.free(unsafe.Pointer(cPath))
		}
	}
	log.Printf("[PrintCatBridge][go] nativeEnsureBridgeInitialized: configPath=%q", path)

	svc := bridge.Bootstrap(path)
	if svc == nil {
		log.Printf("[PrintCatBridge][go] nativeEnsureBridgeInitialized: bootstrap failed")
		return C.jint(-1)
	}
	state := bridge.GetGlobal()
	if state == nil {
		log.Printf("[PrintCatBridge][go] nativeEnsureBridgeInitialized: state is nil after bootstrap")
		return C.jint(-1)
	}
	active := state.ActivePrinter()
	if active == nil {
		log.Printf("[PrintCatBridge][go] nativeEnsureBridgeInitialized: no active printer configured")
		return C.jint(-2)
	}
	log.Printf("[PrintCatBridge][go] nativeEnsureBridgeInitialized: active printer=%q protocol=%s transport=%s paper=%dx%dmm printable=%dum",
		active.Name, active.Connection.Protocol, active.Connection.Transport,
		state.ActivePaperWidthMm(), state.ActivePaperHeightMm(),
		int64(active.Profile.MediaWidth))
	return C.jint(0)
}

//export Java_com_printcat_app_PrintCatPrintService_nativeGetActivePaperWidthMils
func Java_com_printcat_app_PrintCatPrintService_nativeGetActivePaperWidthMils(
	env *C.JNIEnv,
	clazz C.jclass,
) C.jint {
	state := bridge.GetGlobal()
	if state == nil {
		return C.jint(-1)
	}
	mm := state.ActivePaperWidthMm()
	if mm <= 0 {
		return C.jint(-1)
	}
	mils := mm * 5000 / 127
	return C.jint(mils)
}

//export Java_com_printcat_app_PrintCatPrintService_nativeGetActivePaperHeightMils
func Java_com_printcat_app_PrintCatPrintService_nativeGetActivePaperHeightMils(
	env *C.JNIEnv,
	clazz C.jclass,
) C.jint {
	state := bridge.GetGlobal()
	if state == nil {
		return C.jint(-1)
	}
	mm := state.ActivePaperHeightMm()
	if mm <= 0 {
		return C.jint(-1)
	}
	mils := mm * 5000 / 127
	return C.jint(mils)
}

// nativeGetActiveMediaWidthMils returns the active printer's printable
// width in mils (1 mm = 5000/127 mils), or -1 if the printer profile does
// not declare one. PrintCatPrintService uses this value to set MinMargins
// on the advertised PrinterInfo so that Android Print Framework keeps
// content within the leftmost printable-width portion of the physical
// page. It does NOT change the MediaSize (which remains the physical paper
// width).
//
//export Java_com_printcat_app_PrintCatPrintService_nativeGetActiveMediaWidthMils
func Java_com_printcat_app_PrintCatPrintService_nativeGetActiveMediaWidthMils(
	env *C.JNIEnv,
	clazz C.jclass,
) C.jint {
	state := bridge.GetGlobal()
	if state == nil {
		return C.jint(-1)
	}
	active := state.ActivePrinter()
	if active == nil {
		return C.jint(-1)
	}
	um := int64(active.Profile.MediaWidth)
	if um <= 0 {
		return C.jint(-1)
	}
	mils := um * 5 / 127
	return C.jint(mils)
}

//export Java_com_printcat_app_PrintCatPrintService_nativeSubmitJob
func Java_com_printcat_app_PrintCatPrintService_nativeSubmitJob(
	env *C.JNIEnv,
	clazz C.jclass,
	pagesPng C.jobjectArray,
	widthsUm C.jintArray,
	heightsUm C.jintArray,
	dpi C.jint,
	printerIDLocal C.jstring,
) C.jint {
	log.Printf("[PrintCatBridge][go] nativeSubmitJob reached: dpi=%d", int32(dpi))

	state := bridge.GetGlobal()
	if state == nil {
		log.Printf("[PrintCatBridge][go] nativeSubmitJob: bridge state not initialized")
		return C.jint(-1)
	}
	active := state.ActivePrinter()
	if active == nil {
		log.Printf("[PrintCatBridge][go] nativeSubmitJob: no active printer configured")
		return C.jint(-2)
	}
	log.Printf("[PrintCatBridge][go] nativeSubmitJob: active printer=%s protocol=%s transport=%s endpoint=%s",
		active.Name, active.Connection.Protocol, active.Connection.Transport, active.Connection.Endpoint)

	pagesPngPtr := unsafe.Pointer(pagesPng)
	widthsUmPtr := unsafe.Pointer(widthsUm)
	heightsUmPtr := unsafe.Pointer(heightsUm)
	printerIDPtr := unsafe.Pointer(printerIDLocal)

	if pagesPngPtr == nil {
		log.Printf("[PrintCatBridge][go] nativeSubmitJob: pagesPng is null")
		return C.jint(-3)
	}
	if widthsUmPtr == nil || heightsUmPtr == nil {
		log.Printf("[PrintCatBridge][go] nativeSubmitJob: widthsUm or heightsUm is null")
		return C.jint(-4)
	}

	nPages := int(C.printcat_jni_object_array_length(env, pagesPngPtr))
	nWidths := int(C.printcat_jni_object_array_length(env, widthsUmPtr))
	nHeights := int(C.printcat_jni_object_array_length(env, heightsUmPtr))
	if nPages <= 0 {
		log.Printf("[PrintCatBridge][go] nativeSubmitJob: pagesPng is empty (nPages=%d)", nPages)
		return C.jint(-5)
	}
	if nWidths != nPages || nHeights != nPages {
		log.Printf("[PrintCatBridge][go] nativeSubmitJob: array length mismatch pages=%d widths=%d heights=%d",
			nPages, nWidths, nHeights)
		return C.jint(-6)
	}

	if int32(dpi) <= 0 {
		log.Printf("[PrintCatBridge][go] nativeSubmitJob: invalid dpi=%d", int32(dpi))
		return C.jint(-7)
	}

	var widthsCount, heightsCount C.size_t
	widthsPtr := C.printcat_jni_int_array_to_c(env, widthsUmPtr, &widthsCount)
	if widthsPtr == nil {
		log.Printf("[PrintCatBridge][go] nativeSubmitJob: failed to copy widthsUm")
		return C.jint(-8)
	}
	defer C.free(unsafe.Pointer(widthsPtr))

	heightsPtr := C.printcat_jni_int_array_to_c(env, heightsUmPtr, &heightsCount)
	if heightsPtr == nil {
		log.Printf("[PrintCatBridge][go] nativeSubmitJob: failed to copy heightsUm")
		return C.jint(-9)
	}
	defer C.free(unsafe.Pointer(heightsPtr))

	if int(widthsCount) != nPages || int(heightsCount) != nPages {
		log.Printf("[PrintCatBridge][go] nativeSubmitJob: post-copy length mismatch widths=%d heights=%d pages=%d",
			int(widthsCount), int(heightsCount), nPages)
		return C.jint(-6)
	}

	widths := unsafe.Slice((*C.int)(unsafe.Pointer(widthsPtr)), int(widthsCount))
	heights := unsafe.Slice((*C.int)(unsafe.Pointer(heightsPtr)), int(heightsCount))

	pages := make([][]byte, nPages)
	widths64 := make([]int64, nPages)
	heights64 := make([]int64, nPages)
	for i := 0; i < nPages; i++ {
		pageObj := C.printcat_jni_object_array_element(env, pagesPngPtr, C.int(i))
		if pageObj == nil {
			log.Printf("[PrintCatBridge][go] nativeSubmitJob: page %d is null", i)
			return C.jint(-10)
		}
		var bufLen C.size_t
		bufPtr := C.printcat_jni_byte_array_to_c(env, pageObj, &bufLen)
		if bufPtr == nil || bufLen == 0 {
			log.Printf("[PrintCatBridge][go] nativeSubmitJob: page %d byte[] is empty", i)
			return C.jint(-11)
		}
		pages[i] = C.GoBytes(unsafe.Pointer(bufPtr), C.int(bufLen))
		C.free(unsafe.Pointer(bufPtr))
		widths64[i] = int64(widths[i])
		heights64[i] = int64(heights[i])
	}

	printerID := ""
	if printerIDPtr != nil {
		cid := C.printcat_jni_string_to_c(env, printerIDPtr)
		if cid != nil {
			printerID = C.GoString(cid)
			C.free(unsafe.Pointer(cid))
		}
	}

	req := bridge.SubmitRequest{
		Pages:         pages,
		PageWidthsUm:  widths64,
		PageHeightsUm: heights64,
		DPI:           int(int32(dpi)),
		PrinterID:     printerID,
	}

	log.Printf("[PrintCatBridge][go] nativeSubmitJob: submitting %d page(s) via %s / %s",
		nPages, active.Connection.Protocol, active.Connection.Transport)

	if err := state.Submit(context.Background(), req); err != nil {
		log.Printf("[PrintCatBridge][go] nativeSubmitJob: Submit error: %v", err)
		return C.jint(-12)
	}

	log.Printf("[PrintCatBridge][go] nativeSubmitJob: submitted %d page(s) for printer %q",
		nPages, active.Name)
	return C.jint(0)
}