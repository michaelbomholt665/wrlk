// Package telemetry is a router capability extension that registers an optional
// telemetry provider before application extensions boot.
//
// Package Concerns:
//   - Implements router.Extension only; no imports from internal/adapters or internal/ports.
//   - Registering under router.PortOptional makes this capability available to any
//     application extension that declares it in Consumes().
//   - Failure to boot results in an OptionalExtensionFailed warning; boot continues.
package telemetry
