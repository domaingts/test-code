# Go Coding Conventions

Conventions for Go development in this project. These apply to all Go code.

## Module Layout

Follow the standard Go project layout:

```
go/
  cmd/
    <binary-name>/
      main.go              // Entry point, wiring only
  pkg/
    <package>/             // Public packages
  internal/
    <package>/             // Private packages
```

- `cmd/` — Binary entrypoints. Keep `main.go` thin; wire dependencies and call into `pkg/`.
- `pkg/` — Packages other projects may import. Stable API.
- `internal/` — Packages only this module may import. Free to change.

## Interface Design

Define interfaces at the consumer, not the provider. Small interfaces
(fewer than 3 methods) are preferred.

```go
// Good: defined where it's consumed
type Store interface {
    Load(id string) (Record, error)
    Save(id string, r Record) error
}

// Avoid: large "god" interfaces
type Service interface {
    Load(...) // 15 methods
}
```

Return concrete types from constructors. Accept interfaces in functions
that need abstraction.

## Error Handling

Always return `error` as the last return value:

```go
func Parse(b []byte) (*Config, error)
```

Wrap errors to add context:

```go
if err := f.Close(); err != nil {
    return fmt.Errorf("close config file: %w", err)
}
```

Define typed errors for conditions callers need to distinguish:

```go
var ErrNotFound = errors.New("not found")
```

Use `errors.AsType` (Go 1.26+) for type-safe error matching instead of `errors.As`:

```go
// Before
var timeout *net.OpError
if errors.As(err, &timeout) { ... }

// After (preferred)
if timeout, ok := errors.AsType[*net.OpError](err); ok { ... }
```

Check errors immediately. Do not defer error checks.

## Naming

- Package names: short, lowercase, single word (`config`, `llm`, `tool`)
- Exported: `CamelCase`, unexported: `camelCase`
- Interfaces: noun or `-er` suffix (`Reader`, `Store`, `Decider`)
- Avoid stutter: `config.Config` → `config.Settings`

## Testing

- Test files: `<source>_test.go`, same package
- Test functions: `Test<Type>_<Method>_<scenario>`
- Table-driven tests for parameterized cases
- Use `testing` stdlib only; no assertion libraries

```go
func TestConfig_Load_missing(t *testing.T) {
    cases := []struct {
        name    string
        path    string
        wantErr bool
    }{
        {"empty path", "", true},
        {"nonexistent", "/tmp/no-such-file", true},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            _, err := Load(tc.path)
            if (err != nil) != tc.wantErr {
                t.Errorf("Load(%q) error = %v, wantErr %v", tc.path, err, tc.wantErr)
            }
        })
    }
}
```

## Logging

Use `log/slog` from the standard library. Thread the logger via
`context.Value` — never use a global logger.

```go
slog.InfoContext(ctx, "starting server", "addr", addr)
```

For multi-handler logging (Go 1.26+):

```go
h := slog.NewMultiHandler(consoleHandler, fileHandler)
logger := slog.New(h)
```

## Concurrency

- Use `context.Context` for cancellation and timeouts
- Use `errgroup` for fan-out/fan-in patterns
- Never leak goroutines — always ensure they can exit when context is cancelled
- Use channels for streaming data, not shared memory

## Imports

Group imports in three blocks separated by blank lines:

```go
import (
    "context"
    "fmt"
    "os"

    "github.com/anthropics/anthropic-sdk-go"

    "github.com/example/project/internal/config"
)
```

1. Standard library
2. Third-party packages
3. Internal packages
