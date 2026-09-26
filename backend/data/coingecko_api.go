package data

import (
	"encoding/json"
	"fmt"
	"go-crypto/backend/logger"
	"go-crypto/backend/models"
	"net/http"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
)

const cgBase = "https://api.coingecko.com/api/v3"

type CoinGeckoAPI struct {
	client *resty.Client
}

func NewCoinGeckoAPI() *CoinGeckoAPI {
	c := resty.NewWithClient(&http.Client{Timeout: 15 * time.Second}).
		SetRetryCount(2).
		SetRetryWaitTime(2 * time.Second).
		SetHeader("Accept", "application/json")
	return &CoinGeckoAPI{client: c}
}

// GetPricesForWatchlist uses /coins/markets to fetch full price data including 24h high/low.
func (api *CoinGeckoAPI) GetPricesForWatchlist(coinIds []string, currency string) (map[string]*models.CachedPrice, error) {
	resp, err := api.client.R().
		SetQueryParams(map[string]string{
			"vs_currency": currency,
			"ids":         strings.Join(coinIds, ","),
			"order":       "market_cap_desc",
			"per_page":    fmt.Sprintf("%d", len(coinIds)),
			"page":        "1",
			"sparkline":   "false",
		}).
		Get(cgBase + "/coins/markets")
	if err != nil {
		return nil, err
	}

	var raw []struct {
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
	if err := json.Unmarshal(resp.Body(), &raw); err != nil {
		return nil, err
	}

	result := make(map[string]*models.CachedPrice, len(raw))
	for _, c := range raw {
		result[c.ID] = &models.CachedPrice{
			CoinID:    c.ID,
			Symbol:    strings.ToUpper(c.Symbol),
			Name:      c.Name,
			Price:     c.CurrentPrice,
			Change24h: c.PriceChangePercent,
			Volume24h: c.TotalVolume,
			MarketCap: c.MarketCap,
			High24h:   c.High24h,
			Low24h:    c.Low24h,
			UpdatedAt: time.Now(),
		}
	}
	return result, nil
}

func (api *CoinGeckoAPI) SearchCoins(query string) ([]models.CoinSearchResult, error) {
	resp, err := api.client.R().
		SetQueryParam("query", query).
		Get(cgBase + "/search")
	if err != nil {
		return nil, err
	}

	var result struct {
		Coins []struct {
			ID     string `json:"id"`
			Symbol string `json:"symbol"`
			Name   string `json:"name"`
			Thumb  string `json:"thumb"`
		} `json:"coins"`
	}
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, err
	}

	coins := make([]models.CoinSearchResult, 0, len(result.Coins))
	for _, c := range result.Coins {
		coins = append(coins, models.CoinSearchResult{
			ID:     c.ID,
			Symbol: strings.ToUpper(c.Symbol),
			Name:   c.Name,
			Thumb:  c.Thumb,
		})
	}
	return coins, nil
}

func (api *CoinGeckoAPI) GetMarketOverview(currency string) (*models.MarketOverview, error) {
	resp, err := api.client.R().Get(cgBase + "/global")
	if err != nil {
		return nil, err
	}

	var result struct {
		Data struct {
			TotalMarketCap            map[string]float64 `json:"total_market_cap"`
			TotalVolume               map[string]float64 `json:"total_volume"`
			MarketCapPercentage       map[string]float64 `json:"market_cap_percentage"`
			ActiveCryptocurrencies    int                `json:"active_cryptocurrencies"`
			MarketCapChangePercent24h float64            `json:"market_cap_change_percentage_24h_usd"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return nil, err
	}

	cur := strings.ToLower(currency)
	return &models.MarketOverview{
		TotalMarketCap:     result.Data.TotalMarketCap[cur],
		TotalVolume24h:     result.Data.TotalVolume[cur],
		BTCDominance:       result.Data.MarketCapPercentage["btc"],
		ETHDominance:       result.Data.MarketCapPercentage["eth"],
		ActiveCoins:        result.Data.ActiveCryptocurrencies,
		MarketCapChange24h: result.Data.MarketCapChangePercent24h,
	}, nil
}

func (api *CoinGeckoAPI) GetTopCoins(currency string, limit int) ([]models.CachedPrice, error) {
	resp, err := api.client.R().
		SetQueryParams(map[string]string{
			"vs_currency": currency,
			"order":       "market_cap_desc",
			"per_page":    fmt.Sprintf("%d", limit),
			"page":        "1",
			"sparkline":   "false",
		}).
		Get(cgBase + "/coins/markets")
	if err != nil {
		return nil, err
	}

	var raw []struct {
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
	if err := json.Unmarshal(resp.Body(), &raw); err != nil {
		return nil, err
	}

	coins := make([]models.CachedPrice, 0, len(raw))
	for _, c := range raw {
		coins = append(coins, models.CachedPrice{
			CoinID:    c.ID,
			Symbol:    strings.ToUpper(c.Symbol),
			Name:      c.Name,
			Price:     c.CurrentPrice,
			Change24h: c.PriceChangePercent,
			Volume24h: c.TotalVolume,
			MarketCap: c.MarketCap,
			High24h:   c.High24h,
			Low24h:    c.Low24h,
			UpdatedAt: time.Now(),
		})
	}
	_ = logger.Log
	return coins, nil
}
