// Package hive provides the kernel plugin management infrastructure for Tierceron.
//
// This package implements a dynamic plugin system that allows loading, managing, and
// orchestrating Go plugins at runtime. Key features include:
//
//   - Dynamic plugin loading and reloading
//   - Plugin lifecycle management (initialization, startup, shutdown)
//   - Certificate monitoring and automatic renewal
//   - Kernel command routing and execution
//   - Chat message handling and inter-plugin communication
//   - Panic recovery and error handling
//
// # Plugin Handler
//
// The PluginHandler manages multiple plugin kernels, each running in its own goroutine.
// It handles:
//   - Plugin registration and initialization
//   - Certificate lifecycle monitoring
//   - Dynamic plugin updates without downtime
//   - Safe channel communication between plugins
//
// # Safety Features
//
// All channel operations use generic safeChannelSend[T] to prevent panics from:
//   - Sending to closed channels
//   - Nil channel dereferences
//   - Closed channel detection
//
// # Usage
//
// The hive package is designed to work with plugins built using Go's plugin package.
// Plugins must implement the expected interface methods for initialization, startup,
// and shutdown operations.
package hive
