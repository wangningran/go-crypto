package indicators

import (
	"math"
	"testing"
	"time"
)

func near(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

func TestSMA(t *testing.T) {
	v := []float64{1, 2, 3, 4, 5}
	if got, ok := SMA(v, 3); !ok || got != 4 {
		t.Fatalf("SMA = %v,%v want 4", got, ok)
	}
	if _, ok := SMA(v, 6); ok {
		t.Fatal("SMA should fail with too little data")
	}
}

func TestRSI(t *testing.T) {
	up := make([]float64, 20)
	for i := range up {
		up[i] = float64(i + 1)
	}
	if got, _ := RSI(up, 14); got != 100 {
		t.Fatalf("RSI of strictly rising series = %v, want 100", got)
	}
	flat := make([]float64, 20)
	if got, _ := RSI(flat, 14); got != 50 {
		t.Fatalf("RSI of flat series = %v, want 50", got)
	}
	// Classic Wilder example (first 15 closes) -> ~70.46
	closes := []float64{44.34, 44.09, 44.15, 43.61, 44.33, 44.83, 45.10, 45.42, 45.84, 46.08, 45.89, 46.03, 45.61, 46.28, 46.28}
	if got, _ := RSI(closes, 14); !near(got, 70.46, 0.05) {
		t.Fatalf("RSI = %.2f, want ~70.46", got)
	}
	if _, ok := RSI(closes[:10], 14); ok {
		t.Fatal("RSI should fail with too little data")
	}
}

func TestPctChangeAndVol(t *testing.T) {
	v := []float64{100, 110, 121}
	if got, _ := PctChange(v, 2); !near(got, 21, 1e-9) {
		t.Fatalf("PctChange = %v", got)
	}
	vol, ok := DailyVolatility(v)
	if !ok || !near(vol, 0, 1e-9) {
		t.Fatalf("constant 10%% returns should have ~0 volatility, got %v", vol)
	}
}

func TestResampleDaily(t *testing.T) {
	d0 := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	pts := []Point{
		{d0.Add(26 * time.Hour), 3}, // day 2, later
		{d0.Add(1 * time.Hour), 1},
		{d0.Add(23 * time.Hour), 2}, // day 1 close
		{d0.Add(25 * time.Hour), 2.5},
	}
	got := ResampleDaily(pts)
	if len(got) != 2 || got[0] != 2 || got[1] != 3 {
		t.Fatalf("ResampleDaily = %v, want [2 3]", got)
	}
}

func TestSummarizeTrend(t *testing.T) {
	up := make([]float64, 31)
	for i := range up {
		up[i] = 100 + float64(i)
	}
	s := Summarize(up)
	if s.Trend != "up" || s.RangeHigh != 130 || s.RangeLow != 100 || s.Change30dPct == nil {
		t.Fatalf("unexpected snapshot: %+v", s)
	}
	short := Summarize([]float64{1, 2, 3})
	if short.Trend != "unknown" || short.SMA30 != nil {
		t.Fatalf("short series should have unknown trend and no SMA30: %+v", short)
	}
}
