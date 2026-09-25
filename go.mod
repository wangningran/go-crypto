module go-crypto

go 1.21

replace github.com/mattn/go-sqlite3 => ./third_party/mattn-sqlite3-stub

require (
	github.com/coocood/freecache v1.2.4
	github.com/go-resty/resty/v2 v2.12.0
	github.com/google/uuid v1.6.0
	github.com/robfig/cron/v3 v3.0.1
	github.com/wailsapp/wails/v2 v2.9.0
	go.uber.org/zap v1.27.0
	gorm.io/driver/sqlite v1.5.5
	gorm.io/gorm v1.25.7
	modernc.org/sqlite v1.29.1
)
