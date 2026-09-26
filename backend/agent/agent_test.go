package agent

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// fakeLLM replays scripted assistant messages and records the requests it got.
type fakeLLM struct {
	mu       sync.Mutex
	replies  []string // raw JSON for choices[0].message
	requests []map[string]any
}

func (f *fakeLLM) handler(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)
	f.requests = append(f.requests, body)
	i := len(f.requests) - 1
	if i >= len(f.replies) {
		i = len(f.replies) - 1
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"choices":[{"message":` + f.replies[i] + `}],"usage":{"prompt_tokens":10,"completion_tokens":5}}`))
}

func toolCall(id, name, args string) string {
	b, _ := json.Marshal(map[string]any{
		"role": "assistant", "content": nil,
		"tool_calls": []map[string]any{{"id": id, "type": "function", "function": map[string]string{"name": name, "arguments": args}}},
	})
	return string(b)
}

func newRunner(url string) (*Runner, *int) {
	calls := 0
	return &Runner{
		Client: &Client{BaseURL: url, Model: "test"},
		Tools: []Tool{{
			Name: "get_price",
			Run: func(ctx context.Context, args json.RawMessage) (any, error) {
				calls++
				var a struct{ Coin string }
				_ = json.Unmarshal(args, &a)
				if a.Coin == "bad" {
					return nil, errors.New("coin not found")
				}
				return map[string]float64{"price": 42}, nil
			},
		}},
		FinalTool: Tool{Name: "submit"},
		MaxSteps:  4,
	}, &calls
}

func TestRunToolThenSubmit(t *testing.T) {
	f := &fakeLLM{replies: []string{
		toolCall("1", "get_price", `{"coin":"btc"}`),
		toolCall("2", "submit", `{"direction":"bullish"}`),
	}}
	srv := httptest.NewServer(http.HandlerFunc(f.handler))
	defer srv.Close()

	r, calls := newRunner(srv.URL)
	var seen []Step
	r.OnStep = func(s Step) { seen = append(seen, s) }
	res, err := r.Run(context.Background(), "sys", "analyze btc")
	if err != nil {
		t.Fatal(err)
	}
	if string(res.Final) != `{"direction":"bullish"}` {
		t.Fatalf("final = %s", res.Final)
	}
	if *calls != 1 || len(res.Steps) != 1 || len(seen) != 1 || res.Steps[0].Result != `{"price":42}` {
		t.Fatalf("unexpected steps: calls=%d steps=%+v", *calls, res.Steps)
	}
	if res.Usage.PromptTokens != 20 || res.Rounds != 2 {
		t.Fatalf("usage/rounds wrong: %+v rounds=%d", res.Usage, res.Rounds)
	}
	// The second request must contain the tool result message.
	msgs := f.requests[1]["messages"].([]any)
	last := msgs[len(msgs)-1].(map[string]any)
	if last["role"] != "tool" || last["tool_call_id"] != "1" || !strings.Contains(last["content"].(string), "42") {
		t.Fatalf("tool result not sent back: %+v", last)
	}
}

func TestToolErrorIsReportedToModel(t *testing.T) {
	f := &fakeLLM{replies: []string{
		toolCall("1", "get_price", `{"coin":"bad"}`),
		toolCall("2", "submit", `{}`),
	}}
	srv := httptest.NewServer(http.HandlerFunc(f.handler))
	defer srv.Close()
	r, _ := newRunner(srv.URL)
	res, err := r.Run(context.Background(), "sys", "u")
	if err != nil {
		t.Fatal(err)
	}
	if res.Steps[0].Error != "coin not found" {
		t.Fatalf("step error = %q", res.Steps[0].Error)
	}
	msgs := f.requests[1]["messages"].([]any)
	if c := msgs[len(msgs)-1].(map[string]any)["content"].(string); !strings.Contains(c, "coin not found") {
		t.Fatalf("error not passed to model: %s", c)
	}
}

func TestLastRoundForcesFinalTool(t *testing.T) {
	f := &fakeLLM{replies: []string{
		toolCall("1", "get_price", `{}`),
		toolCall("2", "get_price", `{}`),
		toolCall("3", "get_price", `{}`),
		toolCall("4", "submit", `{"ok":true}`),
	}}
	srv := httptest.NewServer(http.HandlerFunc(f.handler))
	defer srv.Close()
	r, _ := newRunner(srv.URL)
	if _, err := r.Run(context.Background(), "s", "u"); err != nil {
		t.Fatal(err)
	}
	if len(f.requests) != 4 {
		t.Fatalf("requests = %d", len(f.requests))
	}
	if _, ok := f.requests[2]["tool_choice"]; ok {
		t.Fatal("tool_choice should not be forced before the last round")
	}
	tc, ok := f.requests[3]["tool_choice"].(map[string]any)
	if !ok || tc["function"].(map[string]any)["name"] != "submit" {
		t.Fatalf("last round should force submit, got %v", f.requests[3]["tool_choice"])
	}
}

func TestPlainTextIsNudgedOnceThenFails(t *testing.T) {
	f := &fakeLLM{replies: []string{`{"role":"assistant","content":"BTC looks fine"}`}}
	srv := httptest.NewServer(http.HandlerFunc(f.handler))
	defer srv.Close()
	r, _ := newRunner(srv.URL)
	_, err := r.Run(context.Background(), "s", "u")
	if !errors.Is(err, ErrNoFinal) {
		t.Fatalf("err = %v, want ErrNoFinal", err)
	}
	if len(f.requests) != 2 {
		t.Fatalf("should nudge exactly once, requests = %d", len(f.requests))
	}
}

func TestAPIErrorSurfaces(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid api key"}}`))
	}))
	defer srv.Close()
	r, _ := newRunner(srv.URL)
	_, err := r.Run(context.Background(), "s", "u")
	if err == nil || !strings.Contains(err.Error(), "invalid api key") {
		t.Fatalf("err = %v", err)
	}
}
