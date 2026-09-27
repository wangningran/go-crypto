package analysis

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"go-crypto/backend/db"
	"go-crypto/backend/models"
)

func setup(t *testing.T) *Service {
	t.Helper()
	if err := db.Open(filepath.Join(t.TempDir(), "a.db")); err != nil {
		t.Fatal(err)
	}
	return &Service{running: map[string]context.CancelFunc{}}
}

func addRecord(t *testing.T, coin, dir string, price float64, age time.Duration) uint {
	t.Helper()
	r := models.AnalysisRecord{CoinID: coin, Currency: "usd", Direction: dir, Confidence: 0.6, Summary: "s",
		PriceAtAnalysis: price, StepsJSON: `[{"tool":"get_news","result":"x"}]`}
	if err := db.DB.Create(&r).Error; err != nil {
		t.Fatal(err)
	}
	db.DB.Model(&r).Update("created_at", time.Now().UTC().Add(-age))
	return r.ID
}

func get(t *testing.T, id uint) models.AnalysisRecord {
	t.Helper()
	var r models.AnalysisRecord
	if err := db.DB.First(&r, id).Error; err != nil {
		t.Fatal(err)
	}
	return r
}

func TestEvaluatePendingScoresAndPersistsFalse(t *testing.T) {
	s := setup(t)
	s.priceAt = func(coin, cur string, at time.Time) (float64, error) { return 90, nil } // price fell 10%
	bull := addRecord(t, "btc", "bullish", 100, 25*time.Hour)
	bear := addRecord(t, "eth", "bearish", 100, 25*time.Hour)
	fresh := addRecord(t, "sol", "bullish", 100, time.Hour) // not due yet

	if n := s.EvaluatePending(10); n != 2 {
		t.Fatalf("scored %d, want 2", n)
	}
	b := get(t, bull)
	if b.Correct == nil || *b.Correct || b.EvaluatedAt == nil || b.PriceAfter24h != 90 {
		t.Fatalf("bullish call on a falling price should be stored as correct=false: %+v", b)
	}
	if e := get(t, bear); e.Correct == nil || !*e.Correct {
		t.Fatalf("bearish call on a falling price should be correct: %+v", e)
	}
	if f := get(t, fresh); f.EvaluatedAt != nil {
		t.Fatal("record younger than 24h must not be scored")
	}
	st := Stats()
	if st.Evaluated != 2 || st.Correct != 1 || st.Pending != 1 {
		t.Fatalf("stats = %+v", st)
	}
}

func TestEvaluatePendingFailingRecordsDontBlockQueue(t *testing.T) {
	s := setup(t)
	s.priceAt = func(coin, cur string, at time.Time) (float64, error) {
		if coin == "dead" {
			return 0, errors.New("coin not found")
		}
		return 110, nil
	}
	// 3 old failing records, then 1 newer good one; each run handles 3.
	var dead []uint
	for i := 0; i < 3; i++ {
		dead = append(dead, addRecord(t, "dead", "bullish", 100, 72*time.Hour))
	}
	good := addRecord(t, "btc", "bullish", 100, 30*time.Hour)

	s.EvaluatePending(3) // all 3 dead records fail once
	s.EvaluatePending(3) // failed ones now sort after the untried good record
	if g := get(t, good); g.Correct == nil || !*g.Correct {
		t.Fatalf("good record should be scored despite failing ones ahead of it: %+v", g)
	}
	for i := 0; i < maxEvalAttempts; i++ {
		s.EvaluatePending(3)
	}
	for _, id := range dead {
		r := get(t, id)
		if r.EvaluatedAt == nil || r.Correct != nil || r.EvalAttempts != maxEvalAttempts || r.EvalError == "" {
			t.Fatalf("failing record should end unscorable after %d attempts: %+v", maxEvalAttempts, r)
		}
	}
	if st := Stats(); st.Unscorable != 3 || st.Pending != 0 || st.Evaluated != 1 {
		t.Fatalf("stats = %+v", st)
	}
}

func TestHistoryOmitsSteps(t *testing.T) {
	setup(t)
	addRecord(t, "btc", "bullish", 100, time.Hour)
	h := History(10)
	if len(h) != 1 || h[0].StepsJSON != "" || h[0].Summary != "s" {
		t.Fatalf("history should include the record without steps: %+v", h)
	}
}
