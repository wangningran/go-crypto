package data

import (
	"testing"

	"go-crypto/backend/models"
)

func TestFilterNews(t *testing.T) {
	items := []models.NewsItem{
		{Title: "Bitcoin ETF inflows hit record"},
		{Title: "Ethereum upgrade scheduled", Summary: "ETH developers confirm date"},
		{Title: "Market wrap: stocks and crypto"},
		{Title: "Ethos token launches"}, // must not match "eth" as a substring
	}
	got := FilterNews(items, "Ethereum", "ETH", 5)
	if len(got) != 1 || got[0].Title != "Ethereum upgrade scheduled" {
		t.Fatalf("FilterNews(ethereum) = %+v", got)
	}
	if got := FilterNews(items, "Bitcoin", "BTC", 5); len(got) != 1 {
		t.Fatalf("FilterNews(bitcoin) = %+v", got)
	}
	if got := FilterNews(items, "Solana", "SOL", 5); len(got) != 0 {
		t.Fatalf("FilterNews(solana) = %+v", got)
	}
}
