package data

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"go-crypto/backend/indicators"
	"go-crypto/backend/models"

	"github.com/go-resty/resty/v2"
)

const cgBase = "https://api.coingecko.com/api/v3"

// APIError is a non-2xx response from CoinGecko.
type APIError struct {
	Status int
	Body   string
}

func (e *APIError) Error() string {
	if e.Status == http.StatusTooManyRequests {
		return "CoinGecko rate limit reached (HTTP 429). The free API allows only a few calls per minute; wait a moment and retry."
	}
	return fmt.Sprintf("CoinGecko HTTP %d: %s", e.Status, e.Body)
}

type CoinGeckoAPI struct {
	client *resty.Client
	cache  *ttlCache

	mu           sync.Mutex
	blockedUntil time.Time // set after a 429: no requests until then
}

func NewCoinGeckoAPI() *CoinGeckoAPI {
	c := resty.NewWithClient(&http.Client{Timeout: 10 * time.Second}).
		// Retries only happen on network errors (resty's default), never on HTTP 4xx/5xx.
		SetRetryCount(1).
		SetRetryWaitTime(2*time.Second).
		SetHeader("Accept", "application/json")
	return &CoinGeckoAPI{client: c, cache: newTTLCache(256)}
}

// rateLimitCooldown is how long all requests pause after a 429 when the
// response has no Retry-After header.
const rateLimitCooldown = 60 * time.Second

// get performs a GET, checks the HTTP status and decodes JSON into out.
// After a 429 it refuses to call CoinGecko until the cooldown ends, so a
// rate-limited app backs off instead of making the limit worse.
func (api *CoinGeckoAPI) get(path string, params map[string]string, out any) error {
	body, err := api.getRaw(path, params)
	if err != nil {
		return err
	}
	return decode(path, body, out)
}

func (api *CoinGeckoAPI) getRaw(path string, params map[string]string) ([]byte, error) {
	api.mu.Lock()
	wait := time.Until(api.blockedUntil)
	api.mu.Unlock()
	if wait > 0 {
		return nil, &APIError{Status: http.StatusTooManyRequests, Body: fmt.Sprintf("cooling down for %ds", int(wait.Seconds())+1)}
	}

	resp, err := api.client.R().SetQueryParams(params).Get(cgBase + path)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() == http.StatusTooManyRequests {
		cool := rateLimitCooldown
		if secs, err := strconv.Atoi(resp.Header().Get("Retry-After")); err == nil && secs > 0 && secs < 600 {
			cool = time.Duration(secs) * time.Second
		}
		api.mu.Lock()
		api.blockedUntil = time.Now().Add(cool)
		api.mu.Unlock()
	}
	if resp.IsError() {
		return nil, &APIError{Status: resp.StatusCode(), Body: truncate(resp.String(), 200)}
	}
	return resp.Body(), nil
}

func decode(path string, body []byte, out any) error {
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

// IsRateLimited reports whether err is (or wraps) a CoinGecko 429.
func IsRateLimited(err error) bool {
	var ae *APIError
	return errors.As(err, &ae) && ae.Status == http.StatusTooManyRequests
}

// cachedGet is get with an in-memory cache (and request de-duplication), to
// stay under the free-tier rate limit.
func (api *CoinGeckoAPI) cachedGet(path string, params map[string]string, ttl time.Duration, out any) error {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	sb.WriteString(path)
	for _, k := range keys {
		sb.WriteString("|" + k + "=" + params[k])
	}
	body, err := api.cache.Do(sb.String(), ttl, 0, func() ([]byte, error) {
		return api.getRaw(path, params)
	})
	if err != nil {
		return err
	}
	return decode(path, body, out)
}

// marketsRow is one row of /coins/markets.
type marketsRow struct {
	ID                 string  `json:"id"`
	Symbol             string  `json:"symbol"`
	Name               string  `json:"name"`
	CurrentPrice       float64 `json:"current_price"`
	PriceChangePercent float64 `json:"price_change_percentage_24h"`
	TotalVolume        float64 `json:"total_volume"`
	MarketCap          float64 `json:"market_cap"`
	High24h            float64 `json:"high_24h"`
	Low24h             float64 `json:"low_24h"`
}

func (r marketsRow) toCachedPrice(currency string) models.CachedPrice {
	return models.CachedPrice{
		CoinID:    r.ID,
		Symbol:    strings.ToUpper(r.Symbol),
		Name:      r.Name,
		Currency:  strings.ToLower(currency),
		Price:     r.CurrentPrice,
		Change24h: r.PriceChangePercent,
		Volume24h: r.TotalVolume,
		MarketCap: r.MarketCap,
		High24h:   r.High24h,
		Low24h:    r.Low24h,
		UpdatedAt: time.Now(),
	}
}

// markets calls /coins/markets. ids may be empty (then the top `limit` coins are returned).
func (api *CoinGeckoAPI) markets(ids []string, currency string, limit int) ([]models.CachedPrice, error) {
	params := map[string]string{
		"vs_currency": strings.ToLower(currency),
		"order":       "market_cap_desc",
		"per_page":    fmt.Sprintf("%d", limit),
		"page":        "1",
		"sparkline":   "false",
	}
	if len(ids) > 0 {
		params["ids"] = strings.Join(ids, ",")
	}
	var raw []marketsRow
	if err := api.cachedGet("/coins/markets", params, 20*time.Second, &raw); err != nil {
		return nil, err
	}
	out := make([]models.CachedPrice, 0, len(raw))
	for _, r := range raw {
		out = append(out, r.toCachedPrice(currency))
	}
	return out, nil
}

// GetPricesForWatchlist returns prices keyed by coin id.
func (api *CoinGeckoAPI) GetPricesForWatchlist(coinIds []string, currency string) (map[string]models.CachedPrice, error) {
	rows, err := api.markets(coinIds, currency, len(coinIds))
	if err != nil {
		return nil, err
	}
	result := make(map[string]models.CachedPrice, len(rows))
	for _, r := range rows {
		result[r.CoinID] = r
	}
	return result, nil
}

func (api *CoinGeckoAPI) GetTopCoins(currency string, limit int) ([]models.CachedPrice, error) {
	return api.markets(nil, currency, limit)
}

func (api *CoinGeckoAPI) SearchCoins(query string) ([]models.CoinSearchResult, error) {
	var result struct {
		Coins []struct {
			ID     string `json:"id"`
			Symbol string `json:"symbol"`
			Name   string `json:"name"`
			Thumb  string `json:"thumb"`
		} `json:"coins"`
	}
	if err := api.cachedGet("/search", map[string]string{"query": query}, 5*time.Minute, &result); err != nil {
		return nil, err
	}
	coins := make([]models.CoinSearchResult, 0, len(result.Coins))
	for _, c := range result.Coins {
		coins = append(coins, models.CoinSearchResult{ID: c.ID, Symbol: strings.ToUpper(c.Symbol), Name: c.Name, Thumb: c.Thumb})
	}
	return coins, nil
}

func (api *CoinGeckoAPI) GetMarketOverview(currency string) (*models.MarketOverview, error) {
	var result struct {
		Data struct {
			TotalMarketCap            map[string]float64 `json:"total_market_cap"`
			TotalVolume               map[string]float64 `json:"total_volume"`
			MarketCapPercentage       map[string]float64 `json:"market_cap_percentage"`
			ActiveCryptocurrencies    int                `json:"active_cryptocurrencies"`
			MarketCapChangePercent24h float64            `json:"market_cap_change_percentage_24h_usd"`
		} `json:"data"`
	}
	if err := api.cachedGet("/global", nil, 60*time.Second, &result); err != nil {
		return nil, err
	}
	cur := strings.ToLower(currency)
	return &models.MarketOverview{
		Currency:           cur,
		TotalMarketCap:     result.Data.TotalMarketCap[cur],
		TotalVolume24h:     result.Data.TotalVolume[cur],
		BTCDominance:       result.Data.MarketCapPercentage["btc"],
		ETHDominance:       result.Data.MarketCapPercentage["eth"],
		ActiveCoins:        result.Data.ActiveCryptocurrencies,
		MarketCapChange24h: result.Data.MarketCapChangePercent24h,
	}, nil
}

type marketChart struct {
	Prices [][2]float64 `json:"prices"` // [ms timestamp, price]
}

func (m marketChart) points() []indicators.Point {
	pts := make([]indicators.Point, 0, len(m.Prices))
	for _, p := range m.Prices {
		pts = append(pts, indicators.Point{T: time.UnixMilli(int64(p[0])), Price: p[1]})
	}
	return pts
}

// GetPriceHistory returns intraday prices for the last `days` days (cached 10 min).
func (api *CoinGeckoAPI) GetPriceHistory(coinID, currency string, days int) ([]indicators.Point, error) {
	var m marketChart
	err := api.cachedGet("/coins/"+coinID+"/market_chart", map[string]string{
		"vs_currency": strings.ToLower(currency),
		"days":        fmt.Sprintf("%d", days),
	}, 10*time.Minute, &m)
	if err != nil {
		return nil, err
	}
	return m.points(), nil
}

// GetPriceAt returns the observed price closest to t (within ±2h).
func (api *CoinGeckoAPI) GetPriceAt(coinID, currency string, t time.Time) (float64, error) {
	var m marketChart
	err := api.get("/coins/"+coinID+"/market_chart/range", map[string]string{
		"vs_currency": strings.ToLower(currency),
		"from":        fmt.Sprintf("%d", t.Add(-2*time.Hour).Unix()),
		"to":          fmt.Sprintf("%d", t.Add(2*time.Hour).Unix()),
	}, &m)
	if err != nil {
		return 0, err
	}
	best, bestDiff := 0.0, time.Duration(1<<62)
	for _, p := range m.points() {
		d := p.T.Sub(t)
		if d < 0 {
			d = -d
		}
		if d < bestDiff {
			best, bestDiff = p.Price, d
		}
	}
	if best == 0 {
		return 0, fmt.Errorf("no price data for %s around %s", coinID, t.Format(time.RFC3339))
	}
	return best, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
