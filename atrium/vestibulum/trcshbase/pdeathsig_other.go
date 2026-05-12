//go:build !linux

package trcshbase

// SetParentDeathSignal is a no-op on non-Linux platforms.
func SetParentDeathSignal() {}
