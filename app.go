package main

import (
	"context"
	"fmt"
	"go-crypto/backend/data"
	"go-crypto/backend/db"
	"go-crypto/backend/logger"
	"go-crypto/backend/models"
	"sync"
	"time"

	"github.com/coocood/freecache"
	"github.com/robfig/cron/v3"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx           context.Context
	cgAPI         *data.CoinGeckoAPI
	cron          *cron.Cron
	cache         *freecache.Cache
	alertLastSent map[string]time.Time
	alertMu       sync.Mutex
}

func NewApp() *App {
	return &App{
		cgAPI:         data.NewCoinGeckoAPI(),
		cache:         freecache.NewCache(512 * 1024),
		alertLastSent: make(map[string]time.Time),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	db.Init()

	a.cron = cron.New(cron.WithSeconds())
	a.cron.AddFunc("@every 30s", func() {
		a.refreshWatchlistPrices()
		a.checkAlerts()
	})
	a.cron.Start()
	logger.Log.Info("go-crypto started")
}

func (a *App) shutdown(ctx context.Context) {
	if a.cron != nil {
		a.cron.Stop()
	}
}

func (a *App) refreshWatchlistPrices() {
	items := a.GetWatchlist()
	if len(items) == 0 {
		return
	}
	ids := make([]string, len(items))
	for i, item := range items {
		ids[i] = item.CoinID
	}
	s := data.GetSettings()
	prices, err := a.cgAPI.GetPrices(ids, s.Currency)
	if err != nil {
		logger.Log.Warnf("refresh prices error: %v", err)
		return
	}
	for _, item := range items {
		if p, ok := prices[item.CoinID]; ok {
			p.CoinID = item.CoinID
			p.Symbol = item.Symbol
			p.Name = item.Name
			db.DB.Where("coin_id = ?", item.CoinID).Assign(*p).FirstOrCreate(p)
		}
	}
	runtime.EventsEmit(a.ctx, "prices-updated", nil)
}

func (a *App) checkAlerts() {
	var alerts []models.PriceAlert
	db.DB.Where("enabled = ?", true).Find(&alerts)
	for _, alert := range alerts {
		var cp models.CachedPrice
		if err := db.DB.Where("coin_id = ?", alert.CoinID).First(&cp).Error; err != nil {
			continue
		}
		a.alertMu.Lock()
		last := a.alertLastSent[alert.CoinID]
		a.alertMu.Unlock()
		if time.Since(last) < 10*time.Minute {
			continue
		}
		triggered := false
		msg := ""
		if alert.HighPrice > 0 && cp.Price >= alert.HighPrice {
			triggered = true
			msg = "🔴 Price Alert: " + alert.Symbol + " reached high target $" + fmt.Sprintf("%.4f", alert.HighPrice)
		} else if alert.LowPrice > 0 && cp.Price <= alert.LowPrice {
			triggered = true
			msg = "🟢 Price Alert: " + alert.Symbol + " reached low target $" + fmt.Sprintf("%.4f", alert.LowPrice)
		}
		if triggered {
			a.alertMu.Lock()
			a.alertLastSent[alert.CoinID] = time.Now()
			a.alertMu.Unlock()
			runtime.EventsEmit(a.ctx, "price-alert", map[string]string{
				"coinId":  alert.CoinID,
				"symbol":  alert.Symbol,
				"message": msg,
			})
		}
	}
}

// --- Watchlist ---

func (a *App) GetWatchlist() []models.WatchlistItem {
	var items []models.WatchlistItem
	db.DB.Order("created_at asc").Find(&items)
	return items
}

func (a *App) AddToWatchlist(coinId, symbol, name string) error {
	item := models.WatchlistItem{CoinID: coinId, Symbol: symbol, Name: name}
	return db.DB.Where("coin_id = ?", coinId).FirstOrCreate(&item).Error
}

func (a *App) RemoveFromWatchlist(coinId string) error {
	return db.DB.Where("coin_id = ?", coinId).Delete(&models.WatchlistItem{}).Error
}

// --- Prices ---

func (a *App) GetWatchlistPrices() []models.CachedPrice {
	items := a.GetWatchlist()
	if len(items) == 0 {
		return []models.CachedPrice{}
	}
	ids := make([]string, len(items))
	for i, item := range items {
		ids[i] = item.CoinID
	}
	var prices []models.CachedPrice
	db.DB.Where("coin_id IN ?", ids).Find(&prices)
	return prices
}

func (a *App) GetTopCoins(limit int) []models.CachedPrice {
	s := data.GetSettings()
	coins, err := a.cgAPI.GetTopCoins(s.Currency, limit)
	if err != nil {
		logger.Log.Warnf("GetTopCoins error: %v", err)
		return []models.CachedPrice{}
	}
	return coins
}

func (a *App) SearchCoins(query string) []models.CoinSearchResult {
	results, err := a.cgAPI.SearchCoins(query)
	if err != nil {
		logger.Log.Warnf("SearchCoins error: %v", err)
		return []models.CoinSearchResult{}
	}
	return results
}

// --- Market Overview ---

func (a *App) GetMarketOverview() *models.MarketOverview {
	s := data.GetSettings()
	overview, err := a.cgAPI.GetMarketOverview(s.Currency)
	if err != nil {
		logger.Log.Warnf("GetMarketOverview error: %v", err)
		return &models.MarketOverview{}
	}
	return overview
}

// --- Alerts ---

func (a *App) GetAlerts() []models.PriceAlert {
	var alerts []models.PriceAlert
	db.DB.Order("created_at asc").Find(&alerts)
	return alerts
}

func (a *App) SetAlert(coinId, symbol string, highPrice, lowPrice float64) error {
	var alert models.PriceAlert
	db.DB.Where("coin_id = ?", coinId).First(&alert)
	alert.CoinID = coinId
	alert.Symbol = symbol
	alert.HighPrice = highPrice
	alert.LowPrice = lowPrice
	alert.Enabled = true
	if alert.ID == 0 {
		return db.DB.Create(&alert).Error
	}
	return db.DB.Save(&alert).Error
}

func (a *App) DeleteAlert(id uint) error {
	return db.DB.Delete(&models.PriceAlert{}, id).Error
}

// --- AI Analysis ---

func (a *App) AnalyzeCoin(coinId string) string {
	var cp models.CachedPrice
	if err := db.DB.Where("coin_id = ?", coinId).First(&cp).Error; err != nil {
		return "No price data available. Please add this coin to your watchlist first."
	}
	client := data.NewOpenAIClient()
	result, err := client.AnalyzeCoin(coinId, &cp)
	if err != nil {
		return "Analysis error: " + err.Error()
	}
	return result
}

// --- News ---

func (a *App) GetNews() []models.NewsItem {
	news, err := data.GetCryptoNews()
	if err != nil {
		logger.Log.Warnf("GetNews error: %v", err)
		return []models.NewsItem{}
	}
	return news
}

// --- Settings ---

func (a *App) GetSettings() *models.Settings {
	return data.GetSettings()
}

func (a *App) SaveSettings(s models.Settings) error {
	return data.SaveSettings(s)
}
