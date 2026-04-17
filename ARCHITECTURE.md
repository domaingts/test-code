# Claude Code Architecture Documentation

> Version 2.1.88 — Recovered source tree rebuilt from the published npm package's sourcemap.

## Overview

Claude Code is a **TypeScript/React-Ink CLI application** that provides an agentic AI coding assistant in the terminal. It uses the Anthropic API for LLM inference, the Model Context Protocol (MCP) for extensible tool integration, and React-Ink for its terminal UI.

**Tech stack:** TypeScript, React 19 (via Ink/react-reconciler), Bun (build), Zod (validation), Commander (CLI args), Chalk (terminal colors).

---

## Project Structure

```
ClaudeCode/
├── src/                    # Main source (1902 files + 142 stubs)
│   ├── entrypoints/        # CLI, MCP server, SDK entry points
│   ├── services/           # Core business logic services
│   ├── tools/              # 50+ tools (file ops, shell, web, agents)
│   ├── commands/           # 80+ slash commands (/help, /compact, etc.)
│   ├── components/         # React-Ink UI components
│   ├── hooks/              # React hooks for permissions, tools, UI
│   ├── utils/              # Shared utilities
│   ├── state/              # App state management
│   ├── tasks/              # Background task execution
│   ├── ink/                # Custom Ink runtime extensions
│   └── ...                 # Many more subdirectories
├── packages/               # Local packages
│   ├── @ant/               # Anthropic internal packages
│   ├── @anthropic-ai/      # Claude Agent SDK
│   ├── audio-capture/      # Native audio module
│   ├── color-diff-napi/    # Syntax-highlighted diffs
│   ├── modifiers-napi/     # Keyboard modifier state
│   ├── image-processor/    # Image processing (sharp-compatible)
│   ├── ripgrep/            # Bundled rg binary (6 platforms)
│   └── url-handler/        # macOS URL scheme handler
├── scripts/                # Build and codegen scripts
├── patches/                # Commander patch for loose flag parsing
└── skills/                 # Bundled skills (claude-api, verify)
```

---

## Entry Points (`src/entrypoints/`)

| File | Purpose |
|------|---------|
| `cli.tsx` | Bootstrap entrypoint. Fast-paths for `--version`, `--dump-system-prompt`, then loads full CLI. Also handles `--daemon`, `--bridge`, `--bg` modes. |
| `init.ts` | One-time initialization: telemetry, OpenTelemetry, OAuth, config, policy limits, LSP, shutdown handlers. |
| `mcp.ts` | Starts Claude Code as an MCP server over stdio, exposing all tools to external MCP clients. |
| `agentSdkTypes.ts` | Re-exports public Agent SDK types for third-party builders. |
| `sdk/` | Full SDK type definitions (control protocol, schemas, runtime, settings, tools). |

---

## Core Services (`src/services/`)

### API Layer (`services/api/`)

| File | Purpose |
|------|---------|
| `client.ts` | Creates Anthropic API clients for multiple providers: direct API key, AWS Bedrock, Azure Foundry, Google Vertex, OAuth. Handles credential refresh and proxy config. |
| `claude.ts` | Core query function — sends messages to the API with streaming, system prompts, tools, cache control, and usage tracking. |
| `promptCacheBreakDetection.ts` | Detects prompt cache invalidation (system prompt changes, tool schema changes, beta header changes). |

### Tool Execution (`services/tools/`)

| File | Purpose |
|------|---------|
| `toolExecution.ts` | Core tool execution engine: validates permissions, runs tools, handles results and progress. |
| `toolOrchestration.ts` | Orchestrates multiple tool calls per assistant turn. Partitions into parallel (read-only) and sequential batches. |
| `StreamingToolExecutor.ts` | Executes tools as they stream from the API response with concurrency control. |
| `toolHooks.ts` | Runs pre/post-tool hooks from `.claude/hooks/`, processing permission decisions and output modifications. |

### MCP Integration (`services/mcp/`)

Manages MCP client connections to external tool servers. Handles stdio, SSE, and HTTP transports. Discovers remote MCP tools and wraps them as local `Tool` objects. Manages OAuth authentication for remote servers.

### OAuth (`services/oauth/`)

Implements OAuth 2.0 authorization code flow with PKCE. Supports browser-based auth (localhost callback) and manual code entry for headless environments.

### Context Management

| Service | Purpose |
|---------|---------|
| `compact/` | Conversation compaction to stay within context window. Full summarization (`compact.ts`), auto-compaction (`autoCompact.ts`), and targeted micro-compaction (`microCompact.ts`). |
| `contextCollapse/` | Lighter alternative to full compaction — collapses redundant/low-value context blocks. |

### Memory and Learning

| Service | Purpose |
|---------|---------|
| `extractMemories/` | Extracts durable memories from the session transcript using a forked subagent. Writes to `~/.claude/projects/<path>/memory/`. |
| `autoDream/` | Background memory consolidation ("dreaming"). Time-gated (24h) and session-gated (5 minimum) forked subagent reviews and synthesizes memories. |
| `teamMemorySync/` | Syncs team memory files between local filesystem and server API, scoped per Git repo. Supports pull/push with secret scanning. |
| `MagicDocs/` | Auto-maintains markdown docs marked with `# MAGIC DOC: [title]` headers by running background forked subagents. |

### Enterprise and Policy

| Service | Purpose |
|---------|---------|
| `policyLimits/` | Fetches org-level policy restrictions. ETag caching, hourly polling, fail-open behavior. |
| `remoteManagedSettings/` | Fetches/caches remote-managed settings for enterprise. Checksum validation, security checks, graceful degradation. |
| `settingsSync/` | Syncs user settings and memory across environments. Incremental uploads, ETag caching. |

### Other Services

| Service | Purpose |
|---------|---------|
| `analytics/` | Event logging to Datadog and first-party systems. PII-aware metadata handling, sampling, killswitch. |
| `AgentSummary/` | Periodic 30s background summarization for sub-agents (e.g., "Reading runAgent.ts"). |
| `PromptSuggestion/` | Generates next-prompt suggestions. Includes speculative execution (pre-computes results). |
| `toolUseSummary/` | Generates short summaries of completed tool batches using Haiku model. |
| `skillSearch/` | Local indexing and searching of available skills/commands. |
| `tips/` | Contextual tip display during loading spinners with cooldown rotation. |
| `plugins/` | Background plugin and marketplace installation from trusted sources. |
| `lsp/` | LSP server manager — initializes, routes requests, provides code intelligence (go-to-def, diagnostics). |
| `sessionTranscript/` | Session recording and retrieval for replay/debugging. |

---

## Tools (`src/tools/`) — 50+ Tools

### File Operations

| Tool | Description |
|------|-------------|
| **FileReadTool** | Reads file contents (images, PDFs, code) with line numbers and token estimation. |
| **FileEditTool** | Targeted search-and-replace edits with diff tracking and git diff integration. |
| **FileWriteTool** | Creates or overwrites files with permissions, LSP notifications, and history tracking. |
| **GlobTool** | Fast file pattern matching using glob patterns. |
| **GrepTool** | Regex content search using ripgrep with output modes and glob filtering. |
| **NotebookEditTool** | Edits Jupyter notebook cells (add/replace/delete). |

### Shell Execution

| Tool | Description |
|------|-------------|
| **BashTool** | Executes bash commands with security validation, destructive command warnings, and sandbox support. |
| **PowerShellTool** | Windows equivalent — PowerShell commands with platform-specific security. |

### Agent and Delegation

| Tool | Description |
|------|-------------|
| **AgentTool** | Spawns sub-agents (local, remote, forked) with built-in types: explore, plan, verification, general-purpose. |
| **SendMessageTool** | Sends structured messages between agents in a swarm/team. |

### User Interaction

| Tool | Description |
|------|-------------|
| **AskUserQuestionTool** | Multiple-choice questions with optional previews and multi-select. |
| **BriefTool** | Sends messages with optional file attachments and proactive status updates. |

### Web and Network

| Tool | Description |
|------|-------------|
| **WebFetchTool** | Fetches URLs and converts to markdown with size limits. |
| **WebSearchTool** | Web search returning structured results with titles and URLs. |

### MCP Integration

| Tool | Description |
|------|-------------|
| **MCPTool** | Generic proxy for invoking MCP server tools with dynamic schemas. |
| **ListMcpResourcesTool** | Lists available resources from connected MCP servers. |
| **ReadMcpResourceTool** | Reads specific resources from MCP servers by URI. |
| **McpAuthTool** | Handles OAuth flows for MCP servers requiring authorization. |

### Code Intelligence

| Tool | Description |
|------|-------------|
| **LSPTool** | LSP-based operations: go-to-definition, find-references, hover, symbols, call hierarchy. |

### Task and Team Management

| Tool | Description |
|------|-------------|
| **TaskCreateTool** | Creates tasks with subject, description, and dependencies. |
| **TaskGetTool** | Retrieves a task by ID with full details. |
| **TaskListTool** | Lists all tasks with status and dependency info. |
| **TaskOutputTool** | Gets output from completed/running tasks with blocking support. |
| **TaskStopTool** | Stops/kills a running background task. |
| **TaskUpdateTool** | Updates task status, description, and dependencies. |
| **TeamCreateTool** | Creates a swarm team with name, description, and team lead. |
| **TeamDeleteTool** | Disbands a team, cleaning up directories and state. |
| **TodoWriteTool** | Manages the session's todo list. |

### Mode and Configuration

| Tool | Description |
|------|-------------|
| **ConfigTool** | Gets/sets Claude Code settings (theme, model, permissions). |
| **EnterPlanModeTool** | Transitions into plan mode for approach design before execution. |
| **ExitPlanModeTool** | Exits plan mode and returns to normal execution. |
| **EnterWorktreeTool** | Creates an isolated git worktree for safe parallel development. |
| **ExitWorktreeTool** | Exits a worktree, optionally keeping or removing it. |

### Scheduling and Automation

| Tool | Description |
|------|-------------|
| **ScheduleCronTool** | Schedules recurring/one-shot cron tasks with durable persistence. |
| **RemoteTriggerTool** | Manages remote agent triggers via API (list, create, run). |

### Other Tools

| Tool | Description |
|------|-------------|
| **SkillTool** | Invokes slash commands/skills by name. |
| **ToolSearchTool** | Searches deferred tool names by keyword for dynamic discovery. |
| **SyntheticOutputTool** | Returns structured JSON output for non-interactive sessions. |

---

## Commands (`src/commands/`) — 80+ Slash Commands

Commands follow three types: `local` (async function), `local-jsx` (React component), `prompt` (text sent to model).

### Categories

| Category | Commands |
|----------|----------|
| **Git/VC** | `commit`, `branch`, `diff`, `fork`, `resume`, `rewind`, `teleport` |
| **Review** | `review`, `security-review`, `bughunter`, `ant-trace`, `perf-issue` |
| **Config** | `config`, `permissions`, `model`, `effort`, `env`, `output-style`, `color`, `theme`, `keybindings`, `vim`, `sandbox-toggle`, `privacy-settings` |
| **Session** | `compact`, `resume`, `session`, `share`, `clear`, `exit` |
| **MCP/Plugins** | `mcp`, `plugin`, `skills`, `hooks`, `passes`, `install-github-app`, `install-slack-app` |
| **Planning** | `plan`, `memory`, `ctx_viz`, `thinkback`, `thinkback-play` |
| **Agents** | `agents`, `tasks`, `peers`, `buddy`, `workflows` |
| **Onboarding** | `help`, `init`, `onboarding`, `status`, `doctor`, `upgrade`, `feedback` |
| **Web/Remote** | `bridge`, `chrome`, `desktop`, `mobile`, `remote-env`, `remote-setup` |
| **Info/Debug** | `cost`, `usage`, `stats`, `debug-tool-call`, `heapdump`, `fast` |
| **Utility** | `add-dir`, `copy`, `export`, `rename`, `files`, `context`, `login`, `logout`, `voice`, `tag`, `summary`, `issue` |

---

## Components (`src/components/`) — UI Layer

React-Ink terminal UI components organized by feature:

| Directory | Purpose |
|-----------|---------|
| `design-system/` | Base design tokens and primitives. |
| `messages/` | Renders assistant/user/system messages. |
| `permissions/` | Permission request dialogs (Bash, FileEdit, FileWrite, etc.) — one per tool type. |
| `diff/` | Syntax-highlighted diff rendering. |
| `PromptInput/` | The main user input field with autocomplete. |
| `Settings/` | Settings configuration panel. |
| `mcp/` | MCP server management UI. |
| `memory/` | Memory browser and editor. |
| `skills/` | Skill browser and invocation UI. |
| `tasks/` | Task list display and management. |
| `agents/` | Agent creation wizard and management. |
| `teams/` | Team/swarm management. |
| `hooks/` | Hook configuration UI. |
| `sandbox/` | Sandbox configuration. |
| `shell/` | Shell output rendering. |
| `wizard/` | Onboarding/setup wizards. |
| `HelpV2/` | Help and command listing. |
| `StructuredDiff/` | Structured diff display with word-level highlights. |
| `HighlightedCode/` | Syntax-highlighted code blocks. |
| `Spinner/` | Loading spinner with tip display. |
| `FeedbackSurvey/` | In-session feedback collection. |
| `LogoV2/` | Claude branding. |
| `TrustDialog/` | Trust confirmation dialogs. |
| `Passes/` | Usage passes management. |

---

## Other `src/` Directories

| Directory | Purpose |
|-----------|---------|
| `assistant/` | Session discovery, history, and session chooser for multi-session management. |
| `bridge/` | Remote control / bridge mode — serve local machine as remote environment. WebSocket communication. |
| `buddy/` | AI coding buddy feature (stub in this build). |
| `cli/` | CLI argument parsing, transport layers, background session handlers (`ps`, `logs`, `attach`, `kill`). |
| `coordinator/` | Coordinator mode for orchestrating multiple sub-agents in parallel. |
| `daemon/` | Long-running daemon process with worker registry for background operations. |
| `environment-runner/` | Headless BYOC (Bring Your Own Cloud) environment runner. |
| `hooks/` | React hooks for tool permissions, notifications, UI state. |
| `ink/` | Custom Ink runtime: terminal I/O, layout engine, event system, yoga layout bindings. |
| `jobs/` | Background job scheduling and management. |
| `keybindings/` | Keyboard shortcut system. |
| `memdir/` | Memory directory management — auto-memory paths, team memory paths. |
| `migrations/` | Data migration scripts (e.g., migrating model aliases). |
| `moreright/` | Extended rendering utilities. |
| `native-ts/` | Pure TypeScript ports of Rust NAPI modules: `color-diff`, `file-index` (fuzzy search), `yoga-layout`. |
| `outputStyles/` | Loadable output style system — markdown files from `.claude/output-styles/` become style prompts. |
| `plugins/` | Plugin system with bundled plugins. |
| `proactive/` | Proactive features (stub in this build). |
| `query/` | Query engine — the core `query()` function that drives the main conversation loop. |
| `remote/` | Remote session management via WebSocket. |
| `schemas/` | JSON Schema definitions. |
| `screens/` | Full-screen views (transcript viewer, session browser). |
| `self-hosted-runner/` | Self-hosted runner for enterprise deployments. |
| `server/` | Local server for direct-connect sessions (IDE integration). |
| `ssh/` | SSH session management (stub in this build). |
| `state/` | Centralized `AppState` type and store creation. |
| `tasks/` | Task execution backends: `LocalShellTask`, `LocalAgentTask`, `RemoteAgentTask`, `InProcessTeammateTask`, `LocalWorkflowTask`, `MonitorMcpTask`, `DreamTask`. |
| `vim/` | Vim motion support for the prompt input. |
| `voice/` | Voice/audio mode integration with native audio capture. |

---

## Utilities (`src/utils/`)

| Directory | Purpose |
|-----------|---------|
| `bash/` | Shell command specs, parsing, and security validation. |
| `claudeInChrome/` | Chrome extension integration — MCP server and native messaging host. |
| `computerUse/` | Computer use (desktop automation) utilities and MCP server. |
| `deepLink/` | Deep link handling for `claude://` URL schemes. |
| `dxt/` | DXT package format utilities. |
| `filePersistence/` | File-based persistence helpers. |
| `git/` | Git operations (status, diff, remotes, branch management). |
| `github/` | GitHub CLI integration (PR creation, issue management). |
| `hooks/` | Hook execution infrastructure for `.claude/hooks/`. |
| `mcp/` | MCP utility functions. |
| `memory/` | Memory file operations and path resolution. |
| `messages/` | Message creation and manipulation utilities. |
| `model/` | Model selection, provider detection, and cost calculation. |
| `nativeInstaller/` | Native binary installation helpers. |
| `permissions/` | Permission system — yolo classifier, rule engine, approval flows. |
| `plugins/` | Plugin loading, marketplace resolution, output style loading. |
| `powershell/` | PowerShell command specs and validation. |
| `processUserInput/` | User input parsing, mention resolution, slash command dispatch. |
| `sandbox/` | Sandbox runtime integration for isolated command execution. |
| `secureStorage/` | Secure credential storage. |
| `settings/` | Settings management — JSON schema, change detection, MDM support. |
| `shell/` | Shell detection and configuration. |
| `skills/` | Skill loading and management. |
| `suggestions/` | Autocomplete suggestions for the prompt input. |
| `swarm/` | Agent swarm coordination with multiple backends. |
| `task/` | Task disk output and management utilities. |
| `telemetry/` | OpenTelemetry integration (traces, metrics, logs). |
| `teleport/` | Teleport/remote session API communication. |
| `todo/` | Todo list management. |
| `ultraplan/` | Advanced planning utilities. |

---

## Packages (`packages/`)

### Anthropic Internal (`@ant/`)

| Package | Purpose |
|---------|---------|
| `claude-for-chrome-mcp` | MCP server bridging Claude to Chrome browser. Exposes browser automation tools (navigate, screenshot, JavaScript execution, form fill). Supports Unix socket and WebSocket transports. |
| `computer-use-mcp` | MCP server for native desktop screen control. Tools: screenshot, click, type, key, scroll, drag. Sophisticated per-app permission tiers, key-combo blocking, and clipboard guards. |
| `computer-use-input` | macOS native module for low-level keyboard/mouse input simulation. |
| `computer-use-swift` | macOS native module for screen capture and app listing using Swift. |

### Agent SDK (`@anthropic-ai/`)

| Package | Purpose |
|---------|---------|
| `claude-agent-sdk` | Official TypeScript SDK for building AI agents with Claude Code. Provides programmatic APIs for agents, tools, MCP servers, and multi-turn workflows. Version 0.2.88. |

### Native Modules

| Package | Purpose |
|---------|---------|
| `audio-capture` | Native audio recording/playback across 6 platforms. Supports microphone auth (TCC on macOS). |
| `color-diff-napi` | Pure TypeScript port of syntax-highlighted diff renderer. Truecolor, 256-color, and ANSI modes. Uses highlight.js for syntax. |
| `modifiers-napi` | macOS-only native module reading keyboard modifier keys (Shift, Ctrl, Option, Cmd). |
| `image-processor` | Sharp-compatible image processing API backed by custom native module. Resize, JPEG/PNG/WebP conversion. |
| `ripgrep` | Bundled ripgrep (`rg`) binary for 6 platforms (arm64/x64 × darwin/linux/win32). |
| `url-handler` | macOS-only native module for `claude://` URL scheme event handling. |

---

## Key Architectural Patterns

1. **Feature Flags** — `bun:bundle` `feature()` calls enable build-time dead code elimination for enterprise-only features.
2. **Forked Subagents** — `runForkedAgent()` creates lightweight sub-agents for summarization, memory extraction, and speculative execution.
3. **Tool Hooks** — Pre/post-execution hooks in `.claude/hooks/` allow user customization of tool behavior and permissions.
4. **MCP Protocol** — External tool servers connect via Model Context Protocol with stdio/SSE/HTTP transports and OAuth auth.
5. **React-Ink TUI** — Terminal UI built on React with custom Ink extensions for terminal I/O, yoga layout, and vim motions.
6. **Multi-Provider API** — Supports Anthropic direct, AWS Bedrock, Azure Foundry, and Google Vertex AI through a unified client layer.
7. **Background Tasks** — 7 task types (`LocalShell`, `LocalAgent`, `RemoteAgent`, `InProcessTeammate`, `LocalWorkflow`, `MonitorMcp`, `Dream`) with abort control and output streaming.
8. **Context Management** — Three-tier compaction: full summarization, auto-compaction, and micro-compaction of old tool results.
9. **Enterprise** — Policy limits, remote managed settings, settings sync, and team memory sync with ETag caching and fail-open behavior.
