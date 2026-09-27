package main

import "testing"

func TestFormatMoney(t *testing.T) {
	cases := map[string]string{
		formatMoney(70123.456, "usd"): "$70,123.46",
		formatMoney(999, "eur"):       "€999.00",
		formatMoney(1234567, "cny"):   "¥1,234,567.00",
		formatMoney(0.0000123, "usd"): "$0.000012",
		formatMoney(12, "gbp"):        "GBP 12.00",
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("got %q want %q", got, want)
		}
	}
}
