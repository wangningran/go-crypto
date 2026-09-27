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

// Init opens ~/.go-crypto/go-crypto.db, exiting the app if that fails.
func Init() {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".go-crypto")
	_ = os.MkdirAll(dir, 0o755)
	dbPath := filepath.Join(dir, "go-crypto.db")
	if err := Open(dbPath); err != nil {
		logger.Log.Fatalf("failed to open db at %s: %v", dbPath, err)
	}
}

// Open opens (and migrates) the database at path. Used directly by tests.
func Open(path string) error {
	// Several goroutines write at once (price refresh, evaluator, UI actions).
	// WAL lets reads run during a write, and busy_timeout makes a writer wait
	// up to 5s for the lock instead of failing immediately with SQLITE_BUSY.
	dsn := path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)"
	conn, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
		// SQLite stores times as text; keeping everything in UTC makes
		// "created_at > ?" comparisons correct across DST changes.
		NowFunc: func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		return err
	}
	if err := conn.AutoMigrate(
		&models.WatchlistItem{},
		&models.PriceAlert{},
		&models.Settings{},
		&models.CachedPrice{},
		&models.AnalysisRecord{},
	); err != nil {
		return err
	}
	DB = conn
	return nil
}
