# Plan: Full Go Rewrite of ClaudeCode — Modular Breakdown

## Context

ClaudeCode (`/workspaces/test-code/ClaudeCode`) is a ~2,000-file TypeScript/React-Ink CLI: terminal UI, Anthropic API streaming, tool execution, MCP, permission system, session storage, plugins, skills, and several SDK/entrypoint modes. A full Go rewrite is only tractable if it is split into **independently buildable modules behind stable interfaces**, composed top-down by a single binary.

This plan divides the project into 12 modules (M1–M12), each a Go package you can implement and test in isolation. Every module declares (a) what it owns, (b) the *Go interface* it exposes, (c) the modules it depends on. A final section shows how the `claude` binary wires them together.

Implement M1 → M12 in order; later modules stub earlier ones with fakes until ready.

---

## Target Repository Layout

```
go/
  cmd/
    claude/              // main binary (wiring only, ~200 LOC)
    claude-sdk/          // headless SDK binary
  pkg/
    claudetypes/         // M1  — shared types, no deps
    config/              // M2  — settings.json, env, paths
    llm/                 // M3  — Anthropic client abstraction
    tool/                // M4  — Tool interface + registry
    tools/               // M5  — concrete tool implementations
    permission/          // M6  — can-use-tool policy
    mcp/                 // M7  — MCP client + server plumbing
    session/             // M8  — transcript / file history / cache
    queryengine/         // M9  — turn loop (see prior plan)
    commands/            // M10 — slash commands
    tui/                 // M11 — Bubble Tea UI
    sdk/                 // M12 — headless SDK façade
  internal/
    testutil/            // shared fakes
```

Module boundary rule: **no import cycles**, and no module imports a module with a higher number.

---

## M1 — `claudetypes` (shared types)

**Owns:** Go equivalents of `src/types/message.ts`, `src/entrypoints/agentSdkTypes.ts`, `PermissionMode`, `ToolUseBlock` wrappers, `Usage`.

**Exposes:**
```go
type Message interface{ isMessage() }
type UserMessage struct{ ... }; type AssistantMessage struct{ ... }; ...
type SDKEvent interface{ Kind() string }
type PermissionMode string // "default"|"acceptEdits"|"plan"|"bypassPermissions"
type Usage struct{ Input, CacheRead, CacheCreate, Output int64 }
```

**Depends on:** nothing (other than stdlib + `anthropic-sdk-go` types).

**Why first:** every other module consumes these types. No logic, just data.

---

## M2 — `config` (settings, paths, env)

**Owns:** loading `~/.claude/settings.json`, project `.claude/settings.json`, env overrides, CWD resolution, feature flags (`feature('X')` equivalent → build tags + runtime flag map).

**Port of:** `src/utils/config.ts`, `src/utils/cwd.ts`, `src/bootstrap/state.ts`, `src/services/remoteManagedSettings`.

**Exposes:**
```go
type Loader interface {
  Global() Settings
  Project(cwd string) Settings
  Feature(name string) bool
}
```

**Depends on:** M1.

---

## M3 — `llm` (Anthropic client abstraction)

**Owns:** streaming chat completion, retry + fallback model, usage accounting, model selection.

**Port of:** `src/services/api/claude.ts`, `withRetry.ts`, `errors.ts`, `logging.ts`, `src/utils/model/*`.

**Exposes:**
```go
type Client interface {
  Stream(ctx, Request) (<-chan StreamEvent, error)
}
type Request struct{ Model string; System []Block; Messages []Message; Tools []ToolSpec; Thinking *ThinkingCfg; ... }
```

**Depends on:** M1, M2. Wraps `anthropic-sdk-go`. Bedrock/Vertex/Foundry are swappable `Client` implementations — add later.

---

## M4 — `tool` (tool interface + registry)

**Owns:** the contract every tool satisfies, the registry, JSON-schema validation of inputs, the `ToolUseContext` passed to each invocation.

**Port of:** `src/Tool.ts`, `src/tools/shared`, `src/tools/utils.ts`.

**Exposes:**
```go
type Tool interface {
  Name() string
  Schema() jsonschema.Schema
  Call(ctx context.Context, input json.RawMessage, tc ToolContext) (<-chan ToolEvent, error)
}
type Registry interface { Get(name string) (Tool, bool); All() []Tool }
```

**Depends on:** M1, M2.

**Key decision:** `Call` returns a channel so tools can stream progress (matches current AsyncGenerator tool model). Blocking tools just send one event + close.

---

## M5 — `tools/*` (concrete tool implementations)

**Owns:** one Go package per tool under `pkg/tools/`. Each is independently testable.

54 tools in TS; group for implementation in priority order:

- **Tier A (must-have for MVP):** `BashTool`, `FileReadTool`, `FileEditTool`, `FileWriteTool`, `GlobTool`, `GrepTool`, `TodoWriteTool`.
- **Tier B (common):** `WebFetchTool`, `WebSearchTool`, `NotebookEditTool`, `AgentTool`, `TaskCreate/Get/List/Update/Output/StopTool`.
- **Tier C (specialized):** `MCPTool`, `McpAuthTool`, `List/ReadMcpResourceTool`, `SkillTool`, `DiscoverSkillsTool`, `WorkflowTool`, `LSPTool`, `ScheduleCronTool`, worktree tools, plan-mode tools, `SendMessageTool`, `REPLTool`, `PowerShellTool`, `WebBrowserTool`, etc.
- **Tier D (defer/skip):** `TungstenTool`, `OverflowTestTool`, `SyntheticOutputTool`, `TerminalCaptureTool`, voice-related.

Each tool depends only on M1, M2, M4, plus its own domain libs (e.g. `FileEditTool` uses an in-process diff library). Register them at startup via M4.

**Depends on:** M1, M2, M4, (for `AgentTool`) M9, (for `MCPTool`) M7.

---

## M6 — `permission` (can-use-tool policy)

**Owns:** the decision engine that says whether a `tool_use` may run: plan-mode rules, allow/deny lists, scratchpad boundaries, worktree scope, elicitation to user.

**Port of:** `src/hooks/useCanUseTool.ts`, `src/utils/permissions/*`, `src/services/mcpServerApproval.tsx`, parts of `src/tools/*/permissions.ts`.

**Exposes:**
```go
type Decider interface {
  CanUse(ctx, toolName string, input json.RawMessage, mode PermissionMode) (Decision, error)
}
type Decision struct{ Allow bool; Reason string; AskUser *Prompt }
```

**Depends on:** M1, M2, M4. UI-level "ask the user" is delegated via a callback the TUI (M11) or SDK (M12) implements.

---

## M7 — `mcp` (Model Context Protocol)

**Owns:** MCP client (stdio + HTTP + SSE transports), server lifecycle, connection pool, tool/resource discovery. Also the built-in MCP servers in `packages/@ant/*`.

**Port of:** `src/services/mcp/*`, `src/entrypoints/mcp.ts`.

**Exposes:**
```go
type ServerConn interface { Name() string; ListTools(ctx) ([]ToolSpec, error); CallTool(...) (...); Close() error }
type Manager interface { Start(cfg []ServerCfg) ([]ServerConn, error) }
```

Use `github.com/mark3labs/mcp-go` or equivalent rather than hand-rolling.

**Depends on:** M1, M2. MCP tools register into M4 at startup.

---

## M8 — `session` (transcript, file cache, history)

**Owns:** on-disk session format, `readFileState` cache, file-history snapshots, transcript replay.

**Port of:** `src/utils/sessionStorage.ts`, `src/utils/fileStateCache.ts`, `src/utils/fileHistory.ts`, `src/history.ts`.

**Exposes:**
```go
type Store interface {
  Load(sessionID string) ([]Message, error)
  Append(sessionID string, msgs []Message) error
  Snapshot(path string) error
}
```

**Compatibility:** write the same JSONL shape as TS so sessions started in either binary are interoperable.

**Depends on:** M1, M2.

---

## M9 — `queryengine` (turn loop)

**Owns:** the main agent loop — wraps M3 with tool-use dispatch (M4/M5), permission checks (M6), session I/O (M8), usage accounting.

**Port of:** `src/QueryEngine.ts`, `src/query.ts`, `src/query/*`, `src/services/compact/*`.

**Exposes:**
```go
type Engine interface {
  SubmitMessage(ctx, prompt any) <-chan SDKEvent
}
func New(cfg Config) Engine
func Ask(ctx, cfg Config, prompt any) <-chan SDKEvent
```

**Depends on:** M1, M2, M3, M4, M6, M8. (Detailed plan in earlier `write-a-plan-to-delightful-dusk.md`.)

---

## M10 — `commands` (slash commands)

**Owns:** the `/help`, `/clear`, `/compact`, `/review`, `/init`, skill dispatchers, custom project commands, etc.

**Port of:** `src/commands/*`, `src/commands.ts`.

**Exposes:**
```go
type Command interface {
  Name() string
  Run(ctx, args string, host Host) error
}
type Registry interface { Resolve(input string) (Command, []string, bool) }
```

`Host` gives commands access to M9 (engine), M4 (tools), M8 (session). Slash commands implemented one-by-one; prioritize `/clear`, `/compact`, `/help`, `/init`, `/review`.

**Depends on:** M1, M2, M4, M8, M9.

---

## M11 — `tui` (Bubble Tea UI)

**Owns:** every interactive screen — main REPL, input editor, scrollback, permission prompts, dialog launchers, vim mode, keybindings.

**Port of:** `src/main.tsx`, `src/ink/*`, `src/components/*`, `src/screens/*`, `src/vim/*`, `src/keybindings/*`, `src/interactiveHelpers.tsx`.

**Framework:** `github.com/charmbracelet/bubbletea` + `lipgloss` + `bubbles`. Component model aligns well with Ink.

**Exposes:**
```go
func Run(ctx, deps Deps) error  // blocks until user exits
```

`Deps` bundles M9 engine, M6 permission UI callback, M10 commands, M2 config.

**Depends on:** M1, M2, M4, M6, M9, M10. This is the single largest module (~400 components in TS); break internally by screen and ship incrementally. A `--headless` flag on the binary skips M11 entirely.

---

## M12 — `sdk` (headless SDK façade)

**Owns:** the non-interactive entrypoint used by `claude -p "..."` and the Agent SDK — stream-JSON output, single-shot mode, resumable sessions.

**Port of:** `src/entrypoints/sdk/*`, `src/entrypoints/agentSdkTypes.ts`.

**Exposes:** a stable wire protocol (JSONL of `SDKEvent`) and a Go library API mirroring the TS SDK.

**Depends on:** M1, M2, M3, M4, M6, M8, M9, M10.

---

## Composition — the `claude` binary

`cmd/claude/main.go` is wiring only; each layer receives already-constructed dependencies:

```go
func main() {
  ctx := signalCtx()
  cfg := config.MustLoad()                        // M2
  llmC := llm.NewAnthropic(cfg)                   // M3
  reg := tool.NewRegistry()                       // M4
  tools.RegisterBuiltins(reg, cfg)                // M5
  mcpMgr, _ := mcp.Start(ctx, cfg.MCPServers())   // M7 → registers more into reg
  perm := permission.New(cfg, reg)                // M6
  store := session.NewFileStore(cfg.SessionDir()) // M8
  eng := queryengine.New(queryengine.Config{      // M9
    LLM: llmC, Tools: reg, Perm: perm, Store: store, Cfg: cfg,
  })
  cmds := commands.NewRegistry(eng, reg, store)   // M10

  if cfg.Headless() {
    sdk.Run(ctx, eng, cmds, os.Stdin, os.Stdout)  // M12
    return
  }
  tui.Run(ctx, tui.Deps{Engine: eng, Perm: perm, Cmds: cmds, Cfg: cfg}) // M11
}
```

Every arrow above points **downward** in the module number list — no cycles, no hidden globals.

---

## Cross-Cutting Concerns

These are not modules but conventions every module follows:

- **Logging/telemetry:** one `log/slog` logger constructed in `main`, threaded via `context.Value`. OTel exporters (port of `src/services/analytics`) plug in behind an interface so they're optional.
- **Abort:** `context.Context` replaces `AbortController` everywhere. No goroutine may outlive its context.
- **Errors:** typed error values for retry/fallback (M3), permission-denied (M6), tool-failure (M5). No string matching.
- **Feature flags:** one package `pkg/feat` with `feat.Enabled("REACTIVE_COMPACT")` — build tags at compile time + settings.json overrides at runtime.

---

## Suggested Implementation Order (stand-alone milestones)

1. **M1 + M2** — types and config. Compilable in a day.
2. **M3 + M4** — LLM client and tool registry; build a 30-line `cmd/ask` that streams a completion with no tools. Proves the wire works.
3. **M5 Tier A** — Bash, FileRead, FileEdit, FileWrite, Glob, Grep, TodoWrite. Enough to be a usable agent.
4. **M6 + M9** — permission + query engine. Now `cmd/ask` can do real tool-use loops.
5. **M8** — session persistence; sessions survive restart.
6. **M7 + M5 Tier B/C** — MCP and more tools.
7. **M10** — minimal command set (`/clear`, `/compact`, `/help`).
8. **M12** — headless SDK binary (ship this before the TUI — most users of the SDK path don't need the TUI).
9. **M11** — TUI, built screen-by-screen; until it's done, users run `claude -p` (headless).

Each milestone produces a shippable binary.

---

## Verification Strategy

- **Per-module:** Go unit tests + fakes in `internal/testutil`. Target ≥80% on M3/M4/M6/M9 where correctness matters most.
- **Contract tests:** for M3 (stream event shape), M4 (tool I/O schema), M9 (SDK event stream) record golden JSONL from the TS CLI and assert byte-equality from Go.
- **End-to-end:** a `testdata/` suite of prompts + expected tool-call traces, run against both TS and Go binaries with the same model and seed.
- **Interop:** a session started in TS must be readable by Go (M8) and vice-versa.

---

## Critical Files to Reference During Port

- `src/QueryEngine.ts` (M9 anchor)
- `src/query.ts` (M9 inner loop)
- `src/Tool.ts`, `src/tools/shared/*` (M4 contract)
- `src/services/api/claude.ts`, `withRetry.ts` (M3)
- `src/hooks/useCanUseTool.ts`, `src/utils/permissions/*` (M6)
- `src/services/mcp/*` (M7)
- `src/utils/sessionStorage.ts`, `fileStateCache.ts`, `fileHistory.ts` (M8)
- `src/commands.ts`, `src/commands/*` (M10)
- `src/main.tsx`, `src/ink/*`, `src/components/*` (M11)
- `src/entrypoints/sdk/*`, `agentSdkTypes.ts` (M12)
- `src/utils/config.ts`, `src/bootstrap/state.ts` (M2)
- `src/types/message.ts`, `src/entrypoints/agentSdkTypes.ts` (M1)

---

## Open Questions

1. Is the goal to **replace** the TS binary (strict behavioral parity) or to **coexist** (Go for headless SDK only, TS keeps the TUI)? Answer changes whether M11 is mandatory.
2. Which LLM backends must ship in v1 — public Anthropic only, or Bedrock/Vertex/Foundry too?
3. Do we keep the existing MCP packages under `packages/@ant/*` running as Node subprocesses (they're MCP servers, language-agnostic) or rewrite them too? Recommend keeping them — MCP is a protocol boundary.
4. Plugin system (`src/plugins`, `src/utils/plugins`): v1 ship without, or port? Plugins currently execute TS; a Go host can't run them directly.



