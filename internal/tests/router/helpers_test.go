package router_test

import (
	"context"
	"testing"
	"time"

	"policycheck/internal/router"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type MockExtension struct {
	mock.Mock

	BootError         error
	AsyncDelay        time.Duration
	IsRequired        bool
	ConsumedPorts     []router.PortName
	ProvidedPorts     []router.PortName
	RegistersPort     router.PortName
	RegistersProvider router.Provider
	RegistrationCalls *int
}

func (m *MockExtension) Required() bool {
	return m.IsRequired
}

func (m *MockExtension) Consumes() []router.PortName {
	return m.ConsumedPorts
}

func (m *MockExtension) Provides() []router.PortName {
	if len(m.ProvidedPorts) > 0 {
		return m.ProvidedPorts
	}
	if m.RegistersPort == "" {
		return nil
	}

	return []router.PortName{m.RegistersPort}
}

func (m *MockExtension) RouterProvideRegistration(reg *router.Registry) error {
	if m.RegistrationCalls != nil {
		*m.RegistrationCalls++
	}

	if m.BootError != nil {
		return m.BootError
	}

	if m.RegistersPort == "" {
		return nil
	}

	return reg.RouterRegisterProvider(m.RegistersPort, m.RegistersProvider)
}

type MockAsyncExtension struct {
	MockExtension

	AsyncBootError         error
	AsyncRegistersPort     router.PortName
	AsyncRegistersProvider router.Provider
}

func (m *MockAsyncExtension) RouterProvideAsyncRegistration(
	reg *router.Registry,
	ctx context.Context,
) error {
	if m.AsyncBootError != nil {
		return m.AsyncBootError
	}

	if m.AsyncDelay > 0 {
		timer := time.NewTimer(m.AsyncDelay)
		defer timer.Stop()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}
	}

	if m.AsyncRegistersPort == "" {
		return nil
	}

	return reg.RouterRegisterProvider(m.AsyncRegistersPort, m.AsyncRegistersProvider)
}

type MockErrorFormattingExtension struct {
	MockExtension
	Formatter router.RouterErrorFormatter
}

func (m *MockErrorFormattingExtension) ErrorFormatter() router.RouterErrorFormatter {
	return m.Formatter
}

func requiredExtension(
	port router.PortName,
	provider router.Provider,
) *MockExtension {
	return &MockExtension{
		IsRequired:        true,
		RegistersPort:     port,
		RegistersProvider: provider,
	}
}

func failingRequiredExtension(err error) *MockExtension {
	return &MockExtension{
		BootError:  err,
		IsRequired: true,
	}
}

func failingOptionalExtension(err error) *MockExtension {
	return &MockExtension{
		BootError:  err,
		IsRequired: false,
	}
}

func asyncExtension(
	port router.PortName,
	delay time.Duration,
) *MockAsyncExtension {
	return &MockAsyncExtension{
		MockExtension: MockExtension{
			AsyncDelay: delay,
			IsRequired: true,
		},
		AsyncRegistersPort:     port,
		AsyncRegistersProvider: struct{}{},
	}
}

func unknownPortExtension() *MockExtension {
	return &MockExtension{
		IsRequired:        true,
		RegistersPort:     router.PortName("unknown_port"),
		RegistersProvider: struct{}{},
	}
}

type RouterSuite struct {
	suite.Suite
}

func (s *RouterSuite) SetupTest() {
	router.RouterResetForTest()
}

func TestRouterSuite(t *testing.T) {
	suite.Run(t, new(RouterSuite))
}
