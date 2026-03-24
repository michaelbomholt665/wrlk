// Package ext wires the concrete provider extensions into the router.
//
// Package Concerns:
//   - Only package permitted to import both internal/router and internal/adapters.
//   - All application boot entry points must call ext.RouterBootExtensions, which
//     keeps extension activation explicit and compile-time.
//   - Router capability extensions (optional, infrastructure-level) live under
//     internal/router/ext/extensions/<name>/ and are referenced only from
//     optional_extensions.go. Application adapters and ports are NOT placed here.
package ext
