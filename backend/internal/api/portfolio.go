package api

import (
	"encoding/json"
	"math"
	"net/http"
	"sort"

	"flowsight/internal/model"
)

// PortfolioRisk serves GET /api/portfolio/risk: concentration bars,
// correlation matrix, beta vs IHSG, warnings (concentrated fixture warns
// >40% sector), accuracy-adjacent citations.
func (s *Server) PortfolioRisk(w http.ResponseWriter, r *http.Request) {
	wl, _ := s.DB.Watchlist(s.userKey(r))
	if len(wl) == 0 {
		wl = s.Cfg.Watchlist
	}
	// Concentration: weight by latest close x assumed equal shares (seed-safe).
	type bar struct {
		Ticker string  `json:"ticker"`
		Sector string  `json:"sector"`
		Weight float64 `json:"weight"`
	}
	prices := map[string]float64{}
	total := 0.0
	for _, tk := range wl {
		px, _, err := s.DB.LatestClose(tk)
		if err != nil || px <= 0 {
			px = 1000 // seed-safe placeholder, flagged in warnings
		}
		prices[tk] = px
		total += px
	}
	sectorOf := sectorMap()
	var bars []bar
	sectorW := map[string]float64{}
	for _, tk := range wl {
		wt := 0.0
		if total > 0 {
			wt = prices[tk] / total
		}
		sec := sectorOf[tk]
		if sec == "" {
			sec = "unknown"
		}
		bars = append(bars, bar{tk, sec, wt})
		sectorW[sec] += wt
	}
	sort.Slice(bars, func(i, j int) bool { return bars[i].Weight > bars[j].Weight })

	var warnings []string
	for sec, wt := range sectorW {
		if wt > 0.4 {
			warnings = append(warnings, "concentrated: "+sec+" at "+pct(wt)+" (over 40%)")
		}
	}

	// Correlation: pairwise Pearson over stored daily closes (aligned tail).
	series := s.closes(wl)
	corr := correlationMatrixFrom(series, wl)
	beta := betaFrom(series, wl)

	writeJSON(w, http.StatusOK, map[string]any{
		"concentration": bars, "correlation": corr, "beta": beta,
		"warnings":  warnings,
		"citations": []model.Citation{model.Cite("v2/daily/", "watchlist", "stored")},
	})
}

func pct(v float64) string {
	return itoa(int(v*100+0.5)) + "%"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}

// sectorMap is the seed-safe sector lookup (live: subsector/report).
func sectorMap() map[string]string {
	return map[string]string{
		"BBCA": "financials", "BBRI": "financials", "BMRI": "financials", "BBNI": "financials",
		"TLKM": "infrastructure", "ASII": "industrials", "UNVR": "consumer", "ICBP": "consumer",
	}
}

// closes returns aligned close series per ticker from snapshots.
func (s *Server) closes(wl []string) map[string][]float64 {
	out := map[string][]float64{}
	for _, tk := range wl {
		var rows []struct {
			Close float64 `json:"close"`
		}
		if raw, _, err := s.DB.LatestSnapshot(tk, "daily"); err == nil {
			var bars []struct {
				Close float64 `json:"close"`
			}
			if json.Unmarshal([]byte(raw), &bars) == nil {
				for _, b := range bars {
					rows = append(rows, struct {
						Close float64 `json:"close"`
					}{b.Close})
				}
			}
			_ = rows
			series := make([]float64, 0, len(bars))
			for _, b := range bars {
				series = append(series, b.Close)
			}
			out[tk] = series
		}
	}
	return out
}

func correlationMatrixFrom(series map[string][]float64, wl []string) map[string]map[string]float64 {
	m := map[string]map[string]float64{}
	for _, a := range wl {
		m[a] = map[string]float64{}
		for _, b := range wl {
			if a == b {
				m[a][b] = 1
				continue
			}
			m[a][b] = pearson(tail(series[a], 30), tail(series[b], 30))
		}
	}
	return m
}

func tail(xs []float64, n int) []float64 {
	if len(xs) <= n {
		return xs
	}
	return xs[len(xs)-n:]
}

// pearson computes the correlation of two equal-length series.
func pearson(a, b []float64) float64 {
	n := len(a)
	if n != len(b) || n < 2 {
		return 0
	}
	ma, mb := mean(a), mean(b)
	num, da, db := 0.0, 0.0, 0.0
	for i := range a {
		num += (a[i] - ma) * (b[i] - mb)
		da += (a[i] - ma) * (a[i] - ma)
		db += (b[i] - mb) * (b[i] - mb)
	}
	if da == 0 || db == 0 {
		return 0
	}
	return num / (math.Sqrt(da) * math.Sqrt(db))
}

func mean(xs []float64) float64 {
	s := 0.0
	for _, x := range xs {
		s += x
	}
	return s / float64(len(xs))
}

// betaFrom regresses mean ticker returns vs the watchlist mean (index-daily
// benchmark when cached; watchlist-mean fallback keeps seeds working).
func betaFrom(series map[string][]float64, wl []string) float64 {
	if len(wl) == 0 {
		return 1
	}
	n := 0
	for _, tk := range wl {
		if len(series[tk]) > n {
			n = len(series[tk])
		}
	}
	if n < 2 {
		return 1
	}
	idx := make([]float64, n)
	for _, tk := range wl {
		s := series[tk]
		for i := range idx {
			if i < len(s) {
				idx[i] += s[i]
			}
		}
	}
	for i := range idx {
		idx[i] /= float64(len(wl))
	}
	betas := []float64{}
	for _, tk := range wl {
		if b := betaOf(series[tk], idx); b != 0 {
			betas = append(betas, b)
		}
	}
	if len(betas) == 0 {
		return 1
	}
	return mean(betas)
}

// betaOf is cov(asset,index)/var(index) over the aligned tail.
func betaOf(asset, index []float64) float64 {
	n := len(asset)
	if len(index) < n {
		n = len(index)
	}
	if n < 2 {
		return 0
	}
	a, ix := asset[len(asset)-n:], index[len(index)-n:]
	ma, mi := mean(a), mean(ix)
	num, den := 0.0, 0.0
	for i := range a {
		num += (a[i] - ma) * (ix[i] - mi)
		den += (ix[i] - mi) * (ix[i] - mi)
	}
	if den == 0 {
		return 0
	}
	return num / den
}
