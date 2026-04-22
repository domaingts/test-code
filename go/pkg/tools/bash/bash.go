package bash

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	toolpkg "github.com/example/claude-code-go/pkg/tool"
)

// Tool implements the "Bash" tool — executes shell commands.
type Tool struct{}

func (Tool) Name() string { return "Bash" }

func (Tool) Schema() toolpkg.JSONSchema {
	return toolpkg.JSONSchema{
		Type: "object",
		Properties: map[string]toolpkg.Property{
			"command":     {Type: "string", Description: "Shell command to execute"},
			"timeout":     {Type: "integer", Description: "Timeout in milliseconds"},
			"description": {Type: "string", Description: "Human-readable description of what the command does"},
		},
		Required: []string{"command"},
	}
}

type input struct {
	Command     string `json:"command"`
	Timeout     int    `json:"timeout,omitempty"`
	Description string `json:"description,omitempty"`
}

type output struct {
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
	ExitCode int   `json:"exitCode"`
}

const defaultTimeout = 120 * time.Second
const maxTimeout = 600 * time.Second

func (t Tool) Call(ctx context.Context, raw json.RawMessage, tc toolpkg.ToolContext) (<-chan toolpkg.ToolEvent, error) {
	var in input
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, fmt.Errorf("parse input: %w", err)
	}

	ch := make(chan toolpkg.ToolEvent, 1)

	timeout := defaultTimeout
	if in.Timeout > 0 {
		timeout = min(time.Duration(in.Timeout)*time.Millisecond, maxTimeout)
	}

	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "bash", "-c", in.Command)
	cmd.Dir = tc.CWD

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Send progress if tool takes > 2s
	done := make(chan error, 1)
	go func() {
		done <- cmd.Run()
	}()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	select {
	case err := <-done:
		exitCode := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				exitCode = 1
			}
		}

		// Apply output limits
		stdoutStr := truncateOutput(stdout.String(), 30000)
		stderrStr := truncateOutput(stderr.String(), 10000)

		out := output{
			Stdout:   stdoutStr,
			Stderr:   stderrStr,
			ExitCode: exitCode,
		}

		data, _ := json.Marshal(out)
		ch <- toolpkg.ToolEvent{
			Type:   toolpkg.EventResult,
			Result: &toolpkg.ToolResult{Output: string(data)},
		}

	case <-ticker.C:
		// Send progress
		if tc.OnProgress != nil {
			tc.OnProgress(toolpkg.ToolEvent{
				Type:     toolpkg.EventProgress,
				Progress: &toolpkg.ToolProgress{Message: fmt.Sprintf("Running: %s...", truncateString(in.Description, 50))},
			})
		}
		// Wait for completion after progress
		err := <-done
		exitCode := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				exitCode = 1
			}
		}

		out := output{
			Stdout:   truncateOutput(stdout.String(), 30000),
			Stderr:   truncateOutput(stderr.String(), 10000),
			ExitCode: exitCode,
		}

		data, _ := json.Marshal(out)
		ch <- toolpkg.ToolEvent{
			Type:   toolpkg.EventResult,
			Result: &toolpkg.ToolResult{Output: string(data)},
		}

	case <-tc.Abort:
		cancel()
		<-done // wait for process to exit
		ch <- toolpkg.ToolEvent{
			Type:   toolpkg.EventResult,
			Result: &toolpkg.ToolResult{Output: `{"stdout":"","stderr":"interrupted","exitCode":-1}`},
		}
	}

	close(ch)
	return ch, nil
}

func truncateOutput(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n... (truncated)"
}

func truncateString(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

var _ toolpkg.Tool = Tool{}
