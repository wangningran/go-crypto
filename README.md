# go-crypto

A cryptocurrency market tracker and AI analysis desktop app built with Go, Wails v2, and Vue 3.

Inspired by [go-stock](https://github.com/ArvinLovegood/go-stock).

## Features

- 📋 **Watchlist** — Track your favorite crypto assets
- 📊 **Market Overview** — Top 50 coins by market cap
- 🤖 **AI Analysis** — LLM-powered market analysis per coin
- 🔔 **Price Alerts** — High/low price notifications
- 📰 **News** — Latest crypto news from CoinGecko
- ⚙️ **Settings** — Configure AI provider (OpenAI-compatible)

## Data Source

Uses [CoinGecko](https://www.coingecko.com/) free API — no API key required for price data.

## Build

Requirements: Go 1.21+, Node.js 18+, [Wails v2](https://wails.io)

```bash
# Install Wails CLI
go install github.com/wailsapp/wails/v2/cmd/wails@latest

# Dev mode
wails dev

# Build production binary
wails build
```

## AI Configuration

In Settings, enter your OpenAI-compatible API key and base URL.
Supports: OpenAI, DeepSeek, Ollama (local), any OpenAI-compatible API.
