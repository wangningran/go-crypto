// Package indicators computes technical indicators from a price series.
//
// The numbers the AI talks about (moving averages, RSI, volatility, 30-day
// range) are computed here in plain Go, never by the model. The model only
// interprets them. This keeps the analysis grounded and testable.
package indicators

import (
	"math"
	"sort"
	"time"
)

// Point is one price observation.
type Point struct {
	T     time.Time
	Price float64
}

// ResampleDaily turns an intraday series into one closing price per UTC day,
// oldest first. The last observation of each day is used as that day's close.
func ResampleDaily(points []Point) []float64 {
	if len(points) == 0 {
		return nil
	}
	sorted := make([]Point, len(points))
	copy(sorted, points)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].T.Before(sorted[j].T) })

	var closes []float64
	var curDay string
	for _, p := range sorted {
		day := p.T.UTC().Format("2006-01-02")
		if day != curDay {
			closes = append(closes, p.Price)
			curDay = day
		} else {
			closes[len(closes)-1] = p.Price
		}
	}
	return closes
}

// SMA is the simple moving average of the last n values.
func SMA(v []float64, n int) (float64, bool) {
	if n <= 0 || len(v) < n {
		return 0, false
	}
	sum := 0.0
	for _, x := range v[len(v)-n:] {
		sum += x
	}
	return sum / float64(n), true
}

// RSI is Wilder's relative strength index over n periods (usually 14).
// It needs at least n+1 values.
func RSI(v []float64, n int) (float64, bool) {
	if n <= 0 || len(v) < n+1 {
		return 0, false
	}
	var gain, loss float64
	for i := 1; i <= n; i++ {
		d := v[i] - v[i-1]
		if d > 0 {
			gain += d
		} else {
			loss -= d
		}
	}
	avgGain, avgLoss := gain/float64(n), loss/float64(n)
	for i := n + 1; i < len(v); i++ {
		d := v[i] - v[i-1]
		g, l := 0.0, 0.0
		if d > 0 {
			g = d
		} else {
			l = -d
		}
		avgGain = (avgGain*float64(n-1) + g) / float64(n)
		avgLoss = (avgLoss*float64(n-1) + l) / float64(n)
	}
	if avgLoss == 0 {
		if avgGain == 0 {
			return 50, true
		}
		return 100, true
	}
	rs := avgGain / avgLoss
	return 100 - 100/(1+rs), true
}

// PctChange is the % change between the last value and the value `lookback`
// periods earlier.
func PctChange(v []float64, lookback int) (float64, bool) {
	if lookback <= 0 || len(v) < lookback+1 {
		return 0, false
	}
	base := v[len(v)-1-lookback]
	if base == 0 {
		return 0, false
	}
	return (v[len(v)-1] - base) / base * 100, true
}

// DailyVolatility is the standard deviation of daily % returns.
func DailyVolatility(v []float64) (float64, bool) {
	if len(v) < 3 {
		return 0, false
	}
	rets := make([]float64, 0, len(v)-1)
	for i := 1; i < len(v); i++ {
		if v[i-1] == 0 {
			continue
		}
		rets = append(rets, (v[i]-v[i-1])/v[i-1]*100)
	}
	if len(rets) < 2 {
		return 0, false
	}
	mean := 0.0
	for _, r := range rets {
		mean += r
	}
	mean /= float64(len(rets))
	ss := 0.0
	for _, r := range rets {
		ss += (r - mean) * (r - mean)
	}
	return math.Sqrt(ss / float64(len(rets)-1)), true
}

// Snapshot is the indicator summary handed to the AI agent.
// Fields that could not be computed (not enough history) are nil.
type Snapshot struct {
	Days         int      `json:"days"`
	Last         float64  `json:"last"`
	Change7dPct  *float64 `json:"change7dPct,omitempty"`
	Change30dPct *float64 `json:"change30dPct,omitempty"`
	SMA7         *float64 `json:"sma7,omitempty"`
	SMA30        *float64 `json:"sma30,omitempty"`
	RSI14        *float64 `json:"rsi14,omitempty"`
	VolatilityPc *float64 `json:"dailyVolatilityPct,omitempty"`
	RangeHigh    float64  `json:"rangeHigh"`
	RangeLow     float64  `json:"rangeLow"`
	// Trend is a rule-based label: "up" if price > SMA7 > SMA30,
	// "down" if price < SMA7 < SMA30, otherwise "sideways".
	Trend string `json:"trend"`
	// SupportLevels / ResistanceLevels are computed price levels, nearest to
	// Last first (see SupportResistance). The model must pick from these
	// rather than invent a number — ParseSubmission snaps to the nearest
	// candidate if it doesn't.
	SupportLevels    []float64 `json:"supportLevels,omitempty"`
	ResistanceLevels []float64 `json:"resistanceLevels,omitempty"`
}

func ptr(x float64, ok bool) *float64 {
	if !ok {
		return nil
	}
	r := math.Round(x*100) / 100
	return &r
}

// Summarize computes a Snapshot from daily closes (oldest first).
func Summarize(daily []float64) Snapshot {
	s := Snapshot{Days: len(daily), Trend: "unknown"}
	if len(daily) == 0 {
		return s
	}
	s.Last = daily[len(daily)-1]
	s.RangeHigh, s.RangeLow = daily[0], daily[0]
	for _, x := range daily {
		s.RangeHigh = math.Max(s.RangeHigh, x)
		s.RangeLow = math.Min(s.RangeLow, x)
	}
	s.Change7dPct = ptr(PctChange(daily, 7))
	s.Change30dPct = ptr(PctChange(daily, 30))
	sma7, ok7 := SMA(daily, 7)
	sma30, ok30 := SMA(daily, 30)
	s.SMA7 = ptr(sma7, ok7)
	s.SMA30 = ptr(sma30, ok30)
	s.RSI14 = ptr(RSI(daily, 14))
	s.VolatilityPc = ptr(DailyVolatility(daily))

	if ok7 && ok30 {
		switch {
		case s.Last > sma7 && sma7 > sma30:
			s.Trend = "up"
		case s.Last < sma7 && sma7 < sma30:
			s.Trend = "down"
		default:
			s.Trend = "sideways"
		}
	}
	s.SupportLevels, s.ResistanceLevels = SupportResistance(daily, s.Last)
	return s
}

// levelClusterTolPct: swing points within this % of each other are treated as
// the same level (price rarely reverses at the exact same cent twice).
const levelClusterTolPct = 0.015

// maxLevels caps how many support/resistance candidates are returned.
const maxLevels = 3

// SupportResistance finds candidate support and resistance levels from daily
// closes: it locates swing highs/lows (points that are a local extreme within
// a small window), clusters nearby ones together (weighting by how many times
// price turned near that level), and returns the levels closest to current,
// split into support (below current) and resistance (above current).
//
// This exists so the AI agent picks support/resistance from real turning
// points in the data instead of inventing plausible-looking numbers: a level
// with no basis in the price history is exactly the kind of unverifiable
// claim the rest of this package (SMA, RSI, volatility) was written to avoid.
func SupportResistance(daily []float64, current float64) (supports, resistances []float64) {
	const window = 2 // a swing point must be the extreme among its window neighbors on each side
	if len(daily) < 2*window+1 {
		return nil, nil
	}

	var swings []float64
	for i := window; i < len(daily)-window; i++ {
		isLow, isHigh := true, true
		for k := i - window; k <= i+window; k++ {
			if k == i {
				continue
			}
			if daily[k] < daily[i] {
				isLow = false
			}
			if daily[k] > daily[i] {
				isHigh = false
			}
		}
		if isLow || isHigh {
			swings = append(swings, daily[i])
		}
	}
	if len(swings) == 0 {
		return nil, nil
	}

	for _, lv := range clusterLevels(swings) {
		switch {
		case lv < current:
			supports = append(supports, lv)
		case lv > current:
			resistances = append(resistances, lv)
		}
	}
	// Nearest to current price first.
	sort.Sort(sort.Reverse(sort.Float64Slice(supports)))
	sort.Float64s(resistances)
	return round2All(topN(supports, maxLevels)), round2All(topN(resistances, maxLevels))
}

// clusterLevels merges swing points within levelClusterTolPct of each other,
// replacing each group with its (touch-weighted) average, ranked by touches.
func clusterLevels(levels []float64) []float64 {
	sorted := append([]float64{}, levels...)
	sort.Float64s(sorted)

	type cluster struct {
		avg     float64
		touches int
	}
	var clusters []cluster
	for _, lv := range sorted {
		if n := len(clusters); n > 0 {
			last := &clusters[n-1]
			if last.avg != 0 && math.Abs(lv-last.avg)/math.Abs(last.avg) <= levelClusterTolPct {
				last.avg = (last.avg*float64(last.touches) + lv) / float64(last.touches+1)
				last.touches++
				continue
			}
		}
		clusters = append(clusters, cluster{avg: lv, touches: 1})
	}
	sort.SliceStable(clusters, func(i, j int) bool { return clusters[i].touches > clusters[j].touches })

	out := make([]float64, len(clusters))
	for i, c := range clusters {
		out[i] = c.avg
	}
	return out
}

func topN(v []float64, n int) []float64 {
	if len(v) > n {
		return v[:n]
	}
	return v
}

func round2All(v []float64) []float64 {
	out := make([]float64, len(v))
	for i, x := range v {
		out[i] = math.Round(x*100) / 100
	}
	return out
}
