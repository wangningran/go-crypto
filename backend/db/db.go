package db

import (
	"os"
	"path/filepath"
	"time"

	"go-crypto/backend/logger"
	"go-crypto/backend/models"

	// Pure-Go SQLite driver for GORM (built on modernc.org/sqlite): no cgo,
	// so the app cross-compiles without a C toolchain.
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

var DB *gorm.DB

func Init() {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".go-crypto")
	_ = os.MkdirAll(dir, 0o755)
	dbPath := filepath.Join(dir, "go-crypto.db")

	var err error
	DB, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
		// SQLite stores times as text; keeping everything in UTC makes
		// "created_at > ?" comparisons correct across DST changes.
		NowFunc: func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		logger.Log.Fatalf("failed to open db at %s: %v", dbPath, err)
	}
	if err := DB.AutoMigrate(
		&models.WatchlistItem{},
		&models.PriceAlert{},
		&models.Settings{},
		&models.CachedPrice{},
		&models.AnalysisRecord{},
	); err != nil {
		logger.Log.Fatalf("failed to migrate db: %v", err)
	}
}
