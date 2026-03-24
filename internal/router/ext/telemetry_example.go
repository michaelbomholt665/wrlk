package ext

import (
	"log"

	"github.com/michaelbomholt665/wrlk/internal/router"
)

// optionalExample is an example optional extension that registers a telemetry provider
// before application extensions boot.
type optionalExample struct{}

// Required reports that the telemetry extension is optional.
// Returning false means failure to boot this extension will result in an
// OptionalExtensionFailed warning, but boot will continue.
func (e *optionalExample) Required() bool {
	return false
}

// Consumes reports that the telemetry extension has no dependencies and can boot first.
func (e *optionalExample) Consumes() []router.PortName {
	return nil
}

// Provides reports that the telemetry extension registers the telemetry port.
func (e *optionalExample) Provides() []router.PortName {
	return []router.PortName{router.PortOptional}
}

// RouterProvideRegistration registers a dummy telemetry provider.
//
// RouterProvideOptionalCapability: This function demonstrates how to register an optional
// capability into the router. The provider simply prints "optional initialized" as an example.
func (e *optionalExample) RouterProvideRegistration(reg *router.Registry) error {
	provider := struct{ Name string }{Name: "optional-provider"}
	if err := reg.RouterRegisterProvider(router.PortOptional, provider); err != nil {
		return err // router will wrap this in OptionalExtensionFailed
	}
	log.Println("optional initialized")
	return nil
}
