// Package agent is a small, framework-free tool-calling agent loop for any
// OpenAI-compatible chat completions API (OpenAI, DeepSeek, Ollama, ...).
//
// The loop:
//
//	messages = [system, user]
//	repeat up to MaxSteps:
//	    reply = LLM(messages, tools)
//	    if reply calls the final tool -> return its arguments
//	    run every other tool call, append results as "tool" messages
//	on the last step the final tool is forced via tool_choice
//
// It only uses the standard library so it can be read and tested in isolation.
package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Tool is something the model may call.
type Tool struct {
	Name        string
	Description string
	// Parameters is a JSON Schema object describing the arguments.
	Parameters map[string]any
	// Run executes the tool. Its result is JSON-encoded and sent back to the
	// model. An error is reported to the model (not fatal to the loop).
	Run func(ctx context.Context, args json.RawMessage) (any, error)
}

// Message is one chat message in OpenAI format.
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// ToolCall is a model's request to run a tool.
type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// Step records one tool execution, for the UI and for debugging.
type Step struct {
	Index      int    `json:"index"`
	Tool       string `json:"tool"`
	Args       string `json:"args"`
	Result     string `json:"result"`
	Error      string `json:"error,omitempty"`
	DurationMs int64  `json:"durationMs"`
}

// Client talks to an OpenAI-compatible /chat/completions endpoint.
type Client struct {
	BaseURL string
	APIKey  string
	Model   string
	HTTP    *http.Client
}

// Usage is token usage reported by the API (if any).
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

type chatResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
	Usage *Usage `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (c *Client) chat(ctx context.Context, msgs []Message, tools []Tool, forceTool string) (*Message, *Usage, error) {
	toolDefs := make([]map[string]any, 0, len(tools))
	for _, t := range tools {
		params := t.Parameters
		if params == nil {
			params = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		toolDefs = append(toolDefs, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        t.Name,
				"description": t.Description,
				"parameters":  params,
			},
		})
	}
	body := map[string]any{
		"model":       c.Model,
		"messages":    msgs,
		"tools":       toolDefs,
		"temperature": 0.2,
	}
	if forceTool != "" {
		body["tool_choice"] = map[string]any{"type": "function", "function": map[string]any{"name": forceTool}}
	}
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.BaseURL, "/")+"/chat/completions", bytes.NewReader(buf))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	hc := c.HTTP
	if hc == nil {
		hc = &http.Client{Timeout: 90 * time.Second}
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	var out chatResponse
	if jerr := json.Unmarshal(raw, &out); jerr != nil {
		return nil, nil, fmt.Errorf("LLM API HTTP %d: %s", resp.StatusCode, truncate(string(raw), 300))
	}
	if out.Error != nil {
		return nil, nil, fmt.Errorf("LLM API error (HTTP %d): %s", resp.StatusCode, out.Error.Message)
	}
	if resp.StatusCode >= 400 {
		return nil, nil, fmt.Errorf("LLM API HTTP %d: %s", resp.StatusCode, truncate(string(raw), 300))
	}
	if len(out.Choices) == 0 {
		return nil, nil, errors.New("LLM API returned no choices")
	}
	return &out.Choices[0].Message, out.Usage, nil
}

// Runner runs the loop.
type Runner struct {
	Client    *Client
	Tools     []Tool
	FinalTool Tool // the model calls this to submit its answer
	MaxSteps  int  // max LLM round trips, default 6
	// OnStep, if set, is called after every tool execution (for live UI updates).
	OnStep func(Step)
}

// Result of a run.
type Result struct {
	Final  json.RawMessage // arguments passed to FinalTool
	Steps  []Step
	Usage  Usage
	Rounds int
}

// ErrNoFinal is returned when the model never submits via the final tool.
var ErrNoFinal = errors.New("model did not submit a final answer")

const maxToolResultChars = 6000

// Run executes the agent loop.
func (r *Runner) Run(ctx context.Context, system, user string) (*Result, error) {
	if r.Client == nil {
		return nil, errors.New("agent: nil client")
	}
	maxSteps := r.MaxSteps
	if maxSteps <= 0 {
		maxSteps = 6
	}
	all := append([]Tool{}, r.Tools...)
	all = append(all, r.FinalTool)
	byName := make(map[string]Tool, len(all))
	for _, t := range all {
		byName[t.Name] = t
	}

	msgs := []Message{{Role: "system", Content: system}, {Role: "user", Content: user}}
	res := &Result{}
	nudged := false

	for round := 0; round < maxSteps; round++ {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		force := ""
		if round == maxSteps-1 {
			force = r.FinalTool.Name
		}
		reply, usage, err := r.Client.chat(ctx, msgs, all, force)
		res.Rounds = round + 1
		if usage != nil {
			res.Usage.PromptTokens += usage.PromptTokens
			res.Usage.CompletionTokens += usage.CompletionTokens
		}
		if err != nil {
			return res, err
		}

		if len(reply.ToolCalls) == 0 {
			// The model answered in plain text. Ask once for a structured submission.
			if nudged {
				return res, fmt.Errorf("%w: %s", ErrNoFinal, truncate(reply.Content, 200))
			}
			nudged = true
			msgs = append(msgs, Message{Role: "assistant", Content: reply.Content},
				Message{Role: "user", Content: "Please call " + r.FinalTool.Name + " now to submit your analysis in the required structure."})
			continue
		}

		msgs = append(msgs, Message{Role: "assistant", Content: reply.Content, ToolCalls: reply.ToolCalls})
		for _, call := range reply.ToolCalls {
			if call.Function.Name == r.FinalTool.Name {
				args := call.Function.Arguments
				if !json.Valid([]byte(args)) {
					// Tell the model its JSON was broken and let it retry.
					msgs = append(msgs, Message{Role: "tool", ToolCallID: call.ID, Content: `{"error":"arguments were not valid JSON, call again"}`})
					continue
				}
				res.Final = json.RawMessage(args)
				return res, nil
			}
			step := r.runTool(ctx, byName, call, len(res.Steps)+1)
			res.Steps = append(res.Steps, step)
			if r.OnStep != nil {
				r.OnStep(step)
			}
			content := step.Result
			if step.Error != "" {
				content = mustJSON(map[string]string{"error": step.Error})
			}
			msgs = append(msgs, Message{Role: "tool", ToolCallID: call.ID, Content: content})
		}
	}
	return res, ErrNoFinal
}

func (r *Runner) runTool(ctx context.Context, byName map[string]Tool, call ToolCall, idx int) Step {
	start := time.Now()
	step := Step{Index: idx, Tool: call.Function.Name, Args: call.Function.Arguments}

	t, ok := byName[call.Function.Name]
	if !ok || t.Run == nil {
		step.Error = "unknown tool: " + call.Function.Name
		step.DurationMs = time.Since(start).Milliseconds()
		return step
	}
	args := json.RawMessage(call.Function.Arguments)
	if len(strings.TrimSpace(call.Function.Arguments)) == 0 {
		args = json.RawMessage("{}")
	}
	out, err := t.Run(ctx, args)
	if err != nil {
		step.Error = err.Error()
	} else {
		step.Result = truncate(mustJSON(out), maxToolResultChars)
	}
	step.DurationMs = time.Since(start).Milliseconds()
	return step
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}
	return string(b)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…(truncated)"
}
