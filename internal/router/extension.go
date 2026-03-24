package router

import (
	"context"
	"errors"
	"fmt"
)

// RouterErrorCode is the stable structured router error code.
type RouterErrorCode string

const (
	// PortUnknown indicates registration attempted with an undeclared port.
	PortUnknown RouterErrorCode = "PortUnknown"
	// PortDuplicate indicates registration attempted for an already-registered port.
	PortDuplicate RouterErrorCode = "PortDuplicate"
	// InvalidProvider indicates registration attempted with an invalid provider.
	InvalidProvider RouterErrorCode = "InvalidProvider"
	// PortNotFound indicates resolution attempted for an unregistered port.
	PortNotFound RouterErrorCode = "PortNotFound"
	// RegistryNotBooted indicates resolution attempted before the router was booted.
	RegistryNotBooted RouterErrorCode = "RegistryNotBooted"
	// PortContractMismatch indicates a resolved provider did not satisfy the expected contract.
	PortContractMismatch RouterErrorCode = "PortContractMismatch"
	// RequiredExtensionFailed indicates a required extension failed during boot.
	RequiredExtensionFailed RouterErrorCode = "RequiredExtensionFailed"
	// OptionalExtensionFailed indicates an optional extension failed during boot.
	OptionalExtensionFailed RouterErrorCode = "OptionalExtensionFailed"
	// DependencyOrderViolation indicates an extension consumed a port before it was available.
	DependencyOrderViolation RouterErrorCode = "DependencyOrderViolation"
	// AsyncInitTimeout indicates an async extension did not finish before timeout/cancellation.
	AsyncInitTimeout RouterErrorCode = "AsyncInitTimeout"
	// MultipleInitializations indicates the router was booted more than once.
	MultipleInitializations RouterErrorCode = "MultipleInitializations"
	// RouterCyclicDependency indicates a circular dependency was detected during boot.
	RouterCyclicDependency RouterErrorCode = "RouterCyclicDependency"
	// PortAccessDenied indicates a consumer was denied access to a restricted port.
	PortAccessDenied RouterErrorCode = "PortAccessDenied"
	// RouterProfileInvalid indicates the configured router profile contains a forbidden combination.
	RouterProfileInvalid RouterErrorCode = "RouterProfileInvalid"
	// RouterEnvironmentMismatch indicates the declared router profile does not match the runtime environment.
	RouterEnvironmentMismatch RouterErrorCode = "RouterEnvironmentMismatch"
)

// RouterError is the structured router error type.
type RouterError struct {
	Code       RouterErrorCode
	Port       PortName
	ConsumerID string
	Err        error
}

// Error returns the router error message.
func (e *RouterError) Error() string {
	if e == nil {
		return ""
	}

	return renderRouterError(e)
}

// Unwrap returns the underlying cause.
func (e *RouterError) Unwrap() error {
	if e == nil {
		return nil
	}

	return e.Err
}

// RouterErrorFormatter formats extension errors.
type RouterErrorFormatter func(err error) error

// Extension declares a router extension.
type Extension interface {
	Required() bool
	Consumes() []PortName
	Provides() []PortName
	RouterProvideRegistration(reg *Registry) error
}

// AsyncExtension declares an async-capable router extension.
type AsyncExtension interface {
	Extension
	RouterProvideAsyncRegistration(reg *Registry, ctx context.Context) error
}

// ErrorFormattingExtension declares a custom error formatter.
type ErrorFormattingExtension interface {
	Extension
	ErrorFormatter() RouterErrorFormatter
}

// Registry is the extension write handle for the local boot registry.
type Registry struct {
	ports        *map[PortName]Provider
	restrictions *map[PortName][]string
}

// RouterRegisterProvider registers a provider into the local boot registry.
func (r *Registry) RouterRegisterProvider(port PortName, provider Provider) error {
	if r == nil || r.ports == nil {
		return fmt.Errorf("router register provider: registry is nil")
	}

	if !RouterValidatePortName(port) {
		return &RouterError{Code: PortUnknown, Port: port}
	}

	if provider == nil {
		return &RouterError{Code: InvalidProvider}
	}

	localPorts := *r.ports
	if _, exists := localPorts[port]; exists {
		return &RouterError{Code: PortDuplicate, Port: port}
	}

	localPorts[port] = provider

	return nil
}

// RouterRegisterPortRestriction adds an access restriction to a port during boot.
func (r *Registry) RouterRegisterPortRestriction(port PortName, allowedConsumerIDs []string) error {
	if r == nil || r.restrictions == nil {
		return fmt.Errorf("router register restriction: registry is nil")
	}

	if !RouterValidatePortName(port) {
		return &RouterError{Code: PortUnknown, Port: port}
	}

	localRestrictions := *r.restrictions
	// Overwrite or append? In boot, we probably overwrite or combine.
	// But usually there's only one policy registration per port. Let's just set it.
	localRestrictions[port] = allowedConsumerIDs

	return nil
}

// RouterLoadExtensions loads extension registrations and publishes the registry.
func RouterLoadExtensions(
	optionalExts []Extension,
	exts []Extension,
	ctx context.Context,
) ([]error, error) {
	if registry.Load() != nil {
		return nil, &RouterError{Code: MultipleInitializations}
	}

	if ctx == nil {
		ctx = context.Background()
	}

	localPorts := make(map[PortName]Provider)
	localRestrictions := make(map[PortName][]string)
	localRegistry := &Registry{ports: &localPorts, restrictions: &localRestrictions}
	warnings := make([]error, 0)

	optionalWarnings, err := routerLoadExtensionLayer(
		localRegistry,
		optionalExts,
		ctx,
	)
	if err != nil {
		return nil, err
	}
	warnings = append(warnings, optionalWarnings...)

	applicationWarnings, err := routerLoadExtensionLayer(
		localRegistry,
		exts,
		ctx,
	)
	if err != nil {
		return nil, err
	}
	warnings = append(warnings, applicationWarnings...)

	snapshot := &routerSnapshot{
		providers:    localPorts,
		restrictions: localRestrictions,
	}

	if !registry.CompareAndSwap(nil, snapshot) {
		return nil, &RouterError{Code: MultipleInitializations}
	}

	return warnings, nil
}

// routerLoadExtensionLayer boots one extension layer against the local registry.
func routerLoadExtensionLayer(
	registryHandle *Registry,
	extensions []Extension,
	ctx context.Context,
) ([]error, error) {
	warnings := make([]error, 0)

	sortedExts, err := RouterSortExtensionsByDependency(extensions)
	if err != nil {
		return nil, err
	}

	for _, ext := range sortedExts {
		if err := routerCheckExtensionDependencies(registryHandle, ext); err != nil {
			return nil, err
		}

		registrationWarnings, err := routerHandleExtensionRegistration(
			registryHandle,
			ext,
			ctx,
		)
		if err != nil {
			return nil, err
		}
		warnings = append(warnings, registrationWarnings...)
	}

	return warnings, nil
}

// routerHandleExtensionRegistration executes one extension's sync and async registration paths.
func routerHandleExtensionRegistration(
	registryHandle *Registry,
	ext Extension,
	ctx context.Context,
) ([]error, error) {
	if ext == nil {
		return nil, nil
	}

	if err := ext.RouterProvideRegistration(registryHandle); err != nil {
		return routerHandleExtensionFailure(ext, err)
	}

	asyncExt, ok := ext.(AsyncExtension)
	if !ok {
		return nil, nil
	}

	if err := asyncExt.RouterProvideAsyncRegistration(registryHandle, ctx); err != nil {
		asyncErr := routerClassifyAsyncError(err)
		if asyncErr != nil {
			return nil, asyncErr
		}

		return routerHandleExtensionFailure(ext, err)
	}

	return nil, nil
}

// routerHandleExtensionFailure classifies one extension failure as warning or fatal error.
func routerHandleExtensionFailure(ext Extension, err error) ([]error, error) {
	classifiedErr := routerClassifyExtensionError(ext, err)
	if ext.Required() {
		return nil, classifiedErr
	}

	return []error{classifiedErr}, nil
}

// routerCheckExtensionDependencies verifies an extension's declared boot dependencies.
func routerCheckExtensionDependencies(registryHandle *Registry, ext Extension) error {
	if ext == nil {
		return nil
	}

	for _, port := range ext.Consumes() {
		if _, exists := (*registryHandle.ports)[port]; exists {
			continue
		}

		return &RouterError{
			Code: DependencyOrderViolation,
			Port: port,
		}
	}

	return nil
}

// routerClassifyAsyncError maps context-driven async failures to router errors.
func routerClassifyAsyncError(err error) error {
	if err == nil {
		return nil
	}

	if err == context.DeadlineExceeded || err == context.Canceled {
		return &RouterError{
			Code: AsyncInitTimeout,
			Err:  err,
		}
	}

	return nil
}

// routerClassifyExtensionError classifies extension failures as fatal or warning outcomes.
func routerClassifyExtensionError(ext Extension, err error) error {
	if err == nil {
		return nil
	}

	var routerErr *RouterError
	if errors.As(err, &routerErr) {
		switch routerErr.Code {
		case PortUnknown, PortDuplicate, InvalidProvider, DependencyOrderViolation, AsyncInitTimeout, MultipleInitializations, RouterProfileInvalid, RouterEnvironmentMismatch:
			return err
		}
	}

	formattedErr := routerFormatExtensionError(ext, err)
	code := RequiredExtensionFailed
	if ext != nil && !ext.Required() {
		code = OptionalExtensionFailed
	}

	if formattedRouterErr, ok := formattedErr.(*RouterError); ok {
		switch formattedRouterErr.Code {
		case RequiredExtensionFailed, OptionalExtensionFailed:
			return &RouterError{
				Code:       code,
				Port:       formattedRouterErr.Port,
				ConsumerID: formattedRouterErr.ConsumerID,
				Err:        formattedRouterErr.Err,
			}
		}
	}

	return &RouterError{
		Code: code,
		Err:  formattedErr,
	}
}

// routerFormatExtensionError applies any extension-specific error formatter.
func routerFormatExtensionError(ext Extension, err error) error {
	if err == nil || ext == nil {
		return err
	}

	formattingExt, ok := ext.(ErrorFormattingExtension)
	if !ok {
		return err
	}

	formatter := formattingExt.ErrorFormatter()
	if formatter == nil {
		return err
	}

	formattedErr := formatter(err)
	if formattedErr == nil {
		return err
	}

	return formattedErr
}

// RouterSortExtensionsByDependency topologically sorts extensions based on their Consumes() declarations.
func RouterSortExtensionsByDependency(exts []Extension) ([]Extension, error) {
	if len(exts) <= 1 {
		return exts, nil
	}

	provides, err := RouterBuildDependencyGraph(exts)
	if err != nil {
		return nil, err
	}

	if err := routerCheckCycles(exts, provides); err != nil {
		return nil, err
	}

	inDegree, adj := routerBuildKahnGraph(exts, provides)
	return routerExecuteKahnSort(exts, inDegree, adj), nil
}

// routerCheckCycles converts the port provides map to a port-level dependency graph and checks for cycles.
func routerCheckCycles(exts []Extension, provides map[PortName]int) error {
	portGraph := make(map[PortName][]PortName)
	for port, extIdx := range provides {
		portGraph[port] = exts[extIdx].Consumes()
	}
	return RouterDetectCyclicDependency(portGraph)
}

// routerBuildKahnGraph builds the in-degree array and adjacency list for Kahn's topological sort.
func routerBuildKahnGraph(exts []Extension, provides map[PortName]int) ([]int, [][]int) {
	inDegree := make([]int, len(exts))
	adj := make([][]int, len(exts))

	for i, ext := range exts {
		if ext == nil {
			continue
		}
		for _, consumed := range ext.Consumes() {
			if providerIdx, exists := provides[consumed]; exists {
				adj[providerIdx] = append(adj[providerIdx], i)
				inDegree[i]++
			}
		}
	}
	return inDegree, adj
}

// routerExecuteKahnSort executes Kahn's algorithm to return a topologically sorted slice of extensions.
func routerExecuteKahnSort(exts []Extension, inDegree []int, adj [][]int) []Extension {
	var queue []int
	for i, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, i)
		}
	}

	var result []Extension
	visited := make([]bool, len(exts))

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		result = append(result, exts[curr])
		visited[curr] = true

		for _, neighbor := range adj[curr] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	for i, v := range visited {
		if !v {
			result = append(result, exts[i])
		}
	}

	return result
}

// RouterBuildDependencyGraph maps each provided port to the index of the extension that provides it.
func RouterBuildDependencyGraph(exts []Extension) (map[PortName]int, error) {
	provides := make(map[PortName]int)
	for i, ext := range exts {
		if ext == nil {
			continue
		}

		for _, port := range ext.Provides() {
			if _, exists := provides[port]; exists {
				return nil, &RouterError{Code: PortDuplicate, Port: port}
			}
			provides[port] = i
		}
	}
	return provides, nil
}

// RouterCollectProvidedPorts executes one extension registration against an isolated
// in-memory registry and returns the ports it actually registered.
func RouterCollectProvidedPorts(ext Extension) ([]PortName, error) {
	if ext == nil {
		return nil, nil
	}

	localPorts := make(map[PortName]Provider)
	localRestrictions := make(map[PortName][]string)
	registryHandle := &Registry{ports: &localPorts, restrictions: &localRestrictions}

	if err := ext.RouterProvideRegistration(registryHandle); err != nil {
		return nil, fmt.Errorf("collect provided ports: %w", err)
	}

	registeredPorts := make([]PortName, 0, len(localPorts))
	for port := range localPorts {
		registeredPorts = append(registeredPorts, port)
	}

	return registeredPorts, nil
}

// RouterDetectCyclicDependency detects if there are any cycles in the dependency graph.
func RouterDetectCyclicDependency(graph map[PortName][]PortName) error {
	visited := make(map[PortName]bool)
	recStack := make(map[PortName]bool)

	for port := range graph {
		if !visited[port] && routerHasCycle(port, graph, visited, recStack) {
			return &RouterError{Code: RouterCyclicDependency}
		}
	}
	return nil
}

// routerHasCycle recursively checks paths in the port dependency graph to detect cycles.
func routerHasCycle(port PortName, graph map[PortName][]PortName, visited, recStack map[PortName]bool) bool {
	visited[port] = true
	recStack[port] = true

	for _, neighbor := range graph[port] {
		if !visited[neighbor] {
			if routerHasCycle(neighbor, graph, visited, recStack) {
				return true
			}
		} else if recStack[neighbor] {
			return true
		}
	}

	recStack[port] = false
	return false
}
