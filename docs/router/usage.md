# Router Usage

Use the router as the host wiring surface, not the adapter packages directly.

## Import Rule

Route capability imports through these layers:

- Port contract: `internal/ports/walk.go`
- Router boot wiring: `internal/router/ext/extensions.go`
- Concrete adapter: `internal/adapters/walk/extension.go`

The intended dependency direction is:

```text
consumer -> internal/ports + internal/router
host boot -> internal/router/ext
internal/router/ext -> internal/adapters/*
```

Application code should not import `internal/adapters/walk` to get a walker. The adapter is registered in the router, and consumers resolve the port from the published registry.

## How To Use It

1. Boot the router once at host startup through `internal/router/ext`.
2. Resolve the provider from `internal/router` by port name.
3. Cast the resolved provider to the matching interface from `internal/ports`.
4. Call the port method, not the adapter type directly.

Example:

```go
import (
	"context"
	"fmt"
	"io/fs"
	"time"

	"policycheck/internal/ports"
	"policycheck/internal/router"
	"policycheck/internal/router/ext"
)

const routerBootTimeout = 30 * time.Second

func bootRouter() error {
	ctx, cancel := context.WithTimeout(context.Background(), routerBootTimeout)
	defer cancel()

	_, err := ext.RouterBootExtensions(ctx)
	return err
}

func walkDirectory(root string, walkFn fs.WalkDirFunc) error {
	provider, err := router.RouterResolveProvider(router.PortWalk)
	if err != nil {
		return fmt.Errorf("resolve walk provider: %w", err)
	}

	walkProvider, ok := provider.(ports.WalkProvider)
	if !ok {
		return &router.RouterError{
			Code: router.PortContractMismatch,
			Port: router.PortWalk,
		}
	}

	return walkProvider.WalkDirectoryTree(root, walkFn)
}
```

## Restricted Ports

Some ports may have access restrictions. Use `RouterResolveRestrictedPort` to resolve a provider with consumer access control:

```go
func resolveRestrictedPort(port router.PortName, consumerID string) (Provider, error) {
	provider, err := router.RouterResolveRestrictedPort(port, consumerID)
	if err != nil {
		return nil, fmt.Errorf("resolve restricted port %q for consumer %q: %w", port, consumerID, err)
	}
	return provider, nil
}

// Example: Resolve config port for a trusted consumer
provider, err := resolveRestrictedPort(router.PortConfig, "trusted-consumer")
```

Access is denied if the consumer ID is not in the allowed list for the restricted port.

## CLI Tools

The router package includes two CLI tools for managing ports and verification:

### portgen - Port Generation

Generates new ports and updates validation automatically:

```bash
# Add a new port
go run ./internal/router/tools/portgen add --name PortFoo --value foo

# Dry-run to see what would happen
go run ./internal/router/tools/portgen add --name PortFoo --value foo --dry-run
```

### wrlk - Router Lock Verification

Manages the router lock file for verifiable bundle state:

```bash
# Verify lock matches tracked files
go run ./internal/router/tools/wrlk lock verify

# Update lock after changes
go run ./internal/router/tools/wrlk lock update

# Run live verification session
go run ./internal/router/tools/wrlk live run --expect participant1 --expect participant2 --timeout 30s
```

The lock tracks SHA256 checksums of `internal/router/extension.go` and `internal/router/registry.go`.

#### Live Report Protocol

Participants post JSON reports to `/report`.

- `{"id":"alpha","status":"success"}` returns `202 Accepted`
- `{"id":"beta","status":"failure","error":"..."}`
  returns `400 Bad Request` and records the failure immediately

Treat `400` on a failure report as "failure recorded", not as a transport-level unknown error.

## What Lives Where

- `internal/ports/walk.go` defines the stable contract: `WalkProvider`.
- `internal/adapters/walk/extension.go` implements that contract with the stdlib walker.
- `internal/router/ext/extensions.go` is the only wiring package that imports both `internal/router` and `internal/adapters/*` and registers `PortWalk`.
- `internal/router/ext/optional_extensions.go` is the explicit compile-time allowlist for optional extensions.
- `internal/router/tools/portgen` generates new port constants and validation cases.
- `internal/router/tools/wrlk` manages lock verification and live sessions.

If you add another capability, follow the same pattern: define the interface in `internal/ports`, implement it in an adapter, and register that adapter in `internal/router/ext` so the rest of the codebase depends on the port instead of the concrete package.
