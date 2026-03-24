package router

// PortName is a typed router port identifier.
type PortName string

// Provider is the registered implementation for a router port.
type Provider any

const (
	// PortConfig is the configuration provider port.
	PortConfig PortName = "config"
	// PortWalk is the filesystem walk provider port.
	PortWalk PortName = "walk"
	// PortScanner is the scanner provider port.
	PortScanner PortName = "scanner"
	// PortTelemetry is the telemetry provider port.
	PortTelemetry PortName = "telemetry"
)
