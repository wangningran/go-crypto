package models

import "time"

type WatchlistItem struct {
	ID        uint      `json:"id" gorm:"primarykey"`
	CoinID    string    `json:"coinId" gorm:"uniqueIndex;size:50"`
	Symbol    string    `json:"symbol" gorm:"size:20"`
	Name      string    `json:"name" gorm:"size:100"`
	CreatedAt time.Time `json:"createdAt"`
}

type PriceAlert struct {
	ID        uint      `json:"id" gorm:"primarykey"`
	CoinID    string    `json:"coinId" gorm:"index;size:50"`
	Symbol    string    `json:"symbol" gorm:"size:20"`
	HighPrice float64   `json:"highPrice"`
	LowPrice  float64   `json:"lowPrice"`
	Enabled   bool      `json:"enabled" gorm:"default:true"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Settings struct {
	ID          uint   `json:"id" gorm:"primarykey"`
	OpenAIKey   string `json:"openAiKey" gorm:"size:200"`
	OpenAIBase  string `json:"openAiBase" gorm:"size:200"`
	OpenAIModel string `json:"openAiModel" gorm:"size:100"`
	Currency    string `json:"currency" gorm:"size:10"`
	RefreshSecs int    `json:"refreshSecs"`
}

type CachedPrice struct {
	ID        uint      `json:"id" gorm:"primarykey"`
	CoinID    string    `json:"coinId" gorm:"uniqueIndex;size:50"`
	Symbol    string    `json:"symbol" gorm:"size:20"`
	Name      string    `json:"name" gorm:"size:100"`
	Price     float64   `json:"price"`
	Change24h float64   `json:"change24h"`
	Volume24h float64   `json:"volume24h"`
	MarketCap float64   `json:"marketCap"`
	High24h   float64   `json:"high24h"`
	Low24h    float64   `json:"low24h"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type CoinSearchResult struct {
	ID     string `json:"id"`
	Symbol string `json:"symbol"`
	Name   string `json:"name"`
	Thumb  string `json:"thumb"`
}

type MarketOverview struct {
	TotalMarketCap     float64 `json:"totalMarketCap"`
	TotalVolume24h     float64 `json:"totalVolume24h"`
	BTCDominance       float64 `json:"btcDominance"`
	ETHDominance       float64 `json:"ethDominance"`
	ActiveCoins        int     `json:"activeCoins"`
	MarketCapChange24h float64 `json:"marketCapChange24h"`
}

type NewsItem struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Source      string `json:"source"`
	PublishedAt string `json:"publishedAt"`
	Summary     string `json:"summary"`
}
