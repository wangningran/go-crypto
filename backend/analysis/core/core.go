// Package core holds the pure logic of the AI analysis feature: prompts, the
// structured-output schema, validation, and how an analysis is scored.
// It has no I/O so it can be unit tested directly.
package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"go-crypto/backend/models"
)

// NeutralBandPct: a "neutral" call is correct if the 24h move stays within ±1%.
const NeutralBandPct = 1.0

const SystemPrompt = `You are a careful crypto market analyst working inside a desktop app.
You have tools that return real data. Rules:
- Always call get_price_history before forming a view. Use get_market_overview and get_news when they help.
- Every number you mention must come from a tool result. Never invent prices, levels or news.
- Support and resistance must be based on the returned 30-day range, moving averages or recent prices.
- Make a call for the NEXT 24 HOURS: bullish, bearish or neutral (expected move within ±1%).
- Confidence is 0 to 1. Use lower confidence when signals conflict or data is missing.
- Finish by calling submit_analysis. The summary is 120-250 words of plain text in English, structured as:
  Price action / Trend & indicators / Key levels / Sentiment & news / Risks.
- This is not financial advice; do not tell the user to buy or sell.`

// UserPrompt is the task for one coin.
func UserPrompt(name, symbol, coinID, currency string) string {
	return fmt.Sprintf("Analyze %s (%s, coin id %q). Quote prices in %s. Give your 24h outlook.",
		name, strings.ToUpper(symbol), coinID, strings.ToUpper(currency))
}

// Submission is what the model returns through the submit_analysis tool.
type Submission struct {
	Direction  string  `json:"direction"`
	Confidence float64 `json:"confidence"`
	Support    float64 `json:"support"`
	Resistance float64 `json:"resistance"`
	Summary    string  `json:"summary"`
}

// SubmissionSchema is the JSON Schema for submit_analysis.
func SubmissionSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"direction":  map[string]any{"type": "string", "enum": []string{"bullish", "bearish", "neutral"}, "description": "Expected direction over the next 24 hours"},
			"confidence": map[string]any{"type": "number", "minimum": 0, "maximum": 1},
			"support":    map[string]any{"type": "number", "description": "Nearest support level, in the quote currency"},
			"resistance": map[string]any{"type": "number", "description": "Nearest resistance level, in the quote currency"},
			"summary":    map[string]any{"type": "string", "description": "120-250 word analysis"},
		},
		"required": []string{"direction", "confidence", "support", "resistance", "summary"},
	}
}

// ParseSubmission validates and normalizes the model's output.
func ParseSubmission(raw json.RawMessage) (Submission, error) {
	var s Submission
	if err := json.Unmarshal(raw, &s); err != nil {
		return s, fmt.Errorf("invalid analysis JSON: %w", err)
	}
	s.Direction = strings.ToLower(strings.TrimSpace(s.Direction))
	switch s.Direction {
	case "bullish", "bearish", "neutral":
	case "up", "long":
		s.Direction = "bullish"
	case "down", "short":
		s.Direction = "bearish"
	default:
		return s, fmt.Errorf("invalid direction %q", s.Direction)
	}
	// Some models answer 0-100 instead of 0-1.
	if s.Confidence > 1 && s.Confidence <= 100 {
		s.Confidence /= 100
	}
	s.Confidence = math.Max(0, math.Min(1, s.Confidence))
	if s.Support < 0 || s.Resistance < 0 {
		return s, errors.New("support/resistance cannot be negative")
	}
	if s.Support > 0 && s.Resistance > 0 && s.Support > s.Resistance {
		s.Support, s.Resistance = s.Resistance, s.Support
	}
	s.Summary = strings.TrimSpace(s.Summary)
	if s.Summary == "" {
		return s, errors.New("empty summary")
	}
	return s, nil
}

// Judge scores a direction call given the price at analysis time and 24h later.
// bullish is correct if the price rose, bearish if it fell, neutral if the
// move stayed within ±NeutralBandPct.
func Judge(direction string, priceAt, priceAfter float64) (returnPct float64, correct bool, err error) {
	if priceAt <= 0 || priceAfter <= 0 {
		return 0, false, errors.New("missing price")
	}
	returnPct = (priceAfter - priceAt) / priceAt * 100
	switch direction {
	case "bullish":
		correct = returnPct > 0
	case "bearish":
		correct = returnPct < 0
	case "neutral":
		correct = math.Abs(returnPct) <= NeutralBandPct
	default:
		return returnPct, false, fmt.Errorf("unknown direction %q", direction)
	}
	return returnPct, correct, nil
}

// ComputeStats aggregates evaluated records.
func ComputeStats(recs []models.AnalysisRecord) models.EvalStats {
	st := models.EvalStats{Total: len(recs), NeutralBandPct: NeutralBandPct}
	byDir := map[string]*models.DirectionStats{}
	order := []string{"bullish", "bearish", "neutral"}
	for _, d := range order {
		byDir[d] = &models.DirectionStats{Direction: d}
	}
	var confOK, confBad float64
	var nBad int
	for _, r := range recs {
		if r.Correct == nil {
			if r.EvaluatedAt != nil {
				st.Unscorable++ // gave up fetching the +24h price
			} else {
				st.Pending++
			}
			continue
		}
		st.Evaluated++
		d := byDir[r.Direction]
		if d != nil {
			d.Evaluated++
		}
		if *r.Correct {
			st.Correct++
			confOK += r.Confidence
			if d != nil {
				d.Correct++
			}
		} else {
			nBad++
			confBad += r.Confidence
		}
	}
	if st.Evaluated > 0 {
		st.Accuracy = float64(st.Correct) / float64(st.Evaluated)
	}
	if st.Correct > 0 {
		st.AvgConfidenceCorrect = confOK / float64(st.Correct)
	}
	if nBad > 0 {
		st.AvgConfidenceWrong = confBad / float64(nBad)
	}
	for _, dname := range order {
		d := byDir[dname]
		if d.Evaluated > 0 {
			d.Accuracy = float64(d.Correct) / float64(d.Evaluated)
		}
		st.ByDirection = append(st.ByDirection, *d)
	}
	return st
}
