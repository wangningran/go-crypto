package data

import (
	"encoding/json"
	"go-crypto/backend/models"
	"net/http"
	"time"

	"github.com/go-resty/resty/v2"
)

func GetCryptoNews() ([]models.NewsItem, error) {
	client := resty.NewWithClient(&http.Client{Timeout: 15 * time.Second}).
		SetHeader("Accept", "application/json")

	resp, err := client.R().Get("https://api.coingecko.com/api/v3/news")
	if err != nil {
		return nil, err
	}

	var raw struct {
		Data []struct {
			Title       string `json:"title"`
			URL         string `json:"url"`
			Author      string `json:"author"`
			PublishedAt int64  `json:"published_at"`
			Description string `json:"description"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp.Body(), &raw); err != nil {
		return nil, err
	}

	items := make([]models.NewsItem, 0, len(raw.Data))
	for _, n := range raw.Data {
		t := time.Unix(n.PublishedAt, 0).Format("2006-01-02 15:04")
		items = append(items, models.NewsItem{
			Title:       n.Title,
			URL:         n.URL,
			Source:      n.Author,
			PublishedAt: t,
			Summary:     n.Description,
		})
	}
	return items, nil
}
