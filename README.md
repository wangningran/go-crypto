# go-crypto

A desktop crypto market tracker with a **tool-calling AI agent** that analyzes coins and **scores its own predictions** against the real price 24 hours later.

Built with Go, Wails v2, and Vue 3 + Naive UI. Inspired by [go-stock](https://github.com/ArvinLovegood/go-stock).

<!-- TODO: add a GIF of the AI analysis drawer and the AI Track Record page -->

## Features

| | |
|---|---|
| 📋 **Watchlist** | Track coins; prices refresh on a schedule you set |
| 🤖 **AI Analysis** | An agent pulls price history, market data and news through tools, then returns a structured 24h call: direction, confidence, support, resistance, summary |
| 🎯 **AI Track Record** | Every call is checked against the real price 24h later. Accuracy by direction, and confidence on right vs. wrong calls |
| 📊 **Market Overview** | Total market cap, BTC dominance, top 50 coins |
| 🔔 **Price Alerts** | High/low targets. An alert fires once, then pauses until you resume it |
| 📰 **News** | Latest crypto headlines |
| ⚙️ **Settings** | Any OpenAI-compatible LLM (OpenAI, DeepSeek, Ollama…), currency (USD/EUR/CNY/BTC), refresh interval |

## How the AI analysis works

```mermaid
flowchart LR
    U[Click 🤖 AI] --> C{Analyzed in the<br/>last 5 min?}
    C -- yes --> R[Return cached result]
    C -- no --> L[Agent loop]
    L -->|tool call| T1[get_price_history<br/>SMA, RSI, volatility,<br/>30-day range]
    L -->|tool call| T2[get_market_overview]
    L -->|tool call| T3[get_news]
    L -->|tool call| T4[get_price_snapshot]
    T1 & T2 & T3 & T4 -->|results| L
    L -->|submit_analysis| V[Validate JSON<br/>save to SQLite]
    V --> UI[Drawer: steps live,<br/>direction, levels, summary]
    V -.24h later.-> E[Evaluator: fetch real price,<br/>score the call]
    E --> TR[AI Track Record page]
```

The agent loop (`backend/agent`) is written from scratch on the chat completions API, with no framework:

1. Send the system prompt, the task and the tool definitions to the model.
2. If the model calls tools, run them and send the results back as `tool` messages.
3. Repeat until the model calls `submit_analysis`. On the last allowed round (6), that tool is forced through `tool_choice`, so the loop always ends with structured output.
4. If a tool fails, the error goes back to the model and the loop keeps going. If the model replies in plain text, it is asked once to submit properly.

### Design decisions

- **Code computes, the model interprets.** Moving averages, RSI and volatility are computed in Go (`backend/indicators`, unit-tested) from CoinGecko history. The prompt forbids numbers that did not come from a tool. The first version sent six 24h numbers and asked for "trend and key levels", and the model had to guess.
- **Structured output through a tool.** `submit_analysis` has a JSON Schema. The output is validated in `backend/analysis/core`: the direction enum is normalized, confidence is clamped to 0–1 (a 0–100 answer is converted), and support/resistance are swapped if reversed.
- **Every call is scored.** Every 30 minutes a job checks analyses older than 24h against the price at +24h. If the analysis-time price is more than 5 minutes old, it is re-fetched first, because it is the baseline. Records whose +24h price can't be fetched are retried, and after 6 attempts marked "unscorable", so they never block newer records. The scoring rules:
  - bullish is right if the price rose
  - bearish is right if it fell
  - neutral is right if it moved less than ±1%
  
  This turns "the AI sounds smart" into a number, and shows whether its confidence means anything.
- **Cost and rate limits.** Results are cached for 5 minutes per coin and model. Closing the panel cancels a running analysis, so no more tokens are spent on it. CoinGecko responses are cached in memory for 20s to 10min by a small TTL cache (`backend/data/ttlcache.go`); concurrent requests for the same data share one API call. After an HTTP 429, all CoinGecko calls pause (honoring `Retry-After`) instead of making the limit worse. The evaluator scores at most a few records per run, 3s apart.
- **Live progress without token streaming.** Each tool call is pushed to the UI as a Wails event, so you can watch the agent work.

## Other engineering notes

- **Pure-Go SQLite** (`glebarez/sqlite`, based on `modernc.org/sqlite`): no cgo, no C toolchain. WAL mode plus a 5s busy timeout, because the price refresh, the evaluator and UI actions write concurrently. All timestamps are stored in UTC.
- **Background jobs never pile up.** If a refresh is still running (e.g. slow network), the next tick is skipped instead of queued.
- **API key in the OS keychain** (macOS Keychain, Windows Credential Manager, Linux Secret Service) via `zalando/go-keyring`, never in SQLite. Keys saved by older versions are migrated on startup. On Linux without a keychain, set `GO_CRYPTO_LLM_API_KEY` instead.
- **Currency-safe.** Prices and alerts record their currency. An alert set in EUR is never compared with a USD price, and switching currency clears and re-fetches cached prices.
- **Explicit errors.** CoinGecko HTTP errors, including 429 rate limits, are reported in the UI instead of showing `$0`.
- **Typed frontend.** Components use the Wails-generated TypeScript bindings (`frontend/wailsjs`) rather than `window.go`. Event listeners are cleaned up on unmount.

## Project layout

```
app.go                      Wails-bound methods (the API the UI calls), cron jobs, alerts
backend/agent/              Framework-free tool-calling agent loop (+ tests with a fake LLM server)
backend/analysis/core/      Prompts, output schema, validation, scoring (+ tests)
backend/analysis/service.go Tools wired to real data, caching, evaluator
backend/indicators/         SMA, RSI, volatility, daily resampling (+ tests)
backend/data/               CoinGecko, news, settings/keychain
frontend/src/               Vue 3 + Naive UI
frontend/wailsjs/           Generated bindings (regenerated by `wails dev/build`)
```

## Build

Requirements: Go 1.22+, Node.js 18+, [Wails v2](https://wails.io/docs/gettingstarted/installation)

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@v2.9.0

wails dev          # dev mode with hot reload
wails build        # production binary in build/bin/

go test ./...      # unit tests + SQLite integration tests
```

## AI configuration

In Settings, enter an API key, base URL and model. The model must support **tool/function calling**, for example `gpt-4o-mini`, `deepseek-chat`, or a tool-capable Ollama model such as `qwen2.5` with base URL `http://localhost:11434/v1` (no key needed).

## Data source

[CoinGecko](https://www.coingecko.com/) free public API, no key required. It is rate-limited to a few calls per minute, so the app caches responses.

> For information only. Not financial advice.
