package router_test

import (
	"context"
	"policycheck/internal/router"
	"sync"
	"testing"
)

// Run with: go test -tags test '-bench=.' -benchtime=3s './internal/tests/router/...'
func BenchmarkRouterResolve(b *testing.B) {
	router.RouterResetForTest()

	if _, err := router.RouterLoadExtensions(
		nil,
		[]router.Extension{
			requiredExtension(
				router.PortConfig,
				&configProviderStub{path: "isr.toml"},
			),
		},
		context.Background(),
	); err != nil {
		b.Fatalf("RouterLoadExtensions: %v", err)
	}

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := router.RouterResolveProvider(router.PortConfig); err != nil {
				b.Fatalf("RouterResolveProvider: %v", err)
			}
		}
	})
}

// BenchmarkRouterResolveRWMutex is the same read-path operation as
// BenchmarkRouterResolve but protected by an sync.RWMutex instead of
// an atomic.Pointer, so benchstat can show the direct cost difference.
func BenchmarkRouterResolveRWMutex(b *testing.B) {
	router.RouterResetForTest()

	if _, err := router.RouterLoadExtensions(
		nil,
		[]router.Extension{
			requiredExtension(
				router.PortConfig,
				&configProviderStub{path: "isr.toml"},
			),
		},
		context.Background(),
	); err != nil {
		b.Fatalf("RouterLoadExtensions: %v", err)
	}

	// Local RWMutex registry seeded with the same port → provider mapping.
	var mu sync.RWMutex

	regMap := map[router.PortName]router.Provider{
		router.PortConfig: &configProviderStub{path: "isr.toml"},
	}

	// resolveRW mirrors RouterResolveProvider using RWMutex.
	resolveRW := func(port router.PortName) (router.Provider, bool) {
		mu.RLock()
		p, ok := regMap[port]
		mu.RUnlock()

		return p, ok
	}

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, ok := resolveRW(router.PortConfig); !ok {
				b.Fatal("resolveRW: port not found")
			}
		}
	})
}
