//go:build !android

package bridge

// The bridge package is platform-agnostic: State and Submit work the same
// way on every operating system. Only the JNI entry points live in
// cmd/printcat/bridge_android.go behind the android build tag.
//
// This file exists as a placeholder so that non-Android builds have an
// explicit, documented counterpart to any future platform-specific code
// that is not Android-only. It carries no logic today.
