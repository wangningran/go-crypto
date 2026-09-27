package data

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go-crypto/backend/models"

	"github.com/go-resty/resty/v2"
)

var newsClient = resty.NewWithClient(&http.Client{Timeout: 10 * time.Second}).
	SetHeader("Accept", "application/json")

// News changes slowly: cache it for 5 minutes, and cache failures for 2
// minutes so a broken endpoint isn't called on every AI analysis.
var newsCache = newTTLCache(4)

// GetCryptoNews fetches the latest headlines from CoinGecko's news endpoint.
func GetCryptoNews() ([]models.NewsItem, error) {
	body, err := newsCache.Do("news", 5*time.Minute, 2*time.Minute, func() ([]byte, error) {
		resp, err := newsClient.R().Get(cgBase + "/news")
		if err != nil {
			return nil, err
		}
		if resp.IsError() {
			return nil, fmt.Errorf("news source unavailable: %w", &APIError{Status: resp.StatusCode(), Body: truncate(resp.String(), 200)})
		}
		return resp.Body(), nil
	})
	if err != nil {
		return nil, err
	}

	var raw struct {
		Data []struct {
			Title       string `json:"title"`
			URL         string `json:"url"`
			Author      string `json:"author"`
			NewsSite    string `json:"news_site"`
			PublishedAt int64  `json:"published_at"`
			UpdatedAt   int64  `json:"updated_at"`
			Description string `json:"description"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("news source returned an unexpected format: %w", err)
	}

	items := make([]models.NewsItem, 0, len(raw.Data))
	for _, n := range raw.Data {
		ts := n.PublishedAt
		if ts == 0 {
			ts = n.UpdatedAt
		}
		published := ""
		if ts > 0 {
			published = time.Unix(ts, 0).Format("2006-01-02 15:04")
		}
		source := n.NewsSite
		if source == "" {
			source = n.Author
		}
		items = append(items, models.NewsItem{
			Title:       n.Title,
			URL:         n.URL,
			Source:      source,
			PublishedAt: published,
			Summary:     n.Description,
		})
	}
	return items, nil
}

// FilterNews keeps headlines that mention the coin's name or symbol.
func FilterNews(items []models.NewsItem, name, symbol string, limit int) []models.NewsItem {
	name, symbol = strings.ToLower(name), strings.ToLower(symbol)
	var out []models.NewsItem
	for _, n := range items {
		text := " " + strings.ToLower(n.Title+" "+n.Summary) + " "
		if (name != "" && strings.Contains(text, name)) || (symbol != "" && strings.Contains(text, " "+symbol+" ")) {
			out = append(out, n)
			if len(out) >= limit {
				break
			}
		}
	}
	return out
}
