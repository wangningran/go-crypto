package db

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"go-crypto/backend/models"
)

// Concurrent writers (price refresh, evaluator, UI) must not fail with
// "database is locked": WAL + busy_timeout make them wait instead.
func TestConcurrentWrites(t *testing.T) {
	if err := Open(filepath.Join(t.TempDir(), "test.db")); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 1000)
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				p := models.CachedPrice{CoinID: fmt.Sprintf("coin-%d-%d", w, i), Currency: "usd", Price: float64(i)}
				if err := DB.Create(&p).Error; err != nil {
					errs <- err
					return
				}
				p.Price++
				if err := DB.Save(&p).Error; err != nil {
					errs <- err
					return
				}
				var n int64
				if err := DB.Model(&models.CachedPrice{}).Count(&n).Error; err != nil {
					errs <- err
					return
				}
			}
		}(w)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent write failed: %v", err)
	}
	var n int64
	DB.Model(&models.CachedPrice{}).Count(&n)
	if n != 400 {
		t.Fatalf("rows = %d, want 400", n)
	}
}

// Times are stored in UTC so text comparisons in SQLite are correct.
func TestTimesStoredUTCAndComparable(t *testing.T) {
	if err := Open(filepath.Join(t.TempDir(), "t.db")); err != nil {
		t.Fatal(err)
	}
	old := models.AnalysisRecord{CoinID: "a", Direction: "neutral", Summary: "x"}
	DB.Create(&old)
	DB.Model(&old).Update("created_at", time.Now().UTC().Add(-48*time.Hour))
	DB.Create(&models.AnalysisRecord{CoinID: "b", Direction: "neutral", Summary: "x"})

	var due []models.AnalysisRecord
	DB.Where("created_at <= ?", time.Now().UTC().Add(-24*time.Hour)).Find(&due)
	if len(due) != 1 || due[0].CoinID != "a" {
		t.Fatalf("due = %+v, want only the 48h-old record", due)
	}
}
