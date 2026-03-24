package ext

import (
	"log"

	"policycheck/internal/router"
)

// telemetryExample is an example optional extension that registers a telemetry provider
// before application extensions boot.
type telemetryExample struct{}

// Required reports that the telemetry extension is optional.
// Returning false means failure to boot this extension will result in an
// OptionalExtensionFailed warning, but boot will continue.
func (e *telemetryExample) Required() bool {
	return false
}

// Consumes reports that the telemetry extension has no dependencies and can boot first.
func (e *telemetryExample) Consumes() []router.PortName {
	return nil
}

// Provides reports that the telemetry extension registers the telemetry port.
func (e *telemetryExample) Provides() []router.PortName {
	return []router.PortName{router.PortTelemetry}
}

// RouterProvideRegistration registers a dummy telemetry provider.
//
// RouterProvideOptionalCapability: This function demonstrates how to register an optional
// capability into the router. The provider simply prints "telemetry initialized" as an example.
func (e *telemetryExample) RouterProvideRegistration(reg *router.Registry) error {
	provider := struct{ Name string }{Name: "telemetry-provider"}
	if err := reg.RouterRegisterProvider(router.PortTelemetry, provider); err != nil {
		return err // router will wrap this in OptionalExtensionFailed
	}
	log.Println("telemetry initialized")
	return nil
}
