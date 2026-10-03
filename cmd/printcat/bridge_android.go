//go:build android

package main

/*
#include <jni.h>
*/
import "C"

import (
	"log"
	"runtime"

	"github.com/timboli111/PrintCat/internal/bridge"
)

// Step 0 probe: proves that a Java -> Go JNI call reaches the Go runtime
// while PrintService is being invoked by the Android Print Framework.
//
// The cgo preamble contains only #include directives, which is required
// when //export is used. Diagnostic content is written via log.Printf,
// which gomobile routes to Android logcat.
//
//export Java_com_printcat_app_PrintCatPrintService_nativeProbe
func Java_com_printcat_app_PrintCatPrintService_nativeProbe(env *C.JNIEnv, clazz C.jclass, marker C.jint) C.jint {
	log.Printf("[PrintCatProbe][go] nativeProbe reached: marker=%d runtime=%s/%s goroutines=%d",
		int32(marker), runtime.GOOS, runtime.GOARCH, runtime.NumGoroutine())
	return C.jint(42)
}

// Step 1A bridge entry point. In Step 1A the function proves that the
// symbol is exported to libmain.so and that the Go-side bridge state is
// reachable from JNI. Full argument extraction (pages, dimensions, DPI,
// printer id) will be implemented in Step 1B alongside the Java
// PrintJobDispatcher.
//
// Return codes:
//
//	 0  symbol reached, state present, active printer present
//	-1  bridge state not initialized (SetGlobal not called)
//	-2  no active printer configured
//
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
	log.Printf("[PrintCatBridge][go] nativeSubmitJob: active printer=%s protocol=%s transport=%s",
		active.Name, active.Connection.Protocol, active.Connection.Transport)

	// TODO(Step 1B): extract pagesPng, widthsUm, heightsUm, printerIDLocal
	// from JNI and call state.Submit(context.Background(), req).
	return C.jint(0)
}
