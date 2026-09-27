package data

import (
	"encoding/xml"
	"net/http"
	"strings"
	"time"

	"go-crypto/backend/models"

	"github.com/go-resty/resty/v2"
)

// rssFeeds are parsed in order; results are merged and deduplicated by URL.
// CoinGecko /news now requires a paid API key (returns 401 on the free tier).
var rssFeeds = []string{
	"https://cointelegraph.com/rss",
	"https://www.coindesk.com/arc/outboundfeeds/rss/",
}

type rssFeed struct {
	XMLName xml.Name   `xml:"rss"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Items []rssItem `xml:"item"`
}

type rssItem struct {
	Title   string `xml:"title"`
	Link    string `xml:"link"`
	PubDate string `xml:"pubDate"`
	Desc    string `xml:"description"`
}

var newsClient = resty.NewWithClient(&http.Client{Timeout: 15 * time.Second}).
	SetHeader("Accept", "application/rss+xml, text/xml")

var newsCache = newTTLCache(4)

// GetCryptoNews fetches headlines from Cointelegraph and CoinDesk RSS feeds.
func GetCryptoNews() ([]models.NewsItem, error) {
	var items []models.NewsItem
	seen := make(map[string]bool)

	for _, feedURL := range rssFeeds {
		feedURL := feedURL
		body, err := newsCache.Do(feedURL, 5*time.Minute, 2*time.Minute, func() ([]byte, error) {
			resp, err := newsClient.R().Get(feedURL)
			if err != nil {
				return nil, err
			}
			return resp.Body(), nil
		})
		if err != nil {
			continue
		}

		var feed rssFeed
		if err := xml.Unmarshal(body, &feed); err != nil {
			continue
		}

		source := feedSource(feedURL)
		for _, it := range feed.Channel.Items {
			link := strings.TrimSpace(it.Link)
			if seen[link] || link == "" {
				continue
			}
			seen[link] = true

			summary := stripHTML(it.Desc)
			if len(summary) > 200 {
				summary = summary[:200] + "…"
			}

			items = append(items, models.NewsItem{
				Title:       strings.TrimSpace(it.Title),
				URL:         link,
				Source:      source,
				PublishedAt: formatPubDate(it.PubDate),
				Summary:     summary,
			})
		}
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

func feedSource(url string) string {
	switch {
	case strings.Contains(url, "cointelegraph"):
		return "Cointelegraph"
	case strings.Contains(url, "coindesk"):
		return "CoinDesk"
	default:
		return "Crypto News"
	}
}

func stripHTML(s string) string {
	inTag := false
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

func formatPubDate(raw string) string {
	formats := []string{
		time.RFC1123Z,
		time.RFC1123,
		"Mon, 2 Jan 2006 15:04:05 -0700",
		"Mon, 2 Jan 2006 15:04:05 +0000",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, strings.TrimSpace(raw)); err == nil {
			return t.Format("2006-01-02 15:04")
		}
	}
	return raw
}
