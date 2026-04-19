---
name: weekly-report
description: Generate a structured weekly engineering report from recent repository activity.
allowed-tools:
  - Bash(git status:*)
  - Bash(git branch --show-current:*)
  - Bash(git log:*)
  - Bash(git diff:*)
  - Bash(git shortlog:*)
  - Bash(git rev-parse:*)
  - Read
  - Glob
  - Grep
when_to_use: Use when the user wants a weekly report, week-in-review, weekly update, or summary of recent repository work. Examples: "write a weekly report", "summarize this week's work", "generate a weekly engineering update", "what happened in the repo this week?" Adapt the report to the user's explicit instructions about date range, audience, focus area, or whether to include in-progress work.
argument-hint: "[range, audience, or focus notes]"
arguments:
  - request
---

# Weekly Report

Generate a concise, evidence-based weekly report for this repository.

## Inputs

- `$request`: Optional notes about the reporting window, audience, focus area, or scope. Examples: `last 7 days`, `this week for engineering leadership`, `focus on the Go rewrite`, `include in-progress work`.

## Goal

Produce a structured weekly report grounded in repository evidence. Default to the last 7 days and an engineering audience when the user does not specify otherwise. If the user gives explicit constraints, follow those instead of the defaults.

## Steps

### 1. Determine the reporting scope

Resolve the report inputs before gathering evidence.

- If the user specifies a date range, use it.
- Otherwise, default to the last 7 days.
- If the user specifies an audience or tone, use it.
- Otherwise, default to engineering.
- If the user specifies a focus area, include/exclude rules, or whether to mention work in progress, treat those instructions as authoritative.
- If there is still ambiguity, make the smallest reasonable assumption and state it briefly in the final report.

**Success criteria**:
- A concrete reporting window is established.
- The audience/tone is identified.
- Any explicit scope constraints from the user are captured.

### 2. Gather repository evidence

Collect enough factual input to support the report.

Use read-only repository evidence such as:
- current branch
- working tree status
- commits in the reporting window
- diff or stat summaries for the reporting window
- changed areas/files that meaningfully represent the work

Guidelines:
- Review all relevant commits in the window, not just the latest commit.
- Prefer grouped patterns and themes over raw chronological dumps.
- Avoid over-indexing on noisy files such as lockfiles, generated artifacts, build outputs, or config-only churn unless they are central to the week's work.
- If the user asked to include in-progress work, label it separately from committed work.
- Distinguish committed work, current uncommitted work, and assumptions.

**Success criteria**:
- You have factual evidence for the report's major accomplishments.
- You can identify the most meaningful changed areas and follow-ups.

**Rules**:
- Do not invent completed work, impact, metrics, or business outcomes.
- Do not blur uncommitted work together with completed work.
- Do not present command output verbatim when a concise synthesis is clearer.

### 3. Synthesize the work into themes

Turn the raw repository evidence into a small set of coherent themes.

- Group the week's activity into 2-5 accomplishments, workstreams, or themes.
- Focus on why the work mattered, not only which files changed.
- Use short, specific labels.
- If the week was fragmented, group by subsystem or outcome rather than by individual commit.

**Success criteria**:
- The week's work is organized into a few clear themes.
- Each theme is supported by repository evidence.

### 4. Write the report in structured markdown

Write the final report with this shape unless the user asked for a different format:

## Reporting period
- State the resolved window.

## Summary
- 2-4 bullets describing the week's most important progress.

## Key accomplishments
- Bullets grouped by theme or workstream.

## Notable changes by area
- Short bullets naming the main subsystems, modules, or files affected and why they matter.

## Risks / open questions
- Mention blockers, uncertainty, or important unfinished work when supported by the evidence.

## Next steps
- Suggest likely follow-up work only when it is directly implied by the week's activity or explicitly requested.

Audience guidance:
- For engineering, include useful technical detail and changed areas.
- For managers or executives, compress jargon and emphasize outcomes.
- For broader stakeholders, reduce implementation detail and keep the language plain.

**Success criteria**:
- The report is concise, structured, and evidence-based.
- The tone matches the requested or inferred audience.

### 5. Call out assumptions clearly

If any part of the report required inference, say so briefly.

Examples:
- assumed the reporting window was the last 7 days
- excluded work in progress because it was not requested
- focused on a named subsystem because the user asked for it

**Success criteria**:
- The report is transparent about assumptions.
- Readers can distinguish verified activity from inferred framing.
