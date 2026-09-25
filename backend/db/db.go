package db

import (
	"go-crypto/backend/logger"
	"go-crypto/backend/models"
	"os"
	"path/filepath"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
	_ "modernc.org/sqlite"
)

var DB *gorm.DB

func Init() {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".go-crypto")
	_ = os.MkdirAll(dir, 0755)
	dbPath := filepath.Join(dir, "go-crypto.db")

	var err error
	DB, err = gorm.Open(sqlite.Dialector{DSN: dbPath}, &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		logger.Log.Fatalf("failed to open db: %v", err)
	}
	DB.AutoMigrate(
		&models.WatchlistItem{},
		&models.PriceAlert{},
		&models.Settings{},
		&models.CachedPrice{},
	)
}
