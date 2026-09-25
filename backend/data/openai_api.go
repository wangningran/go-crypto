package data

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go-crypto/backend/models"
	"io"
	"net/http"
	"strings"
	"time"
)

type OpenAIClient struct{}

func NewOpenAIClient() *OpenAIClient { return &OpenAIClient{} }

func (c *OpenAIClient) AnalyzeCoin(coinId string, price *models.CachedPrice) (string, error) {
	s := GetSettings()
	if strings.TrimSpace(s.OpenAIKey) == "" {
		return "⚙️ Please configure your OpenAI API key in Settings to enable AI analysis.", nil
	}

	prompt := fmt.Sprintf(
		"Analyze %s (%s) with current data:\n"+
			"Price: $%.4f | 24h Change: %.2f%%\n"+
			"24h High: $%.4f | 24h Low: $%.4f\n"+
			"Market Cap: $%.0f | 24h Volume: $%.0f\n\n"+
			"Provide a concise professional crypto market analysis covering: price action, trend, key levels, and sentiment. Under 400 words.",
		price.Name, price.Symbol,
		price.Price, price.Change24h,
		price.High24h, price.Low24h,
		price.MarketCap, price.Volume24h,
	)

	body := map[string]interface{}{
		"model": s.OpenAIModel,
		"messages": []map[string]string{
			{"role": "system", "content": "You are a professional cryptocurrency analyst. Provide concise, objective market analysis."},
			{"role": "user", "content": prompt},
		},
		"max_tokens": 600,
	}
	bodyBytes, _ := json.Marshal(body)

	req, err := http.NewRequest("POST", strings.TrimRight(s.OpenAIBase, "/")+"/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+s.OpenAIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return "", err
	}
	if result.Error != nil {
		return "", fmt.Errorf("API error: %s", result.Error.Message)
	}
	if len(result.Choices) == 0 {
		return "No analysis returned.", nil
	}
	return result.Choices[0].Message.Content, nil
}
