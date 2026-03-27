// internal/router/registry_imports.go
// Defines the internal registry state snapshot and port validation logic
// used during the extension compilation phase.

package router

import "sync/atomic"

type routerRegistrySnapshot struct {
	providers    map[PortName]Provider
	restrictions map[PortName][]string
}

var registry atomic.Pointer[routerRegistrySnapshot]

// RouterValidatePortName reports whether the port is declared in the router whitelist.
func RouterValidatePortName(port PortName) bool {
	switch port {
	case PortPrimary, PortSecondary, PortTertiary, PortOptional, PortCLIStyle, PortCLIChrome, PortCLIInteraction:
		return true
	default:
		return false
	}
}
