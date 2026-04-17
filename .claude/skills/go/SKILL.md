---
name: go
description: >
  This skill should be used when the user asks to "write Go code", "modify Go files",
  "create a Go file", "update go.mod", "run go fix", "update Go version", "go build",
  "go test", or when working with any `.go`, `go.mod`, or `go.sum` files in this project.
  Also triggers when the user mentions "Go module", "Go package", "Go interface",
  "go vet", "golang", or any Go-specific development task.
version: 0.2.0
---

# Go Coding Skill

A project-level skill for Go development that enforces version hygiene, automates
repetitive tasks, and ensures consistent coding practices.

## On Skill Trigger

When this skill activates, execute these setup steps before doing anything else:

### 1. Check Go Version

Run the version check script to detect any Go installation updates:

```bash
bash .claude/skills/go/scripts/check-go-version.sh
```

If the output shows "UPDATE AVAILABLE", update `references/go-version.md` with the
new version and commit the change. Then update any `go.mod` files in the project
to use the new version.

### 2. Verify go.mod Version

If a `go.mod` file exists, check its `go` directive. Compare against the recorded
version in `references/go-version.md`. If the go.mod version is older, update it:

```bash
go mod edit -go=<latest_version>
```

Do not add `toolchain` directives or compatibility constraints for older Go versions.

## Core Rules

### Rule 1: Always Use Latest Go Version

This project does not maintain compatibility with older Go versions. Always use the
latest stable Go release. The `go` directive in `go.mod` must match the installed
Go version exactly.

Never add `//go:build` constraints for version gating, never use deprecated APIs
for backward compatibility, and never add `toolchain` directives pointing to older
versions.

### Rule 2: Run go fix After Every Edit

After modifying any `.go` file, immediately run:

```bash
go fix ./...
```

This applies the latest Go static analysis fixes automatically. Do not skip this
step, even for small changes.

### Rule 3: Commit After Complex Changes

After completing a non-trivial component — such as a new module, interface
definition, multi-file refactor, or a working test suite — create a git commit
before moving to the next task.

A change is "complex" if it spans multiple files, introduces a new package,
defines a new interface, or implements a significant feature. Simple edits
(typo fixes, import reordering, single-line changes) do not need individual
commits.

### Rule 4: Track Go Version

The file `references/go-version.md` records the current known Go version.
Always consult this file at the start of Go work and update it when a newer
Go version is detected. This ensures version drift is caught early.

## Go 1.26 Language Features

Use these features when writing new code:

### new() with expressions

`new()` now accepts an expression as its operand:

```go
// Before: two lines
x := int64(300)
ptr := &x

// After: one line
ptr := new(int64(300))
```

Use this for optional fields with pointer types:

```go
type Person struct {
    Name string   `json:"name"`
    Age  *int     `json:"age"`
}

p := Person{Name: "Alice", Age: new(yearsSince(born))}
```

### Self-referencing generic types

Generic types can now reference themselves in their type parameter list:

```go
type Adder[A Adder[A]] interface {
    Add(A) A
}
```

### New standard library utilities

- `errors.AsType[T]()` — type-safe generic version of `errors.As`
- `slog.NewMultiHandler()` — log to multiple handlers at once
- `bytes.Buffer.Peek()` — peek at buffer contents without advancing
- `reflect` iterators — `Type.Fields()`, `Type.Methods()`, `Value.Fields()`
- `testing.T.ArtifactDir()` — write test output files to a managed directory

## Development Workflow

### Starting a New Go Package

1. Create the package directory under the appropriate module path
2. Write interfaces before implementations
3. Write tests alongside the code (same package)
4. Run `go fix ./...` after all files are created
5. Run `go vet ./...` to catch issues
6. Commit when the package compiles and tests pass

### Modifying Existing Go Code

1. Read the current file to understand context
2. Make the targeted change
3. Run `go fix ./...`
4. Run `go vet ./...`
5. If the change is complex, commit

### Adding Dependencies

1. Use `go get <package>` to add dependencies
2. Prefer standard library over third-party packages
3. Run `go mod tidy` after changes
4. Commit the updated `go.mod` and `go.sum`

## Error Handling

Use the patterns documented in `references/conventions.md`. In summary:

- Return `error` as the last return value
- Wrap errors with context using `fmt.Errorf("operation: %w", err)`
- Check errors immediately after the call that produced them
- Use typed errors for recoverable conditions

## Testing

- Tests live in the same package as the code they test
- Use `testing` from the standard library
- Table-driven tests for parameterized cases
- Test files named `<source>_test.go`
- Run `go test ./...` before every commit

## Additional Resources

### Reference Files

Consult these for detailed guidance:

- **`references/go-version.md`** — Current Go version record
- **`references/conventions.md`** — Coding conventions and patterns for this project

### Scripts

- **`scripts/check-go-version.sh`** — Detects Go version updates
