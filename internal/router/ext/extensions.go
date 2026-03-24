package ext

import (
	"context"
	"fmt"
	"os"
	"strings"

	adapterconfig "policycheck/internal/adapters/config"
	adapterscanners "policycheck/internal/adapters/scanners"
	adapterwalk "policycheck/internal/adapters/walk"
	"policycheck/internal/router"
)

var extensions = []router.Extension{
	&configExtension{},
	&walkExtension{},
	&scannerExtension{},
}

const (
	policyCheckEnvKey = "POLICYCHECK_ENV"
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
	runtimeEnv := normalizeRouterProfile(os.Getenv(policyCheckEnvKey))
	declaredProfile := normalizeRouterProfile(os.Getenv(routerProfileKey))

	if declaredProfile != "" && runtimeEnv != "" && declaredProfile != runtimeEnv {
		return &router.RouterError{
			Code: router.RouterEnvironmentMismatch,
			Err: fmt.Errorf(
				"%s=%q does not match %s=%q",
				routerProfileKey,
				declaredProfile,
				policyCheckEnvKey,
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
				policyCheckEnvKey,
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

type configExtension struct{}

// Required reports that the config extension is mandatory for boot.
func (e *configExtension) Required() bool {
	return true
}

// Consumes reports that the config extension has no boot-time port dependencies.
func (e *configExtension) Consumes() []router.PortName {
	return nil
}

// Provides reports that the config extension registers the config port.
func (e *configExtension) Provides() []router.PortName {
	return []router.PortName{router.PortConfig}
}

// RouterProvideRegistration registers the config provider into the boot registry.
func (e *configExtension) RouterProvideRegistration(reg *router.Registry) error {
	if err := reg.RouterRegisterProvider(router.PortConfig, adapterconfig.NewConfigProvider()); err != nil {
		return fmt.Errorf("register config provider: %w", err)
	}

	return nil
}

type walkExtension struct{}

// Required reports that the walk extension is mandatory for boot.
func (e *walkExtension) Required() bool {
	return true
}

// Consumes reports that the walk extension has no boot-time port dependencies.
func (e *walkExtension) Consumes() []router.PortName {
	return nil
}

// Provides reports that the walk extension registers the walk port.
func (e *walkExtension) Provides() []router.PortName {
	return []router.PortName{router.PortWalk}
}

// RouterProvideRegistration registers the walk provider into the boot registry.
func (e *walkExtension) RouterProvideRegistration(reg *router.Registry) error {
	if err := reg.RouterRegisterProvider(router.PortWalk, adapterwalk.NewWalkProvider()); err != nil {
		return fmt.Errorf("register walk provider: %w", err)
	}

	return nil
}

type scannerExtension struct{}

// Required reports that the scanner extension is mandatory for boot.
func (e *scannerExtension) Required() bool {
	return true
}

// Consumes reports that the scanner extension has no boot-time port dependencies.
func (e *scannerExtension) Consumes() []router.PortName {
	return nil
}

// Provides reports that the scanner extension registers the scanner port.
func (e *scannerExtension) Provides() []router.PortName {
	return []router.PortName{router.PortScanner}
}

// RouterProvideRegistration registers the scanner provider into the boot registry.
func (e *scannerExtension) RouterProvideRegistration(reg *router.Registry) error {
	if err := reg.RouterRegisterProvider(router.PortScanner, adapterscanners.NewScannerProvider()); err != nil {
		return fmt.Errorf("register scanner provider: %w", err)
	}

	return nil
}
