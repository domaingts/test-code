package queryengine

import (
	"encoding/json"

	"github.com/example/claude-code-go/pkg/claudetypes"
	"github.com/example/claude-code-go/pkg/llm"
)

// buildRequest constructs an llm.Request from the engine config and messages.
func (e *engine) buildRequest(messages []claudetypes.Message) llm.Request {
	return llm.Request{
		Model:     e.cfg.Model,
		MaxTokens: e.cfg.MaxTokens,
		System:    e.cfg.SystemPrompt,
		Messages:  messages,
		Tools:     e.toolSpecs(),
		Stream:    true,
	}
}

// toolSpecs converts the registered tools into llm.ToolSpec values.
func (e *engine) toolSpecs() []llm.ToolSpec {
	if e.cfg.Tools == nil {
		return nil
	}
	tools := e.cfg.Tools.All()
	specs := make([]llm.ToolSpec, 0, len(tools))
	for _, t := range tools {
		schema := t.Schema()
		schemaBytes, err := json.Marshal(schema)
		if err != nil {
			continue
		}
		var schemaMap map[string]any
		if err := json.Unmarshal(schemaBytes, &schemaMap); err != nil {
			continue
		}
		specs = append(specs, llm.ToolSpec{
			Name:        t.Name(),
			Description: schema.Description,
			Schema:      schemaMap,
		})
	}
	return specs
}
