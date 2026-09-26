package models

import "time"

type WatchlistItem struct {
	ID        uint      `json:"id" gorm:"primarykey"`
	CoinID    string    `json:"coinId" gorm:"uniqueIndex;size:50"`
	Symbol    string    `json:"symbol" gorm:"size:20"`
	Name      string    `json:"name" gorm:"size:100"`
	CreatedAt time.Time `json:"createdAt"`
}

// PriceAlert fires once when the price crosses HighPrice or LowPrice, then
// pauses itself (Enabled=false) until the user resumes it.
type PriceAlert struct {
	ID          uint       `json:"id" gorm:"primarykey"`
	CoinID      string     `json:"coinId" gorm:"uniqueIndex;size:50"`
	Symbol      string     `json:"symbol" gorm:"size:20"`
	Currency    string     `json:"currency" gorm:"size:10"`
	HighPrice   float64    `json:"highPrice"`
	LowPrice    float64    `json:"lowPrice"`
	Enabled     bool       `json:"enabled" gorm:"default:true"`
	TriggeredAt *time.Time `json:"triggeredAt"`
	LastMessage string     `json:"lastMessage" gorm:"size:200"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

// Settings is stored in SQLite. The API key is NOT stored here: it lives in
// the OS keychain. APIKey / HasAPIKey / APIKeyHint only travel to and from the UI.
type Settings struct {
	ID          uint   `json:"id" gorm:"primarykey"`
	OpenAIBase  string `json:"openAiBase" gorm:"size:200"`
	OpenAIModel string `json:"openAiModel" gorm:"size:100"`
	Currency    string `json:"currency" gorm:"size:10"`
	RefreshSecs int    `json:"refreshSecs"`

	// APIKey is only used when saving: empty means "keep the current key".
	APIKey     string `json:"apiKey" gorm:"-"`
	HasAPIKey  bool   `json:"hasApiKey" gorm:"-"`
	APIKeyHint string `json:"apiKeyHint" gorm:"-"`
}

type CachedPrice struct {
	ID        uint      `json:"id" gorm:"primarykey"`
	CoinID    string    `json:"coinId" gorm:"uniqueIndex;size:50"`
	Symbol    string    `json:"symbol" gorm:"size:20"`
	Name      string    `json:"name" gorm:"size:100"`
	Currency  string    `json:"currency" gorm:"size:10"`
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
	Currency           string  `json:"currency"`
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

// AgentStep is one tool call the AI agent made while analyzing a coin.
type AgentStep struct {
	Index      int    `json:"index"`
	Tool       string `json:"tool"`
	Args       string `json:"args"`
	Result     string `json:"result"`
	Error      string `json:"error"`
	DurationMs int64  `json:"durationMs"`
}

// AnalysisRecord is one AI analysis. Every record is later scored against the
// real price 24h after it was made, which is how the app measures the AI.
type AnalysisRecord struct {
	ID       uint   `json:"id" gorm:"primarykey"`
	CoinID   string `json:"coinId" gorm:"index;size:50"`
	Symbol   string `json:"symbol" gorm:"size:20"`
	Name     string `json:"name" gorm:"size:100"`
	Currency string `json:"currency" gorm:"size:10"`
	Model    string `json:"model" gorm:"size:100"`

	Direction  string  `json:"direction" gorm:"size:10"` // bullish | bearish | neutral
	Confidence float64 `json:"confidence"`               // 0..1
	Support    float64 `json:"support"`
	Resistance float64 `json:"resistance"`
	Summary    string  `json:"summary" gorm:"type:text"`

	PriceAtAnalysis  float64     `json:"priceAtAnalysis"`
	StepsJSON        string      `json:"-" gorm:"type:text"`
	Steps            []AgentStep `json:"steps" gorm:"-"`
	PromptTokens     int         `json:"promptTokens"`
	CompletionTokens int         `json:"completionTokens"`
	Cached           bool        `json:"cached" gorm:"-"`
	CreatedAt        time.Time   `json:"createdAt" gorm:"index"`

	// Filled in by the evaluator 24h later.
	EvaluatedAt   *time.Time `json:"evaluatedAt"`
	PriceAfter24h float64    `json:"priceAfter24h"`
	ReturnPct     float64    `json:"returnPct"`
	Correct       *bool      `json:"correct"`
}

type DirectionStats struct {
	Direction string  `json:"direction"`
	Evaluated int     `json:"evaluated"`
	Correct   int     `json:"correct"`
	Accuracy  float64 `json:"accuracy"`
}

// EvalStats summarizes how often the AI's 24h direction call was right.
type EvalStats struct {
	Total                int              `json:"total"`
	Evaluated            int              `json:"evaluated"`
	Pending              int              `json:"pending"`
	Correct              int              `json:"correct"`
	Accuracy             float64          `json:"accuracy"`
	AvgConfidenceCorrect float64          `json:"avgConfidenceCorrect"`
	AvgConfidenceWrong   float64          `json:"avgConfidenceWrong"`
	ByDirection          []DirectionStats `json:"byDirection"`
	NeutralBandPct       float64          `json:"neutralBandPct"`
}
