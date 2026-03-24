// Package ext Wires the concrete adapter providers into the router.
//
// Package Concerns:
// - Only package permitted to import both internal/router and internal/adapters.
// - All application boot entry points must call ext.RouterBootExtensions, which keeps extension activation explicit and compile-time.
package ext
