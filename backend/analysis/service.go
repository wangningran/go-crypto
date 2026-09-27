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

// MaxPriceAge: if the cached price is older than this (e.g. refreshes have
// been failing), Analyze fetches a fresh one first. The price at analysis time
// is the baseline the 24h score is measured from, so it must be current.
const MaxPriceAge = 5 * time.Minute

// maxEvalAttempts: after this many failed tries (one per evaluator run, every
// 30 min) an analysis is marked unscorable so it stops blocking the queue.
const maxEvalAttempts = 6

// storedResultChars caps each tool result kept in the database.
const storedResultChars = 1500

type Service struct {
	CG *data.CoinGeckoAPI
	// priceAt returns the historical price at time t (overridable in tests).
	priceAt func(coinID, currency string, t time.Time) (float64, error)
	// evalPause is the delay between evaluator API calls (rate limit).
	evalPause time.Duration

	mu      sync.Mutex
	running map[string]context.CancelFunc // coinID -> cancel of the running analysis
	evalMu  sync.Mutex
}

func NewService(cg *data.CoinGeckoAPI) *Service {
	return &Service{CG: cg, priceAt: cg.GetPriceAt, evalPause: 3 * time.Second, running: map[string]context.CancelFunc{}}
}

// Cancel stops a running analysis for coinID (e.g. the user closed the panel),
// so no more LLM tokens are spent on it.
func (s *Service) Cancel(coinID string) {
	s.mu.Lock()
	if cancel, ok := s.running[coinID]; ok {
		cancel()
	}
	s.mu.Unlock()
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

	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	s.mu.Lock()
	if _, busy := s.running[coinID]; busy {
		s.mu.Unlock()
		return nil, errors.New("an analysis for this coin is already running")
	}
	s.running[coinID] = cancel
	s.mu.Unlock()
	defer func() { s.mu.Lock(); delete(s.running, coinID); s.mu.Unlock() }()

	if time.Since(cp.UpdatedAt) > MaxPriceAge {
		fresh, err := s.freshPrice(cp, cur)
		if err != nil {
			return nil, fmt.Errorf("the saved price is %s old and refreshing it failed: %w", time.Since(cp.UpdatedAt).Round(time.Minute), err)
		}
		cp = fresh
	}

	key, err := data.GetAPIKey()
	if err != nil {
		return nil, fmt.Errorf("could not read the API key from the system keychain: %w", err)
	}
	if strings.TrimSpace(key) == "" && !isLocal(settings.OpenAIBase) {
		return nil, errors.New("please add your LLM API key in Settings to enable AI analysis")
	}

	// levels is filled in by the get_price_history tool (if the model calls
	// it, which the system prompt requires) with the real computed
	// support/resistance candidates for this coin, so the submitted analysis
	// can be snapped to one of them instead of trusting a free-form number.
	var levels indicators.Snapshot
	runner := &agent.Runner{
		Client:    &agent.Client{BaseURL: settings.OpenAIBase, APIKey: key, Model: settings.OpenAIModel},
		Tools:     s.tools(cp, cur, &levels),
		FinalTool: agent.Tool{Name: "submit_analysis", Description: "Submit the final 24h analysis. Call exactly once, at the end.", Parameters: core.SubmissionSchema()},
		MaxSteps:  6,
		OnStep: func(st agent.Step) {
			if onStep != nil {
				onStep(toModelStep(st))
			}
		},
	}

	res, err := runner.Run(ctx, core.SystemPrompt, core.UserPrompt(cp.Name, cp.Symbol, cp.CoinID, cur))
	if errors.Is(err, context.Canceled) {
		return nil, errors.New("analysis cancelled")
	}
	if err != nil {
		return nil, err
	}
	sub, err := core.ParseSubmission(res.Final, levels.SupportLevels, levels.ResistanceLevels)
	if err != nil {
		return nil, err
	}

	steps := make([]models.AgentStep, 0, len(res.Steps))
	for _, st := range res.Steps {
		steps = append(steps, toModelStep(st))
	}
	stored := make([]models.AgentStep, len(steps))
	for i, st := range steps {
		if len(st.Result) > storedResultChars {
			st.Result = st.Result[:storedResultChars] + "…"
		}
		stored[i] = st
	}
	stepsJSON, _ := json.Marshal(stored)

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

// freshPrice fetches the current price for one coin and updates the cache table.
func (s *Service) freshPrice(cp models.CachedPrice, cur string) (models.CachedPrice, error) {
	prices, err := s.CG.GetPricesForWatchlist([]string{cp.CoinID}, cur)
	if err != nil {
		return cp, err
	}
	p, ok := prices[cp.CoinID]
	if !ok {
		return cp, errors.New("coin not returned by CoinGecko")
	}
	p.ID = cp.ID
	if err := db.DB.Save(&p).Error; err != nil {
		logger.Log.Warnf("save fresh price: %v", err)
	}
	return p, nil
}

func isLocal(base string) bool {
	return strings.Contains(base, "localhost") || strings.Contains(base, "127.0.0.1")
}

func toModelStep(st agent.Step) models.AgentStep {
	return models.AgentStep{Index: st.Index, Tool: st.Tool, Args: st.Args, Result: st.Result, Error: st.Error, DurationMs: st.DurationMs}
}

// tools the agent may call. Numbers are computed here, in code; the model only interprets them.
// tools returns the agent's tools for one analysis run. levelsOut is filled
// in by get_price_history's Run func with the computed indicator snapshot
// (including support/resistance candidates), so the caller can read it back
// after the agent loop finishes and use it to validate the final submission.
func (s *Service) tools(cp models.CachedPrice, cur string, levelsOut *indicators.Snapshot) []agent.Tool {
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
			Description: "Technical indicators computed from daily closes: 7d/30d % change, SMA7, SMA30, RSI14, daily volatility, 30-day high/low, rule-based trend, support/resistance candidates, and the last 10 daily closes.",
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
				snap := indicators.Summarize(daily)
				if levelsOut != nil {
					*levelsOut = snap
				}
				return map[string]any{
					"currency":        cur,
					"indicators":      snap,
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

// EvaluatePending scores analyses that are at least 24h old. It handles at
// most max per run, pauses between API calls, and stops early on a rate limit.
// Records that keep failing are retried on later runs and, after
// maxEvalAttempts, marked unscorable so newer records aren't blocked behind them.
// Returns the number of records scored.
func (s *Service) EvaluatePending(max int) int {
	if !s.evalMu.TryLock() {
		return 0 // another run (cron or the "Score now" button) is in progress
	}
	defer s.evalMu.Unlock()

	var recs []models.AnalysisRecord
	db.DB.Omit("steps_json").
		Where("evaluated_at IS NULL AND created_at <= ?", time.Now().UTC().Add(-24*time.Hour)).
		Order("eval_attempts asc, created_at asc").Limit(max).Find(&recs)

	scored := 0
	for i, r := range recs {
		if i > 0 && s.evalPause > 0 {
			time.Sleep(s.evalPause)
		}
		price, err := s.priceAt(r.CoinID, r.Currency, r.CreatedAt.Add(24*time.Hour))
		var ret float64
		var ok bool
		if err == nil {
			ret, ok, err = core.Judge(r.Direction, r.PriceAtAnalysis, price)
		}
		if err != nil {
			logger.Log.Warnf("evaluate analysis %d (attempt %d): %v", r.ID, r.EvalAttempts+1, err)
			upd := models.AnalysisRecord{EvalAttempts: r.EvalAttempts + 1, EvalError: truncate(err.Error(), 300)}
			fields := []any{"EvalAttempts", "EvalError"}
			if upd.EvalAttempts >= maxEvalAttempts {
				now := time.Now().UTC()
				upd.EvaluatedAt = &now // give up: unscorable (Correct stays nil)
				fields = append(fields, "EvaluatedAt")
			}
			if dbErr := db.DB.Model(&models.AnalysisRecord{ID: r.ID}).Select(fields[0], fields[1:]...).Updates(upd).Error; dbErr != nil {
				logger.Log.Warnf("save eval attempt %d: %v", r.ID, dbErr)
			}
			if data.IsRateLimited(err) {
				return scored // CoinGecko is cooling down: try again next run
			}
			continue
		}
		now := time.Now().UTC()
		// Select by field name so zero values (e.g. correct=false) are written too.
		if dbErr := db.DB.Model(&models.AnalysisRecord{ID: r.ID}).
			Select("EvaluatedAt", "PriceAfter24h", "ReturnPct", "Correct", "EvalAttempts", "EvalError").
			Updates(models.AnalysisRecord{EvaluatedAt: &now, PriceAfter24h: price, ReturnPct: ret, Correct: &ok, EvalAttempts: r.EvalAttempts + 1}).Error; dbErr != nil {
			logger.Log.Warnf("save evaluation %d: %v", r.ID, dbErr)
			continue
		}
		scored++
	}
	return scored
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// History returns the most recent analyses, newest first. Agent steps are
// left out: with tool results they can be ~10 KB per record, and the history
// table doesn't show them.
func History(limit int) []models.AnalysisRecord {
	var recs []models.AnalysisRecord
	db.DB.Omit("steps_json").Order("created_at desc").Limit(limit).Find(&recs)
	return recs
}

// Stats aggregates the track record over all stored analyses.
func Stats() models.EvalStats {
	var recs []models.AnalysisRecord
	db.DB.Select("direction", "confidence", "correct", "evaluated_at").Find(&recs)
	return core.ComputeStats(recs)
}
