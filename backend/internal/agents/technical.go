package agents

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"flowsight/internal/model"
	"flowsight/internal/sectors"
)

// AnalyzeTechnical (A5) scores momentum + volume anomaly + liquidity.
// Anomaly: volume > 2x 20d avg. Liquidity grade from free-float %.
func AnalyzeTechnical(ctx context.Context, d Deps, ticker string) model.AgentResult {
	_ = ctx
	ticker = strings.ToUpper(ticker)
	res := model.AgentResult{Summary: "no technical snapshots available"}

	var daily []sectors.DailyBar
	dailyDate, dailyOK := payload(d.DB, ticker, "daily", &daily)
	var movers struct {
		TopGainers map[string][]sectors.MoverRow `json:"top_gainers"`
		TopLosers  map[string][]sectors.MoverRow `json:"top_losers"`
	}
	moverDate, moverOK := payload(d.DB, "IDX", "top-changes", &movers)
	if !dailyOK && !moverOK {
		if vols, dates, err := d.DB.DailyVolumes(ticker, 25); err == nil && len(vols) > 0 {
			return technicalFromStored(ticker, vols, dates)
		}
		return res
	}
	if dailyOK {
		res.Citations = append(res.Citations, model.Cite("v2/daily/"+ticker+"/", ticker, dailyDate))
	}
	if moverOK {
		res.Citations = append(res.Citations, model.Cite("v2/companies/top-changes/", "IDX", moverDate))
	}

	momentum := "flat"
	moverRank := ""
	if moverOK {
		for period, rows := range movers.TopGainers {
			for i, r := range rows {
				if strings.HasPrefix(strings.ToUpper(r.Symbol), ticker) {
					momentum = "up"
					moverRank = fmt.Sprintf("top-gainer #%d (%s)", i+1, period)
				}
			}
		}
		for period, rows := range movers.TopLosers {
			for i, r := range rows {
				if strings.HasPrefix(strings.ToUpper(r.Symbol), ticker) {
					momentum = "down"
					moverRank = fmt.Sprintf("top-loser #%d (%s)", i+1, period)
				}
			}
		}
	}

	volMult, volDate, lastVol := 0.0, "", 0.0
	if len(daily) >= 21 {
		win := daily
		if len(win) > 60 {
			win = win[len(win)-60:]
		}
		base := avgVol(win[:len(win)-1], 20)
		last := win[len(win)-1]
		lastVol = float64(last.Volume)
		if base > 0 {
			volMult = lastVol / base
			volDate = last.Date
		}
	}

	// Price momentum over the window: last close vs first close.
	priceChg := 0.0
	if len(daily) >= 2 {
		first, last := daily[0], daily[len(daily)-1]
		if first.Close > 0 {
			priceChg = float64(last.Close-first.Close) / float64(first.Close)
		}
	}
	if momentum == "flat" {
		switch {
		case priceChg > 0.05:
			momentum = "up"
		case priceChg < -0.05:
			momentum = "down"
		}
	}
	if momentum == "up" && priceChg > 0.15 {
		momentum = "strong"
	}

	// Relative volume vs market: ticker's latest volume against the
	// most-traded median for the same session.
	relVol := 0.0
	if med, mtDate, ok := mostTradedMedian(d.DB); ok {
		res.Citations = append(res.Citations, model.Cite("v2/most-traded/", "IDX", mtDate))
		if med > 0 && lastVol > 0 {
			relVol = lastVol / med
			res.Values = append(res.Values, model.Value{
				Label:     "relative volume",
				Display:   fmt.Sprintf("%.1fx most-traded median", relVol),
				Citations: res.Citations,
			})
		}
	}

	liquidity := "unknown"
	var ff []sectors.FreeFloatRow
	if ffDate, ok := payload(d.DB, "IDX", "free-float", &ff); ok {
		res.Citations = append(res.Citations, model.Cite("v2/free-float/", "IDX", ffDate))
		for _, r := range ff {
			if strings.HasPrefix(strings.ToUpper(r.Symbol), ticker) {
				switch {
				case r.FreeFloat >= 0.4:
					liquidity = "A"
				case r.FreeFloat >= 0.25:
					liquidity = "B"
				case r.FreeFloat >= 0.1:
					liquidity = "C"
				default:
					liquidity = "D"
				}
				res.Values = append(res.Values, model.Value{
					Label:     "free float",
					Display:   fmt.Sprintf("%.0f%% (grade %s)", r.FreeFloat*100, liquidity),
					Citations: res.Citations,
				})
			}
		}
	}

	if volMult > 2 {
		res.Flags = append(res.Flags, "volume-anomaly")
	}

	score := priceChg * 300
	if volMult > 1 {
		score += (volMult - 1) * 10
	}
	switch momentum {
	case "strong":
		score += 15
	case "up":
		score += 8
	case "down":
		score -= 8
	}
	res.Score = clampScore(score, -100, 100)

	res.Values = append([]model.Value{{
		Label:     "momentum",
		Display:   fmt.Sprintf("%s (%+.1f%% window)", momentum, priceChg*100),
		Citations: res.Citations,
	}}, res.Values...)
	if volMult > 0 {
		res.Values = append(res.Values, model.Value{
			Label:     "volume anomaly",
			Display:   fmt.Sprintf("%.1fx 20d avg on %s", volMult, volDate),
			Citations: res.Citations,
		})
	}
	if moverRank != "" {
		res.Values = append(res.Values, model.Value{Label: "mover rank", Display: moverRank, Citations: res.Citations})
	}
	res.Summary = fmt.Sprintf("%s momentum %+.1f%%, volume %.1fx, liquidity %s",
		momentum, priceChg*100, volMult, liquidity)
	res.Extra = map[string]any{
		"momentum": momentum, "price_change": priceChg,
		"volume_mult": volMult, "volume_date": volDate, "liquidity": liquidity,
		"rel_volume": relVol,
	}
	return res
}

// mtRow is one most-traded entry (volume in shares).
type mtRow struct {
	Symbol string  `json:"symbol"`
	Volume float64 `json:"volume"`
}

// mostTradedMedian returns the median volume across the cached most-traded
// snapshot plus its snapshot date. Accepts both stored shapes: the wrapped
// {results:[...]} form and the bare array the scheduler persists.
func mostTradedMedian(db interface {
	LatestSnapshot(ticker, source string) (string, string, error)
}) (med float64, date string, ok bool) {
	raw, d, err := db.LatestSnapshot("IDX", "most-traded")
	if err != nil || raw == "" {
		return 0, "", false
	}
	var vols []float64
	var wrapped struct {
		Results []mtRow `json:"results"`
	}
	if json.Unmarshal([]byte(raw), &wrapped) == nil && len(wrapped.Results) > 0 {
		for _, r := range wrapped.Results {
			if r.Volume > 0 {
				vols = append(vols, r.Volume)
			}
		}
	} else {
		var rows []mtRow
		if json.Unmarshal([]byte(raw), &rows) != nil {
			return 0, "", false
		}
		for _, r := range rows {
			if r.Volume > 0 {
				vols = append(vols, r.Volume)
			}
		}
	}
	if len(vols) == 0 {
		return 0, "", false
	}
	sort.Float64s(vols)
	m := vols[len(vols)/2]
	if len(vols)%2 == 0 {
		m = (vols[len(vols)/2-1] + vols[len(vols)/2]) / 2
	}
	return m, d, true
}

func avgVol(bars []sectors.DailyBar, n int) float64 {
	if len(bars) < n {
		n = len(bars)
	}
	if n == 0 {
		return 0
	}
	sum := 0.0
	for _, b := range bars[len(bars)-n:] {
		sum += float64(b.Volume)
	}
	return sum / float64(n)
}

// technicalFromStored derives momentum from stored snapshot volumes.
func technicalFromStored(ticker string, vols []float64, dates []string) model.AgentResult {
	res := model.AgentResult{}
	last := vols[len(vols)-1]
	base := avg(vols[:len(vols)-1])
	mult := 0.0
	if base > 0 {
		mult = last / base
	}
	res.Score = clampScore((mult-1)*20, -100, 100)
	res.Citations = []model.Citation{model.Cite("v2/daily/"+ticker+"/", ticker, "stored")}
	date := ""
	if len(dates) > 0 {
		date = dates[len(dates)-1]
	}
	res.Values = []model.Value{{
		Label:     "volume anomaly",
		Display:   fmt.Sprintf("%.1fx 20d avg on %s", mult, date),
		Citations: res.Citations,
	}}
	if mult > 2 {
		res.Flags = append(res.Flags, "volume-anomaly")
	}
	res.Summary = fmt.Sprintf("stored-volume momentum %.1fx", mult)
	res.Extra = map[string]any{"volume_mult": mult, "volume_date": date}
	return res
}
