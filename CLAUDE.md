# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This repo contains two things:

1. **`ClaudeCode/`** — A recovered source tree of `@anthropic-ai/claude-code@2.1.88`, rebuilt from the published npm package's sourcemap. ~2000 TypeScript files, React-Ink terminal UI, streaming Anthropic API, 50+ tools, 80+ slash commands. For personal study only.

2. **Planned Go rewrite** — A full rewrite into Go, organized into 12 modules (M1–M12). See `.claude/plans/write-a-plan-to-delightful-dusk.md` for the plan. Target layout under `go/` with `cmd/`, `pkg/`, and `internal/`.

## Build Commands (ClaudeCode)

```bash
cd ClaudeCode
bun install                  # install dependencies
bun run gen-stubs            # generate stub files for missing modules (required before build)
bun run build                # production build → dist/cli.js (minified, external features off)
bun run build:dev            # dev build (all features on, no minify)
bun run build:sourcemap      # build with sourcemap
bun run dev                  # run source directly with Bun (no build)
bun run start                # run built dist/cli.js with Node
bun run typecheck            # TypeScript type checking (tsc --noEmit)
bun run clean                # remove dist/
```

## Architecture (ClaudeCode)

Full documentation: [`ClaudeCode/ARCHITECTURE.md`](ClaudeCode/ARCHITECTURE.md).

**Tech stack:** TypeScript, React 19 (via Ink/react-reconciler), Bun (build + macros), Zod, Commander, Chalk.

**Module system:** ESM (`"type": "module"`). TypeScript strict mode is OFF (recovered source).

**Path alias:** `src/*` resolves to `./src/*` (configured in tsconfig.json).

**Entry point:** `src/entrypoints/cli.tsx` — fast-paths for `--version`, then dynamic imports for full CLI, daemon, bridge, MCP server, and SDK modes.

### Key Layers

- **Query Engine** (`src/QueryEngine.ts`, `src/query/`) — The main conversation loop. Sends messages to Anthropic API, processes tool calls, manages context.
- **Tool System** (`src/Tool.ts`, `src/tools/`) — Each tool is a directory with `buildTool()` factory, Zod input schema, and optional React UI. 50+ tools (Bash, FileEdit, Grep, Agent, MCP, etc.).
- **Services** (`src/services/`) — API client (multi-provider: Anthropic/Bedrock/Vertex/Foundry), tool execution/orchestration, MCP client, OAuth, context compaction, memory extraction, enterprise policy.
- **Commands** (`src/commands/`) — Three types: `local` (async function), `local-jsx` (React component), `prompt` (text to model). 80+ slash commands.
- **Components** (`src/components/`) — React-Ink terminal UI. Permission dialogs, diff viewer, message renderer, prompt input, settings panel.
- **Ink** (`src/ink/`) — Custom fork of Ink with yoga-layout, terminal querying, focus management, keypress parsing.

### Important Patterns

- **Feature flags:** `bun:bundle` `feature()` calls → build-time dead code elimination. Flags defined in `scripts/build.ts` and `bunfig.toml`.
- **MACRO constants:** `MACRO.VERSION`, `MACRO.BUILD_TIME` etc. are compile-time replacements defined in `bunfig.toml`.
- **Stubs:** 142 modules are auto-generated stubs (missing from sourcemap). Managed by `scripts/gen-stubs.ts`. These export `{} as any` or empty objects.
- **Forked subagents:** `runForkedAgent()` creates lightweight sub-agents for summarization, memory, speculation. Uses `CacheSafeParams` to share prompt cache.
- **Native modules:** `packages/` contains platform-specific `.node` binaries with lazy-loading wrappers. TypeScript ports exist in `src/native-ts/`.

## Go Rewrite

A Go skill is configured at `.claude/skills/go/SKILL.md` with conventions and version tracking (Go 1.26.2).

The rewrite plan targets:
```
go/
  cmd/claude/          # main binary
  cmd/claude-sdk/      # headless SDK
  pkg/claudetypes/     # M1 — shared types
  pkg/config/          # M2 — settings, env, paths
  pkg/llm/             # M3 — Anthropic client
  pkg/tool/            # M4 — Tool interface + registry
  pkg/tools/           # M5 — concrete implementations
  pkg/permission/      # M6 — can-use-tool policy
  pkg/mcp/             # M7 — MCP client + server
  pkg/session/         # M8 — transcript, history, cache
  pkg/queryengine/     # M9 — turn loop
  pkg/commands/        # M10 — slash commands
  pkg/tui/             # M11 — Bubble Tea UI
  pkg/sdk/             # M12 — headless SDK
```

Module rule: no import cycles, no module imports a module with a higher number.

## Editor Config

2-space indent, LF line endings, UTF-8, trim trailing whitespace. No linter or formatter is configured beyond `.editorconfig`.
