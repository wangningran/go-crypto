// Package analysis wires the agent loop to real market data, stores every
// analysis, and scores it against the actual price 24h later.
package analysis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"go-crypto/backend/agent"
	"go-crypto/backend/analysis/core"
	"go-crypto/backend/data"
	"go-crypto/backend/db"
	"go-crypto/backend/indicators"
	"go-crypto/backend/logger"
	"go-crypto/backend/models"
)

// CacheTTL: asking about the same coin again within this window returns the
// stored analysis instead of paying for another LLM run.
const CacheTTL = 5 * time.Minute

type Service struct {
	CG *data.CoinGeckoAPI

	mu       sync.Mutex
	inFlight map[string]bool
}

func NewService(cg *data.CoinGeckoAPI) *Service {
	return &Service{CG: cg, inFlight: map[string]bool{}}
}

// Analyze runs (or returns a cached) analysis for a watchlist coin.
// onStep is called live for each tool the agent uses.
func (s *Service) Analyze(ctx context.Context, coinID string, onStep func(models.AgentStep)) (*models.AnalysisRecord, error) {
	settings := data.GetSettings()
	cur := settings.Currency

	var cp models.CachedPrice
	if err := db.DB.Where("coin_id = ? AND currency = ?", coinID, cur).First(&cp).Error; err != nil {
		return nil, errors.New("no price data yet for this coin; wait for the next refresh and try again")
	}

	// 1. Cache: same coin, same currency, same model, within CacheTTL.
	var recent models.AnalysisRecord
	err := db.DB.Where("coin_id = ? AND currency = ? AND model = ? AND created_at > ?", coinID, cur, settings.OpenAIModel, time.Now().UTC().Add(-CacheTTL)).
		Order("created_at desc").First(&recent).Error
	if err == nil {
		recent.Cached = true
		_ = json.Unmarshal([]byte(recent.StepsJSON), &recent.Steps)
		return &recent, nil
	}

	s.mu.Lock()
	if s.inFlight[coinID] {
		s.mu.Unlock()
		return nil, errors.New("an analysis for this coin is already running")
	}
	s.inFlight[coinID] = true
	s.mu.Unlock()
	defer func() { s.mu.Lock(); delete(s.inFlight, coinID); s.mu.Unlock() }()

	key, err := data.GetAPIKey()
	if err != nil {
		return nil, fmt.Errorf("could not read the API key from the system keychain: %w", err)
	}
	if strings.TrimSpace(key) == "" && !isLocal(settings.OpenAIBase) {
		return nil, errors.New("please add your LLM API key in Settings to enable AI analysis")
	}

	runner := &agent.Runner{
		Client:    &agent.Client{BaseURL: settings.OpenAIBase, APIKey: key, Model: settings.OpenAIModel},
		Tools:     s.tools(cp, cur),
		FinalTool: agent.Tool{Name: "submit_analysis", Description: "Submit the final 24h analysis. Call exactly once, at the end.", Parameters: core.SubmissionSchema()},
		MaxSteps:  6,
		OnStep: func(st agent.Step) {
			if onStep != nil {
				onStep(toModelStep(st))
			}
		},
	}

	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	res, err := runner.Run(ctx, core.SystemPrompt, core.UserPrompt(cp.Name, cp.Symbol, cp.CoinID, cur))
	if err != nil {
		return nil, err
	}
	sub, err := core.ParseSubmission(res.Final)
	if err != nil {
		return nil, err
	}

	steps := make([]models.AgentStep, 0, len(res.Steps))
	for _, st := range res.Steps {
		steps = append(steps, toModelStep(st))
	}
	stepsJSON, _ := json.Marshal(steps)

	rec := &models.AnalysisRecord{
		CoinID:           cp.CoinID,
		Symbol:           cp.Symbol,
		Name:             cp.Name,
		Currency:         cur,
		Model:            settings.OpenAIModel,
		Direction:        sub.Direction,
		Confidence:       sub.Confidence,
		Support:          sub.Support,
		Resistance:       sub.Resistance,
		Summary:          sub.Summary,
		PriceAtAnalysis:  cp.Price,
		StepsJSON:        string(stepsJSON),
		Steps:            steps,
		PromptTokens:     res.Usage.PromptTokens,
		CompletionTokens: res.Usage.CompletionTokens,
	}
	if err := db.DB.Create(rec).Error; err != nil {
		logger.Log.Warnf("save analysis: %v", err)
	}
	return rec, nil
}

func isLocal(base string) bool {
	return strings.Contains(base, "localhost") || strings.Contains(base, "127.0.0.1")
}

func toModelStep(st agent.Step) models.AgentStep {
	return models.AgentStep{Index: st.Index, Tool: st.Tool, Args: st.Args, Result: st.Result, Error: st.Error, DurationMs: st.DurationMs}
}

// tools the agent may call. Numbers are computed here, in code; the model only interprets them.
func (s *Service) tools(cp models.CachedPrice, cur string) []agent.Tool {
	noArgs := map[string]any{"type": "object", "properties": map[string]any{}}
	return []agent.Tool{
		{
			Name:        "get_price_snapshot",
			Description: "Current price, 24h change, 24h high/low, market cap and volume for the coin being analyzed.",
			Parameters:  noArgs,
			Run: func(ctx context.Context, _ json.RawMessage) (any, error) {
				return map[string]any{
					"coin": cp.Name, "symbol": cp.Symbol, "currency": cur,
					"price": cp.Price, "change24hPct": cp.Change24h,
					"high24h": cp.High24h, "low24h": cp.Low24h,
					"marketCap": cp.MarketCap, "volume24h": cp.Volume24h,
					"asOf": cp.UpdatedAt.Format(time.RFC3339),
				}, nil
			},
		},
		{
			Name:        "get_price_history",
			Description: "Technical indicators computed from daily closes: 7d/30d % change, SMA7, SMA30, RSI14, daily volatility, 30-day high/low, rule-based trend, and the last 10 daily closes.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"days": map[string]any{"type": "integer", "description": "History window in days (7-90). Default 30.", "minimum": 7, "maximum": 90},
				},
			},
			Run: func(ctx context.Context, args json.RawMessage) (any, error) {
				var a struct {
					Days int `json:"days"`
				}
				_ = json.Unmarshal(args, &a)
				if a.Days < 7 || a.Days > 90 {
					a.Days = 30
				}
				// Ask for one extra day so a full 30-day change can be computed.
				pts, err := s.CG.GetPriceHistory(cp.CoinID, cur, a.Days+1)
				if err != nil {
					return nil, err
				}
				daily := indicators.ResampleDaily(pts)
				recent := daily
				if len(recent) > 10 {
					recent = recent[len(recent)-10:]
				}
				return map[string]any{
					"currency":        cur,
					"indicators":      indicators.Summarize(daily),
					"lastDailyCloses": recent,
				}, nil
			},
		},
		{
			Name:        "get_market_overview",
			Description: "Whole crypto market: total market cap, 24h market cap change, BTC and ETH dominance.",
			Parameters:  noArgs,
			Run: func(ctx context.Context, _ json.RawMessage) (any, error) {
				return s.CG.GetMarketOverview(cur)
			},
		},
		{
			Name:        "get_news",
			Description: "Recent crypto news headlines. Returns headlines mentioning this coin, or general market headlines if none mention it.",
			Parameters:  noArgs,
			Run: func(ctx context.Context, _ json.RawMessage) (any, error) {
				items, err := data.GetCryptoNews()
				if err != nil {
					return nil, err
				}
				related := data.FilterNews(items, cp.Name, cp.Symbol, 8)
				scope := "coin"
				if len(related) == 0 {
					scope = "general"
					if len(items) > 8 {
						items = items[:8]
					}
					related = items
				}
				type headline struct {
					Title, Source, PublishedAt string
				}
				out := make([]headline, 0, len(related))
				for _, n := range related {
					out = append(out, headline{n.Title, n.Source, n.PublishedAt})
				}
				return map[string]any{"scope": scope, "headlines": out}, nil
			},
		},
	}
}

// EvaluatePending scores analyses that are at least 24h old. It handles a few
// per run to stay within CoinGecko's free rate limit.
func (s *Service) EvaluatePending(max int) {
	var recs []models.AnalysisRecord
	db.DB.Where("evaluated_at IS NULL AND created_at <= ?", time.Now().UTC().Add(-24*time.Hour)).
		Order("created_at asc").Limit(max).Find(&recs)
	for _, r := range recs {
		target := r.CreatedAt.Add(24 * time.Hour)
		price, err := s.CG.GetPriceAt(r.CoinID, r.Currency, target)
		if err != nil {
			logger.Log.Warnf("evaluate analysis %d: %v", r.ID, err)
			continue
		}
		ret, ok, err := core.Judge(r.Direction, r.PriceAtAnalysis, price)
		if err != nil {
			logger.Log.Warnf("evaluate analysis %d: %v", r.ID, err)
			continue
		}
		now := time.Now().UTC()
		// Select by field name so zero values (e.g. correct=false) are written too.
		db.DB.Model(&models.AnalysisRecord{ID: r.ID}).
			Select("EvaluatedAt", "PriceAfter24h", "ReturnPct", "Correct").
			Updates(models.AnalysisRecord{EvaluatedAt: &now, PriceAfter24h: price, ReturnPct: ret, Correct: &ok})
	}
}

// History returns the most recent analyses, newest first.
func History(limit int) []models.AnalysisRecord {
	var recs []models.AnalysisRecord
	db.DB.Order("created_at desc").Limit(limit).Find(&recs)
	for i := range recs {
		_ = json.Unmarshal([]byte(recs[i].StepsJSON), &recs[i].Steps)
	}
	return recs
}

// Stats aggregates the track record over all stored analyses.
func Stats() models.EvalStats {
	var recs []models.AnalysisRecord
	db.DB.Select("direction", "confidence", "correct").Find(&recs)
	return core.ComputeStats(recs)
}
