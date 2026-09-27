package core

import (
	"encoding/json"
	"math"
	"testing"
	"time"

	"go-crypto/backend/models"
)

func TestParseSubmission(t *testing.T) {
	s, err := ParseSubmission(json.RawMessage(`{"direction":" Up ","confidence":72,"support":120,"resistance":100,"summary":" ok "}`), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if s.Direction != "bullish" || math.Abs(s.Confidence-0.72) > 1e-9 || s.Support != 100 || s.Resistance != 120 || s.Summary != "ok" {
		t.Fatalf("not normalized: %+v", s)
	}
	for _, bad := range []string{
		`not json`,
		`{"direction":"moon","confidence":0.5,"summary":"x"}`,
		`{"direction":"neutral","confidence":0.5,"summary":""}`,
		`{"direction":"neutral","confidence":0.5,"support":-1,"summary":"x"}`,
	} {
		if _, err := ParseSubmission(json.RawMessage(bad), nil, nil); err == nil {
			t.Errorf("expected error for %s", bad)
		}
	}
}

func TestParseSubmissionSnapsLevelsToCandidates(t *testing.T) {
	// The model invents 95 and 130 instead of using the real computed levels.
	raw := json.RawMessage(`{"direction":"bullish","confidence":0.6,"support":95,"resistance":130,"summary":"x"}`)
	s, err := ParseSubmission(raw, []float64{90.5, 80}, []float64{125, 140})
	if err != nil {
		t.Fatal(err)
	}
	if s.Support != 90.5 {
		t.Fatalf("support should snap to nearest candidate 90.5, got %v", s.Support)
	}
	if s.Resistance != 125 {
		t.Fatalf("resistance should snap to nearest candidate 125, got %v", s.Resistance)
	}

	// No candidates (not enough history) -> value passes through unchanged.
	s2, err := ParseSubmission(json.RawMessage(`{"direction":"neutral","confidence":0.5,"support":95,"resistance":130,"summary":"x"}`), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if s2.Support != 95 || s2.Resistance != 130 {
		t.Fatalf("without candidates, levels should pass through unchanged: %+v", s2)
	}
}

func TestJudge(t *testing.T) {
	cases := []struct {
		dir       string
		at, after float64
		want      bool
	}{
		{"bullish", 100, 101, true},
		{"bullish", 100, 99, false},
		{"bearish", 100, 99, true},
		{"neutral", 100, 100.9, true},
		{"neutral", 100, 98.5, false},
	}
	for _, c := range cases {
		_, got, err := Judge(c.dir, c.at, c.after)
		if err != nil || got != c.want {
			t.Errorf("Judge(%s,%v,%v) = %v,%v want %v", c.dir, c.at, c.after, got, err, c.want)
		}
	}
	if _, _, err := Judge("bullish", 0, 1); err == nil {
		t.Error("expected error for missing price")
	}
}

func TestComputeStats(t *testing.T) {
	yes, no := true, false
	now := time.Now()
	recs := []models.AnalysisRecord{
		{Direction: "bullish", Confidence: 0.8, Correct: &yes},
		{Direction: "bullish", Confidence: 0.6, Correct: &no},
		{Direction: "bearish", Confidence: 0.4, Correct: &yes},
		{Direction: "neutral", Confidence: 0.5},                    // pending
		{Direction: "neutral", Confidence: 0.5, EvaluatedAt: &now}, // unscorable
	}
	st := ComputeStats(recs)
	if st.Total != 5 || st.Evaluated != 3 || st.Pending != 1 || st.Unscorable != 1 || st.Correct != 2 {
		t.Fatalf("counts wrong: %+v", st)
	}
	if math.Abs(st.Accuracy-2.0/3) > 1e-9 || math.Abs(st.AvgConfidenceCorrect-0.6) > 1e-9 || math.Abs(st.AvgConfidenceWrong-0.6) > 1e-9 {
		t.Fatalf("rates wrong: %+v", st)
	}
	if st.ByDirection[0].Direction != "bullish" || st.ByDirection[0].Accuracy != 0.5 || st.ByDirection[1].Accuracy != 1 {
		t.Fatalf("by-direction wrong: %+v", st.ByDirection)
	}
}
