package router_test

import (
	"context"

	"github.com/michaelbomholt665/wrlk/internal/router"

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

func withRestriction(provider router.Provider, allowed []string) *restrictionMockExtension {
	ext := &restrictionMockExtension{
		RestrictPort: router.PortPrimary,
		AllowedUsers: allowed,
	}
	ext.IsRequired = true
	ext.RegistersPort = router.PortPrimary
	ext.RegistersProvider = provider
	return ext
}

func (s *RouterSuite) TestRestricted_TrustedConsumer_Resolves() {
	ext := withRestriction(struct{}{}, []string{"trusted-user"})

	_, err := router.RouterLoadExtensions(nil, []router.Extension{ext}, context.Background())
	require.NoError(s.T(), err)

	provider, err := router.RouterResolveRestrictedPort(router.PortPrimary, "trusted-user")
	require.NoError(s.T(), err, "trusted consumer should resolve port")
	require.NotNil(s.T(), provider, "provider should be non-nil")
}

func (s *RouterSuite) TestRestricted_UntrustedConsumer_AccessDenied() {
	ext := withRestriction(struct{}{}, []string{"trusted-user"})

	_, err := router.RouterLoadExtensions(nil, []router.Extension{ext}, context.Background())
	require.NoError(s.T(), err)

	provider, err := router.RouterResolveRestrictedPort(router.PortPrimary, "untrusted-user")
	require.Error(s.T(), err)
	require.Nil(s.T(), provider)

	var routerErr *router.RouterError
	require.ErrorAs(s.T(), err, &routerErr)
	assert.Equal(s.T(), router.PortAccessDenied, routerErr.Code)
	assert.Equal(s.T(), router.PortPrimary, routerErr.Port)
	assert.Equal(s.T(), "untrusted-user", routerErr.ConsumerID)
	assert.Contains(s.T(), err.Error(), "untrusted-user")
	assert.Contains(s.T(), err.Error(), "primary")
}

func (s *RouterSuite) TestRestricted_EmptyConsumerID_AccessDenied() {
	ext := withRestriction(struct{}{}, []string{"trusted-user"})

	_, err := router.RouterLoadExtensions(nil, []router.Extension{ext}, context.Background())
	require.NoError(s.T(), err)

	provider, err := router.RouterResolveRestrictedPort(router.PortPrimary, "")
	require.Error(s.T(), err)
	require.Nil(s.T(), provider)

	var routerErr *router.RouterError
	require.ErrorAs(s.T(), err, &routerErr)
	assert.Equal(s.T(), router.PortAccessDenied, routerErr.Code)
	assert.Equal(s.T(), "", routerErr.ConsumerID)
}

func (s *RouterSuite) TestRestricted_AnyConsumer_WildcardResolves() {
	ext := withRestriction(struct{}{}, []string{"Any"})

	_, err := router.RouterLoadExtensions(nil, []router.Extension{ext}, context.Background())
	require.NoError(s.T(), err)

	provider, err := router.RouterResolveRestrictedPort(router.PortPrimary, "some-random-consumer")
	require.NoError(s.T(), err)
	require.NotNil(s.T(), provider)
}

func (s *RouterSuite) TestRestricted_UnrestrictedPort_AlwaysResolvable() {
	ext := requiredExtension(router.PortSecondary, struct{}{})

	_, err := router.RouterLoadExtensions(nil, []router.Extension{ext}, context.Background())
	require.NoError(s.T(), err, "router boot failed")

	provider, err := router.RouterResolveProvider(router.PortSecondary)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), provider)

	provider2, err := router.RouterResolveRestrictedPort(router.PortSecondary, "any-user")
	require.NoError(s.T(), err)
	require.NotNil(s.T(), provider2)
}

func (s *RouterSuite) TestRestricted_TrustPolicy_InMutableWiringOnly() {
	ext := withRestriction(struct{}{}, []string{"trusted"})

	require.NotNil(s.T(), ext)
}
