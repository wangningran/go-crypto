package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"go-crypto/backend/analysis"
	"go-crypto/backend/data"
	"go-crypto/backend/db"
	"go-crypto/backend/logger"
	"go-crypto/backend/models"

	"github.com/robfig/cron/v3"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"gorm.io/gorm"
)

type App struct {
	ctx            context.Context
	cgAPI          *data.CoinGeckoAPI
	analysis       *analysis.Service
	cron           *cron.Cron
	refreshEntryID cron.EntryID
	schedMu        sync.Mutex // guards refreshEntryID
	refreshMu      sync.Mutex // serializes price refreshes
}

func NewApp() *App {
	cg := data.NewCoinGeckoAPI()
	return &App{cgAPI: cg, analysis: analysis.NewService(cg)}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	db.Init()
	data.MigrateLegacyAPIKey()

	// SkipIfStillRunning: if a refresh is slow (e.g. network timeouts), the next
	// tick is skipped instead of queuing up goroutines behind it.
	a.cron = cron.New(cron.WithSeconds(), cron.WithChain(cron.SkipIfStillRunning(cron.DiscardLogger)))
	a.cron.Start()

	s := data.GetSettings()
	// Data saved by older versions has no currency: tag alerts with the current
	// currency and drop cached prices (they are re-fetched on the next refresh).
	db.DB.Model(&models.PriceAlert{}).Where("currency = '' OR currency IS NULL").Update("currency", s.Currency)
	db.DB.Where("currency = '' OR currency IS NULL").Delete(&models.CachedPrice{})

	a.scheduleRefresh(s.RefreshSecs)
	// Score analyses that are 24h old. A few per run keeps us under the free API rate limit.
	_, _ = a.cron.AddFunc("@every 30m", func() { a.analysis.EvaluatePending(5) })
	go func() {
		a.refreshNow()
		a.analysis.EvaluatePending(5)
	}()
	logger.Log.Info("go-crypto started")
}

func (a *App) scheduleRefresh(secs int) {
	secs = data.ClampRefreshSecs(secs)
	a.schedMu.Lock()
	defer a.schedMu.Unlock()
	if a.refreshEntryID != 0 {
		a.cron.Remove(a.refreshEntryID)
	}
	id, err := a.cron.AddFunc(fmt.Sprintf("@every %ds", secs), a.refreshNow)
	if err != nil {
		logger.Log.Warnf("schedule refresh: %v", err)
		return
	}
	a.refreshEntryID = id
}

func (a *App) shutdown(ctx context.Context) {
	if a.cron != nil {
		a.cron.Stop()
	}
}

// refreshNow updates watchlist prices and then checks alerts. Runs are serialized.
func (a *App) refreshNow() {
	a.refreshMu.Lock()
	defer a.refreshMu.Unlock()
	a.refreshWatchlistPrices()
	a.checkAlerts()
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
	cur := data.GetSettings().Currency
	prices, err := a.cgAPI.GetPricesForWatchlist(ids, cur)
	if err != nil {
		logger.Log.Warnf("refresh prices error: %v", err)
		runtime.EventsEmit(a.ctx, "data-error", err.Error())
		return
	}
	for _, item := range items {
		p, ok := prices[item.CoinID]
		if !ok {
			continue
		}
		var existing models.CachedPrice
		if db.DB.Where("coin_id = ?", item.CoinID).First(&existing).Error == nil {
			p.ID = existing.ID
		}
		if err := db.DB.Save(&p).Error; err != nil {
			logger.Log.Warnf("save price %s: %v", item.CoinID, err)
		}
	}
	runtime.EventsEmit(a.ctx, "prices-updated", nil)
}

// checkAlerts fires each alert once, then pauses it until the user resumes it.
func (a *App) checkAlerts() {
	var alerts []models.PriceAlert
	db.DB.Where("enabled = ?", true).Find(&alerts)
	for _, alert := range alerts {
		var cp models.CachedPrice
		if err := db.DB.Where("coin_id = ?", alert.CoinID).First(&cp).Error; err != nil {
			continue
		}
		// Never compare a EUR threshold with a USD price.
		if !strings.EqualFold(cp.Currency, alert.Currency) {
			continue
		}
		msg := ""
		switch {
		case alert.HighPrice > 0 && cp.Price >= alert.HighPrice:
			msg = fmt.Sprintf("%s reached your high target %s (now %s)", alert.Symbol, formatMoney(alert.HighPrice, alert.Currency), formatMoney(cp.Price, cp.Currency))
		case alert.LowPrice > 0 && cp.Price <= alert.LowPrice:
			msg = fmt.Sprintf("%s fell to your low target %s (now %s)", alert.Symbol, formatMoney(alert.LowPrice, alert.Currency), formatMoney(cp.Price, cp.Currency))
		default:
			continue
		}
		now := time.Now().UTC()
		if err := db.DB.Model(&models.PriceAlert{ID: alert.ID}).
			Select("Enabled", "TriggeredAt", "LastMessage").
			Updates(models.PriceAlert{Enabled: false, TriggeredAt: &now, LastMessage: msg}).Error; err != nil {
			// Don't notify if we couldn't pause it, or it would fire again next tick.
			logger.Log.Warnf("pause alert %d: %v", alert.ID, err)
			continue
		}
		runtime.EventsEmit(a.ctx, "price-alert", map[string]string{
			"coinId":  alert.CoinID,
			"symbol":  alert.Symbol,
			"message": msg,
		})
	}
}

var currencySymbols = map[string]string{"usd": "$", "eur": "€", "cny": "¥", "btc": "₿"}

func formatMoney(v float64, currency string) string {
	sym := currencySymbols[strings.ToLower(currency)]
	if sym == "" {
		sym = strings.ToUpper(currency) + " "
	}
	if v < 1 {
		return fmt.Sprintf("%s%.6f", sym, v)
	}
	// 70123.456 -> "70,123.46"
	s := fmt.Sprintf("%.2f", v)
	intPart, frac := s[:len(s)-3], s[len(s)-3:]
	var b strings.Builder
	for i, c := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(c)
	}
	return sym + b.String() + frac
}

// --- Watchlist ---

func (a *App) GetWatchlist() []models.WatchlistItem {
	var items []models.WatchlistItem
	db.DB.Order("created_at asc").Find(&items)
	return items
}

func (a *App) AddToWatchlist(coinId, symbol, name string) error {
	item := models.WatchlistItem{CoinID: coinId, Symbol: strings.ToUpper(symbol), Name: name}
	if err := db.DB.Where("coin_id = ?", coinId).FirstOrCreate(&item).Error; err != nil {
		return err
	}
	// Fetch the new coin's price right away instead of waiting for the next tick.
	go a.refreshNow()
	return nil
}

// RemoveFromWatchlist also removes the coin's alert and cached price, so a
// stale price can never trigger an alert.
func (a *App) RemoveFromWatchlist(coinId string) error {
	return db.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("coin_id = ?", coinId).Delete(&models.WatchlistItem{}).Error; err != nil {
			return err
		}
		if err := tx.Where("coin_id = ?", coinId).Delete(&models.PriceAlert{}).Error; err != nil {
			return err
		}
		return tx.Where("coin_id = ?", coinId).Delete(&models.CachedPrice{}).Error
	})
}

// --- Prices ---

// GetWatchlistPrices returns cached prices in the current currency, in watchlist order.
func (a *App) GetWatchlistPrices() []models.CachedPrice {
	items := a.GetWatchlist()
	if len(items) == 0 {
		return []models.CachedPrice{}
	}
	ids := make([]string, len(items))
	for i, item := range items {
		ids[i] = item.CoinID
	}
	cur := data.GetSettings().Currency
	var prices []models.CachedPrice
	db.DB.Where("coin_id IN ? AND currency = ?", ids, cur).Find(&prices)
	byID := make(map[string]models.CachedPrice, len(prices))
	for _, p := range prices {
		byID[p.CoinID] = p
	}
	out := make([]models.CachedPrice, 0, len(items))
	for _, it := range items {
		if p, ok := byID[it.CoinID]; ok {
			out = append(out, p)
		} else {
			// Not fetched yet: show the row with no price instead of hiding it.
			out = append(out, models.CachedPrice{CoinID: it.CoinID, Symbol: it.Symbol, Name: it.Name, Currency: cur})
		}
	}
	return out
}

func (a *App) GetTopCoins(limit int) ([]models.CachedPrice, error) {
	return a.cgAPI.GetTopCoins(data.GetSettings().Currency, limit)
}

func (a *App) SearchCoins(query string) ([]models.CoinSearchResult, error) {
	return a.cgAPI.SearchCoins(query)
}

// --- Market Overview ---

func (a *App) GetMarketOverview() (*models.MarketOverview, error) {
	return a.cgAPI.GetMarketOverview(data.GetSettings().Currency)
}

// --- Alerts ---

func (a *App) GetAlerts() []models.PriceAlert {
	var alerts []models.PriceAlert
	db.DB.Order("created_at asc").Find(&alerts)
	return alerts
}

// GetAlert returns the alert for a coin, or nil if there is none.
func (a *App) GetAlert(coinId string) *models.PriceAlert {
	var alert models.PriceAlert
	if err := db.DB.Where("coin_id = ?", coinId).First(&alert).Error; err != nil {
		return nil
	}
	return &alert
}

// SetAlert creates or replaces the alert for a coin, in the current currency, and (re)activates it.
func (a *App) SetAlert(coinId, symbol string, highPrice, lowPrice float64) error {
	if highPrice <= 0 && lowPrice <= 0 {
		return errors.New("set a high or a low price")
	}
	if highPrice > 0 && lowPrice > 0 && lowPrice >= highPrice {
		return errors.New("the low price must be below the high price")
	}
	var alert models.PriceAlert
	db.DB.Where("coin_id = ?", coinId).First(&alert)
	alert.CoinID = coinId
	alert.Symbol = symbol
	alert.Currency = data.GetSettings().Currency
	alert.HighPrice = highPrice
	alert.LowPrice = lowPrice
	alert.Enabled = true
	alert.TriggeredAt = nil
	alert.LastMessage = ""
	if alert.ID == 0 {
		return db.DB.Create(&alert).Error
	}
	return db.DB.Save(&alert).Error
}

// SetAlertEnabled pauses or resumes an alert.
func (a *App) SetAlertEnabled(id uint, enabled bool) error {
	return db.DB.Model(&models.PriceAlert{ID: id}).Select("Enabled").Updates(models.PriceAlert{Enabled: enabled}).Error
}

func (a *App) DeleteAlert(id uint) error {
	return db.DB.Delete(&models.PriceAlert{}, id).Error
}

// --- AI Analysis ---

// AnalyzeCoin runs the AI agent for a coin. Progress is streamed to the UI
// through "analysis-step" events; the final result is returned.
func (a *App) AnalyzeCoin(coinId string) (*models.AnalysisRecord, error) {
	return a.analysis.Analyze(a.ctx, coinId, func(step models.AgentStep) {
		runtime.EventsEmit(a.ctx, "analysis-step", map[string]any{"coinId": coinId, "step": step})
	})
}

// CancelAnalysis stops a running analysis (called when the user closes the panel).
func (a *App) CancelAnalysis(coinId string) {
	a.analysis.Cancel(coinId)
}

func (a *App) GetAnalysisHistory(limit int) []models.AnalysisRecord {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	return analysis.History(limit)
}

func (a *App) GetEvalStats() models.EvalStats {
	return analysis.Stats()
}

// EvaluateNow scores any analyses that are due (older than 24h).
func (a *App) EvaluateNow() models.EvalStats {
	a.analysis.EvaluatePending(10)
	return analysis.Stats()
}

// --- News ---

func (a *App) GetNews() ([]models.NewsItem, error) {
	return data.GetCryptoNews()
}

// --- Settings ---

func (a *App) GetSettings() *models.Settings {
	return data.GetSettingsView()
}

func (a *App) SaveSettings(s models.Settings) error {
	old := data.GetSettings()
	if err := data.SaveSettings(s); err != nil {
		return err
	}
	a.scheduleRefresh(s.RefreshSecs)
	if !strings.EqualFold(old.Currency, s.Currency) {
		// Prices in the old currency are no longer valid.
		if err := db.DB.Where("1 = 1").Delete(&models.CachedPrice{}).Error; err != nil {
			logger.Log.Warnf("clear cached prices: %v", err)
		}
		go a.refreshNow()
	}
	return nil
}

func (a *App) ClearAPIKey() error {
	return data.ClearAPIKey()
}
