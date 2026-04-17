# Claude Code — File Path Reference

## Entry & Bootstrap

| Path | Description |
|------|-------------|
| `ClaudeCode/src/entrypoints/cli.tsx` | CLI entry point — fast paths, then loads full app |
| `ClaudeCode/src/entrypoints/init.ts` | One-time setup: telemetry, OAuth, config, LSP |
| `ClaudeCode/src/main.tsx` | Main React-Ink app mounting |

## Core Engine

| Path | Description |
|------|-------------|
| `ClaudeCode/src/QueryEngine.ts` | Main conversation loop — sends to API, processes tool calls |
| `ClaudeCode/src/Tool.ts` | Tool interface, types, tool matching |
| `ClaudeCode/src/Task.ts` | Task types (local_bash, local_agent, remote_agent, dream, etc.) |
| `ClaudeCode/src/state/AppState.tsx` | Global React state provider |

## API & Auth

| Path | Description |
|------|-------------|
| `ClaudeCode/src/services/api/claude.ts` | Core `query()` function — streaming Anthropic API calls |
| `ClaudeCode/src/services/api/client.ts` | Multi-provider API client (Anthropic/Bedrock/Vertex/Foundry) |
| `ClaudeCode/src/services/oauth/index.ts` | OAuth 2.0 PKCE flow |

## Tool Execution

| Path | Description |
|------|-------------|
| `ClaudeCode/src/services/tools/toolExecution.ts` | Runs individual tools with permissions |
| `ClaudeCode/src/services/tools/toolOrchestration.ts` | Batches parallel/sequential tool calls |
| `ClaudeCode/src/services/tools/StreamingToolExecutor.ts` | Executes tools as they stream from API |

## Key Tools

| Path | Description |
|------|-------------|
| `ClaudeCode/src/tools/BashTool/BashTool.tsx` | Shell command execution |
| `ClaudeCode/src/tools/FileEditTool/FileEditTool.ts` | Search-and-replace file edits |
| `ClaudeCode/src/tools/FileReadTool/FileReadTool.ts` | File reading (images, PDFs, code) |
| `ClaudeCode/src/tools/AgentTool/AgentTool.tsx` | Sub-agent spawning (explore, plan, general) |
| `ClaudeCode/src/tools/GrepTool/GrepTool.ts` | ripgrep-based code search |
| `ClaudeCode/src/tools/MCPTool/MCPTool.ts` | Proxy for MCP server tools |
| `ClaudeCode/src/tools/AskUserQuestionTool/AskUserQuestionTool.tsx` | Multi-choice user prompts |
| `ClaudeCode/src/tools/WebFetchTool/WebFetchTool.ts` | URL fetching and markdown conversion |
| `ClaudeCode/src/tools/WebSearchTool/WebSearchTool.ts` | Web search |
| `ClaudeCode/src/tools/NotebookEditTool/NotebookEditTool.ts` | Jupyter notebook cell editing |
| `ClaudeCode/src/tools/ScheduleCronTool/CronCreateTool.ts` | Cron-based task scheduling |
| `ClaudeCode/src/tools/LSPTool/LSPTool.ts` | LSP code intelligence |

## Task & Team Management Tools

| Path | Description |
|------|-------------|
| `ClaudeCode/src/tools/TaskCreateTool/TaskCreateTool.ts` | Create tasks |
| `ClaudeCode/src/tools/TaskGetTool/TaskGetTool.ts` | Get task by ID |
| `ClaudeCode/src/tools/TaskListTool/TaskListTool.ts` | List all tasks |
| `ClaudeCode/src/tools/TaskOutputTool/TaskOutputTool.tsx` | Get task output (with blocking) |
| `ClaudeCode/src/tools/TaskStopTool/TaskStopTool.ts` | Stop a running task |
| `ClaudeCode/src/tools/TaskUpdateTool/TaskUpdateTool.ts` | Update task status/dependencies |
| `ClaudeCode/src/tools/TeamCreateTool/TeamCreateTool.ts` | Create swarm team |
| `ClaudeCode/src/tools/TeamDeleteTool/TeamDeleteTool.ts` | Disband team |
| `ClaudeCode/src/tools/SendMessageTool/SendMessageTool.ts` | Inter-agent messaging |

## Mode & Config Tools

| Path | Description |
|------|-------------|
| `ClaudeCode/src/tools/ConfigTool/ConfigTool.ts` | Get/set settings |
| `ClaudeCode/src/tools/EnterPlanModeTool/EnterPlanModeTool.ts` | Enter plan mode |
| `ClaudeCode/src/tools/ExitPlanModeTool/ExitPlanModeTool.ts` | Exit plan mode |
| `ClaudeCode/src/tools/EnterWorktreeTool/EnterWorktreeTool.ts` | Create isolated git worktree |
| `ClaudeCode/src/tools/ExitWorktreeTool/ExitWorktreeTool.ts` | Exit worktree |

## MCP Integration

| Path | Description |
|------|-------------|
| `ClaudeCode/src/services/mcp/client.ts` | MCP client — stdio/SSE/HTTP transports |
| `ClaudeCode/src/services/mcp/MCPConnectionManager.tsx` | React context for MCP server management |
| `ClaudeCode/src/entrypoints/mcp.ts` | Claude Code as an MCP server |
| `ClaudeCode/src/tools/McpAuthTool/McpAuthTool.ts` | OAuth for MCP servers |
| `ClaudeCode/src/tools/ListMcpResourcesTool/ListMcpResourcesTool.ts` | List MCP resources |
| `ClaudeCode/src/tools/ReadMcpResourceTool/ReadMcpResourceTool.ts` | Read MCP resource by URI |

## Context & Memory

| Path | Description |
|------|-------------|
| `ClaudeCode/src/services/compact/compact.ts` | Full conversation summarization |
| `ClaudeCode/src/services/compact/autoCompact.ts` | Auto-compact when context fills up |
| `ClaudeCode/src/services/compact/microCompact.ts` | Targeted compaction of old tool results |
| `ClaudeCode/src/services/extractMemories/extractMemories.ts` | Memory extraction from sessions |
| `ClaudeCode/src/services/autoDream/autoDream.ts` | Background memory consolidation |
| `ClaudeCode/src/services/teamMemorySync/index.ts` | Team memory sync per Git repo |
| `ClaudeCode/src/services/MagicDocs/magicDocs.ts` | Auto-maintain `# MAGIC DOC:` markdown files |
| `ClaudeCode/src/memdir/memdir.ts` | Memory directory management |

## Enterprise & Policy

| Path | Description |
|------|-------------|
| `ClaudeCode/src/services/policyLimits/index.ts` | Org policy restrictions (fail-open, hourly poll) |
| `ClaudeCode/src/services/remoteManagedSettings/index.ts` | Enterprise managed settings |
| `ClaudeCode/src/services/settingsSync/index.ts` | Settings sync across environments |

## Agent Intelligence

| Path | Description |
|------|-------------|
| `ClaudeCode/src/services/PromptSuggestion/promptSuggestion.ts` | Next-prompt suggestions |
| `ClaudeCode/src/services/PromptSuggestion/speculation.ts` | Speculative execution |
| `ClaudeCode/src/services/AgentSummary/agentSummary.ts` | 30s background agent summaries |
| `ClaudeCode/src/services/toolUseSummary/toolUseSummaryGenerator.ts` | Tool batch summaries via Haiku |

## UI Components

| Path | Description |
|------|-------------|
| `ClaudeCode/src/components/PromptInput/` | Main user input field with autocomplete |
| `ClaudeCode/src/components/permissions/` | Permission dialogs per tool type |
| `ClaudeCode/src/components/messages/` | Message rendering (assistant, user, tool, bash) |
| `ClaudeCode/src/components/diff/` | Diff browsing with per-turn inspection |
| `ClaudeCode/src/components/Settings/` | Tabbed settings dialog |
| `ClaudeCode/src/components/HelpV2/` | Tabbed help overlay |
| `ClaudeCode/src/components/agents/` | Agent creation wizard and editor |
| `ClaudeCode/src/components/tasks/` | Background task management UI |
| `ClaudeCode/src/components/mcp/` | MCP server management UI |
| `ClaudeCode/src/components/memory/` | Memory file browser |
| `ClaudeCode/src/components/TrustDialog/` | First-run trust acceptance |
| `ClaudeCode/src/components/Spinner/` | Loading animations and tips |
| `ClaudeCode/src/components/StructuredDiff/` | Word-level diff highlighting |
| `ClaudeCode/src/components/HighlightedCode/` | Syntax-highlighted code (LRU cache) |
| `ClaudeCode/src/components/design-system/` | Base primitives (Dialog, Pane, Tabs, FuzzyPicker) |
| `ClaudeCode/src/components/wizard/` | Multi-step wizard framework |
| `ClaudeCode/src/screens/REPL.tsx` | Main REPL screen |
| `ClaudeCode/src/ink/` | Custom Ink runtime fork (layout, termio, events) |

## React Hooks

| Path | Description |
|------|-------------|
| `ClaudeCode/src/hooks/useCanUseTool.ts` | Tool permission checking |
| `ClaudeCode/src/hooks/useAppState.ts` | Global state hook |
| `ClaudeCode/src/hooks/useCommandQueue.ts` | Command processing queue |
| `ClaudeCode/src/hooks/useVirtualScroll.ts` | Message list virtual scrolling |

## Commands (`src/commands/`)

Each command is a directory with an `index.ts`. Three types: `local`, `local-jsx`, `prompt`.

| Category | Commands |
|----------|----------|
| Git/VC | `commit/`, `branch/`, `diff/`, `fork/`, `resume/`, `rewind/`, `teleport/` |
| Review | `review/`, `bughunter/` |
| Config | `config/`, `permissions/`, `model/`, `effort/`, `env/`, `output-style/`, `color/`, `theme/`, `keybindings/`, `vim/`, `sandbox-toggle/` |
| Session | `compact/`, `session/`, `share/`, `clear/`, `exit/` |
| MCP/Plugins | `mcp/`, `plugin/`, `skills/`, `hooks/`, `passes/` |
| Planning | `plan/`, `memory/`, `ctx_viz/`, `thinkback/`, `thinkback-play/` |
| Agents | `agents/`, `tasks/`, `peers/`, `buddy/`, `workflows/` |
| Onboarding | `help/`, `onboarding/`, `status/`, `doctor/`, `upgrade/` |
| Web/Remote | `bridge/`, `chrome/`, `desktop/`, `mobile/`, `remote-env/`, `remote-setup/` |
| Info | `cost/`, `usage/`, `stats/`, `debug-tool-call/`, `fast/` |
| Utility | `add-dir/`, `copy/`, `export/`, `files/`, `login/`, `logout/`, `voice/` |

## Utilities (`src/utils/`)

| Path | Description |
|------|-------------|
| `src/utils/bash/` | Shell command parsing via tree-sitter AST |
| `src/utils/git/` | Filesystem-based git state (no subprocesses) |
| `src/utils/github/` | GitHub CLI auth detection |
| `src/utils/permissions/` | Permission system, YOLO classifier, rule engine |
| `src/utils/sandbox/` | Sandbox runtime adapter |
| `src/utils/settings/` | Multi-scope settings (user/project/local/policy) |
| `src/utils/model/` | Model selection, providers, cost tiers |
| `src/utils/plugins/` | Plugin lifecycle, marketplaces, MCP integration |
| `src/utils/swarm/` | In-process teammate runner with context isolation |
| `src/utils/hooks/` | Hook system (pre/post-tool, agent, HTTP hooks) |
| `src/utils/telemetry/` | OpenTelemetry traces, metrics, BigQuery export |
| `src/utils/teleport/` | Remote session API client |
| `src/utils/processUserInput/` | Input dispatch (bash/slash/text) |
| `src/utils/suggestions/` | Autocomplete (Fuse.js, shell history, skills) |
| `src/utils/powershell/` | PowerShell AST parsing and security |
| `src/utils/computerUse/` | Desktop automation (mouse, keyboard, screenshots) |
| `src/utils/claudeInChrome/` | Chrome extension MCP server and native host |
| `src/utils/deepLink/` | `claude-cli://open` URL scheme handling |

## Other `src/` Directories

| Path | Description |
|------|-------------|
| `src/assistant/` | Session discovery, history pagination, chooser |
| `src/bridge/` | Remote control / bridge mode (IDE integration) |
| `src/buddy/` | Pixel art companion character system |
| `src/cli/` | CLI subcommand handlers (bg, up, exit, rollback) |
| `src/coordinator/` | Multi-agent coordinator mode |
| `src/daemon/` | Background daemon with worker registry |
| `src/environment-runner/` | Headless BYOC environment runner |
| `src/jobs/` | Background job classifier |
| `src/keybindings/` | Keyboard shortcuts with chord support |
| `src/migrations/` | Settings migration on upgrade |
| `src/native-ts/` | Pure TS ports of NAPI modules (file-index, color-diff, yoga) |
| `src/outputStyles/` | Loadable output styles from `.claude/output-styles/*.md` |
| `src/plugins/` | Plugin system index |
| `src/remote/` | Remote session WebSocket management |
| `src/server/` | Local server for direct-connect (IDE) |
| `src/ssh/` | SSH session management |
| `src/vim/` | Vim motions for prompt input |
| `src/voice/` | Voice mode feature gating |

## Packages (`packages/`)

| Path | Description |
|------|-------------|
| `packages/@ant/claude-for-chrome-mcp/src/` | Chrome browser automation MCP (navigate, screenshot, JS) |
| `packages/@ant/computer-use-mcp/src/` | Desktop screen control MCP (click, type, scroll, screenshot) |
| `packages/@ant/computer-use-input/js/` | macOS native keyboard/mouse input |
| `packages/@ant/computer-use-swift/js/` | macOS native screen capture and app listing |
| `packages/@anthropic-ai/claude-agent-sdk/` | Public TypeScript Agent SDK (v0.2.88) |
| `packages/audio-capture/` | Native audio recording/playback (6 platforms) |
| `packages/color-diff-napi/` | Syntax-highlighted diff rendering (highlight.js) |
| `packages/modifiers-napi/` | macOS keyboard modifier key state |
| `packages/image-processor/` | Sharp-compatible image processing |
| `packages/ripgrep/` | Bundled `rg` binary (arm64/x64 × darwin/linux/win32) |
| `packages/url-handler/` | macOS `claude://` URL scheme handler |

## Go Rewrite Plan

| Path | Description |
|------|-------------|
| `.claude/plans/write-a-plan-to-delightful-dusk.md` | 12-module Go rewrite plan (M1–M12) |
| `.claude/skills/go/SKILL.md` | Go skill definition, rules, Go 1.26 features |
| `.claude/skills/go/references/conventions.md` | Go coding conventions |
| `.claude/skills/go/references/go-version.md` | Go version tracking (1.26.2) |
| `.claude/skills/go/scripts/check-go-version.sh` | Version drift detection script |

## Config & Build

| Path | Description |
|------|-------------|
| `ClaudeCode/package.json` | Dependencies and scripts |
| `ClaudeCode/bunfig.toml` | Bun MACRO defines (VERSION, BUILD_TIME) |
| `ClaudeCode/tsconfig.json` | TypeScript config (ES2022, ESM, strict: false) |
| `ClaudeCode/global.d.ts` | Global type declarations |
| `ClaudeCode/scripts/build.ts` | Build script with feature flags |
| `ClaudeCode/scripts/gen-stubs.ts` | Stub generator for missing modules |
| `ClaudeCode/patches/commander@13.0.0.patch` | Loose short-flag parsing patch |
| `ClaudeCode/.editorconfig` | 2-space indent, LF, UTF-8 |
| `.claude/settings.local.json` | Local permissions config |
