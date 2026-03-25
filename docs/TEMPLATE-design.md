# [Component Name] — Design Document

**Version:** 0.1.0  
**Status:** Draft — design in progress  
**Scope:** [What this component does]

## Overview

[One paragraph describing the component's purpose and value proposition.]

**Key properties:**
- [Property 1]
- [Property 2]
- [Property 3]

[Copy this folder into any Go project. Or: This component lives in `internal/[component]/`.]

## Core Purpose

**Primary problem solved:** [Describe the main problem this component addresses.]

**Secondary problem solved:** [Describe any secondary concerns.]

**Out of scope:** [What's explicitly not covered by this design.]

***

## What It Is vs What It Is Not

| **Is**         | **Is Not**         |
| -------------- | ------------------ |
| [Capability 1] | [Non-capability 1] |
| [Capability 2] | [Non-capability 2] |
| [Capability 3] | [Non-capability 3] |
| [Capability 4] | [Non-capability 4] |

***

## Folder Structure

```text
internal/[component]/
│
├── MUTABLE — host project wiring
│   ├── [file1.go]        # [description]
│   └── [file2.go]        # [description]
│
├── FROZEN — never edit directly
│   ├── [file3.go]        # [description]
│   └── [file4.go]        # [description]
│
└── tools/
    └── [tool]/
        └── main.go       # [description]
```

## File Responsibilities

### `file.go` — MUTABLE

```go
package [component]

// [Code sample if applicable]
```

**Rules:**
- [Rule 1]
- [Rule 2]
- [Rule 3]

### `file.go` — FROZEN

[Description of frozen file responsibilities.]

Contains:
- [Item 1]
- [Item 2]
- [Item 3]

## Bootstrap Contract (main.go)

```go
package main

import (
    "context"
    "log"
    "time"

    "your-project/internal/[component]"
)

func main() {
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    // [Bootstrap call]
    if err != nil {
        log.Fatal(err)
    }

    // [Continue startup]
}
```

### Bootstrap Semantics

- [Semantic 1]
- [Semantic 2]
- [Semantic 3]

***

## Data Model

### State Model

[Describe the state management approach.]

### Consequences

1. [Consequence 1]
2. [Consequence 2]
3. [Consequence 3]

***

## Error Catalog

[Structured errors for all failure modes.]

| Code         | Category   | Notes         |
| ------------ | ---------- | ------------- |
| `ErrorCode1` | [Category] | [Description] |
| `ErrorCode2` | [Category] | [Description] |
| `ErrorCode3` | [Category] | [Description] |

**Mandated message formats:**
- `ErrorCode1`: [Format requirement]
- `ErrorCode2`: [Format requirement]

***

## Interface Contracts

```go
type [InterfaceName] interface {
    // Method 1 does X
    Method1() [return type]

    // Method 2 does Y
    Method2() [return type]
}
```

### Extension Interface

```go
type Extension interface {
    // Required declares whether boot failure is fatal.
    Required() bool

    // Consumes declares dependencies.
    Consumes() []DependencyType

    // PerformRegistration performs the actual work.
    PerformRegistration(reg *Registry) error
}
```

***

## Adding New Capabilities

**New capability:**
1. Add constant to `[file].go`
2. Add validation case to `[file].go`
3. Define interface in `internal/ports/`
4. Implement adapter + `Extension()` in adapter package
5. Add import + line to wiring file

**Swap implementation:** [How to swap implementations.]

***

## AI Guardrails (Development Constraints)

| **Mechanism** | **Purpose** | **Effect** |
| ------------- | ----------- | ---------- |
| [Mechanism 1] | [Purpose]   | [Effect]   |
| [Mechanism 2] | [Purpose]   | [Effect]   |
| [Mechanism 3] | [Purpose]   | [Effect]   |

***

## Security Model

| **Concern** | **Contribution** | **Claim** |
| ----------- | ---------------- | --------- |
| [Concern 1] | [Contribution]   | [Claim]   |
| [Concern 2] | [Contribution]   | [Claim]   |

***

## v0.x Features (In Scope)

### Feature Name — Phase X

[Description and status]

**Status:** [Implemented/In Scope/Draft]

[Implementation details]

Key functions: `Function1`, `Function2`

***

## Design Contracts

- [Contract 1]
- [Contract 2]
- [Contract 3]
- [Contract 4]

***

## Q&A

### #1 — [Question]
**Q:** [Question text]
**A:** [Answer]

### #2 — [Question]
**Q:** [Question text]
**A:** [Answer]

---

## Resolved Questions — Addendum

### #N — [Question]
**Q:** [Question text]
**A:** [Answer]