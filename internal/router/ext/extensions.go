package ext

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/michaelbomholt665/wrlk/internal/router"
)

var extensions = []router.Extension{
	&primaryExtension{},
	&secondaryExtension{},
	&tertiaryExtension{},
}

const (
	wrlkEnvKey        = "WRLK_ENV"
	routerProfileKey  = "ROUTER_PROFILE"
	routerAllowAnyKey = "ROUTER_ALLOW_ANY"
)

// RouterBuildExtensionBundle returns the compiled-in optional and application extension bundles.
func RouterBuildExtensionBundle() ([]router.Extension, []router.Extension) {
	return append([]router.Extension(nil), optionalExtensions...), append([]router.Extension(nil), extensions...)
}

// RouterBootExtensions wires optional extensions first, then application extensions.
func RouterBootExtensions(ctx context.Context) ([]error, error) {
	if err := validateRouterBootPolicy(); err != nil {
		return nil, err
	}

	optionalBundle, applicationBundle := RouterBuildExtensionBundle()
	return router.RouterLoadExtensions(optionalBundle, applicationBundle, ctx)
}

// validateRouterBootPolicy rejects router profile combinations that are unsafe at boot.
func validateRouterBootPolicy() error {
	runtimeEnv := normalizeRouterProfile(os.Getenv(wrlkEnvKey))
	declaredProfile := normalizeRouterProfile(os.Getenv(routerProfileKey))

	if declaredProfile != "" && runtimeEnv != "" && declaredProfile != runtimeEnv {
		return &router.RouterError{
			Code: router.RouterEnvironmentMismatch,
			Err: fmt.Errorf(
				"%s=%q does not match %s=%q",
				routerProfileKey,
				declaredProfile,
				wrlkEnvKey,
				runtimeEnv,
			),
		}
	}

	if runtimeEnv == "prod" && parseRouterBoolEnv(os.Getenv(routerAllowAnyKey)) {
		return &router.RouterError{
			Code: router.RouterProfileInvalid,
			Err: fmt.Errorf(
				"%s=true is not allowed when %s=%q",
				routerAllowAnyKey,
				wrlkEnvKey,
				runtimeEnv,
			),
		}
	}

	return nil
}

// normalizeRouterProfile trims and lowercases a router profile value for stable comparison.
func normalizeRouterProfile(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

// parseRouterBoolEnv reports whether an environment value should be treated as enabled.
func parseRouterBoolEnv(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "t", "true", "y", "yes", "on":
		return true
	default:
		return false
	}
}

type primaryExtension struct{}

// Required reports that the primary extension is mandatory for boot.
func (e *primaryExtension) Required() bool {
	return true
}

// Consumes reports that the primary extension has no boot-time port dependencies.
func (e *primaryExtension) Consumes() []router.PortName {
	return nil
}

// Provides reports that the primary extension registers the primary port.
func (e *primaryExtension) Provides() []router.PortName {
	return []router.PortName{router.PortPrimary}
}

// RouterProvideRegistration registers the primary provider into the boot registry.
func (e *primaryExtension) RouterProvideRegistration(reg *router.Registry) error {
	if err := reg.RouterRegisterProvider(router.PortPrimary, struct{ Name string }{Name: "standalone-primary"}); err != nil {
		return fmt.Errorf("register primary provider: %w", err)
	}

	return nil
}

type secondaryExtension struct{}

// Required reports that the secondary extension is mandatory for boot.
func (e *secondaryExtension) Required() bool {
	return true
}

// Consumes reports that the secondary extension has no boot-time port dependencies.
func (e *secondaryExtension) Consumes() []router.PortName {
	return nil
}

// Provides reports that the secondary extension registers the secondary port.
func (e *secondaryExtension) Provides() []router.PortName {
	return []router.PortName{router.PortSecondary}
}

// RouterProvideRegistration registers the secondary provider into the boot registry.
func (e *secondaryExtension) RouterProvideRegistration(reg *router.Registry) error {
	if err := reg.RouterRegisterProvider(router.PortSecondary, struct{ Name string }{Name: "standalone-secondary"}); err != nil {
		return fmt.Errorf("register secondary provider: %w", err)
	}

	return nil
}

type tertiaryExtension struct{}

// Required reports that the tertiary extension is mandatory for boot.
func (e *tertiaryExtension) Required() bool {
	return true
}

// Consumes reports that the tertiary extension has no boot-time port dependencies.
func (e *tertiaryExtension) Consumes() []router.PortName {
	return nil
}

// Provides reports that the tertiary extension registers the tertiary port.
func (e *tertiaryExtension) Provides() []router.PortName {
	return []router.PortName{router.PortTertiary}
}

// RouterProvideRegistration registers the tertiary provider into the boot registry.
func (e *tertiaryExtension) RouterProvideRegistration(reg *router.Registry) error {
	if err := reg.RouterRegisterProvider(router.PortTertiary, struct{ Name string }{Name: "standalone-tertiary"}); err != nil {
		return fmt.Errorf("register tertiary provider: %w", err)
	}

	return nil
}
