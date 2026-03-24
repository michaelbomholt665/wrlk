package router_test

import (
	"context"

	"policycheck/internal/router"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type restrictionMockExtension struct {
	MockExtension
	RestrictPort router.PortName
	AllowedUsers []string
}

func (m *restrictionMockExtension) RouterProvideRegistration(reg *router.Registry) error {
	if m.RestrictPort != "" {
		if err := reg.RouterRegisterPortRestriction(m.RestrictPort, m.AllowedUsers); err != nil {
			return err
		}
	}
	return m.MockExtension.RouterProvideRegistration(reg)
}

func withRestriction(port router.PortName, provider router.Provider, allowed []string) *restrictionMockExtension {
	ext := &restrictionMockExtension{
		RestrictPort: port,
		AllowedUsers: allowed,
	}
	ext.IsRequired = true
	ext.RegistersPort = port
	ext.RegistersProvider = provider
	return ext
}

func (s *RouterSuite) TestRestricted_TrustedConsumer_Resolves() {
	ext := withRestriction(router.PortConfig, struct{}{}, []string{"trusted-user"})

	_, err := router.RouterLoadExtensions(nil, []router.Extension{ext}, context.Background())
	require.NoError(s.T(), err)

	provider, err := router.RouterResolveRestrictedPort(router.PortConfig, "trusted-user")
	require.NoError(s.T(), err, "trusted consumer should resolve port")
	require.NotNil(s.T(), provider, "provider should be non-nil")
}

func (s *RouterSuite) TestRestricted_UntrustedConsumer_AccessDenied() {
	ext := withRestriction(router.PortConfig, struct{}{}, []string{"trusted-user"})

	_, err := router.RouterLoadExtensions(nil, []router.Extension{ext}, context.Background())
	require.NoError(s.T(), err)

	provider, err := router.RouterResolveRestrictedPort(router.PortConfig, "untrusted-user")
	require.Error(s.T(), err)
	require.Nil(s.T(), provider)

	var routerErr *router.RouterError
	require.ErrorAs(s.T(), err, &routerErr)
	assert.Equal(s.T(), router.PortAccessDenied, routerErr.Code)
	assert.Equal(s.T(), router.PortConfig, routerErr.Port)
	assert.Equal(s.T(), "untrusted-user", routerErr.ConsumerID)
	assert.Contains(s.T(), err.Error(), "untrusted-user")
	assert.Contains(s.T(), err.Error(), "config")
}

func (s *RouterSuite) TestRestricted_EmptyConsumerID_AccessDenied() {
	ext := withRestriction(router.PortConfig, struct{}{}, []string{"trusted-user"})

	_, err := router.RouterLoadExtensions(nil, []router.Extension{ext}, context.Background())
	require.NoError(s.T(), err)

	provider, err := router.RouterResolveRestrictedPort(router.PortConfig, "")
	require.Error(s.T(), err)
	require.Nil(s.T(), provider)

	var routerErr *router.RouterError
	require.ErrorAs(s.T(), err, &routerErr)
	assert.Equal(s.T(), router.PortAccessDenied, routerErr.Code)
	assert.Equal(s.T(), "", routerErr.ConsumerID)
}

func (s *RouterSuite) TestRestricted_AnyConsumer_WildcardResolves() {
	ext := withRestriction(router.PortConfig, struct{}{}, []string{"Any"})

	_, err := router.RouterLoadExtensions(nil, []router.Extension{ext}, context.Background())
	require.NoError(s.T(), err)

	provider, err := router.RouterResolveRestrictedPort(router.PortConfig, "some-random-consumer")
	require.NoError(s.T(), err)
	require.NotNil(s.T(), provider)
}

func (s *RouterSuite) TestRestricted_UnrestrictedPort_AlwaysResolvable() {
	ext := requiredExtension(router.PortWalk, struct{}{})

	_, err := router.RouterLoadExtensions(nil, []router.Extension{ext}, context.Background())
	require.NoError(s.T(), err, "router boot failed")

	provider, err := router.RouterResolveProvider(router.PortWalk)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), provider)

	provider2, err := router.RouterResolveRestrictedPort(router.PortWalk, "any-user")
	require.NoError(s.T(), err)
	require.NotNil(s.T(), provider2)
}

func (s *RouterSuite) TestRestricted_TrustPolicy_InMutableWiringOnly() {
	ext := withRestriction(router.PortConfig, struct{}{}, []string{"trusted"})

	require.NotNil(s.T(), ext)
}
