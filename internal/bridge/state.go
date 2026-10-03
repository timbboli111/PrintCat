package bridge

import "sync"

var (
	globalMu    sync.RWMutex
	globalState *State
)

// SetGlobal stores the process-wide bridge state. It is intended to be
// called once during application startup, before any external print job
// arrives through the JNI entry points.
func SetGlobal(s *State) {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalState = s
}

// GetGlobal returns the process-wide bridge state, or nil if SetGlobal has
// not been called yet. The JNI entry points use this to locate the engine.
func GetGlobal() *State {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return globalState
}
