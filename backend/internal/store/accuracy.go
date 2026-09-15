package store

import (
	"sort"
)

// ResolveDue resolves predictions older than 30d against actual return:
// first stored close on/after predict_date vs latest stored close.
// BUY hits above +5%, AVOID hits below -5%, HOLD hits within ±10%.
// Returns the number of newly resolved rows.
func (db *DB) ResolveDue() (int, error) {
	due, err := db.DueForResolution()
	if err != nil {
		return 0, err
	}
	resolved := 0
	for _, p := range due {
		id, _ := p["id"].(int64)
		ticker, _ := p["ticker"].(string)
		pred, _ := p["prediction"].(string)
		since, _ := p["predict_date"].(string)
		series, err := db.dailyCloses(ticker)
		if err != nil || len(series) < 2 {
			continue
		}
		base, ok := firstOnOrAfter(series, since)
		if !ok {
			continue
		}
		last := series[len(series)-1]
		if base.close <= 0 {
			continue
		}
		ret := (last.close - base.close) / base.close
		hit := false
		switch pred {
		case "BUY":
			hit = ret > 0.05
		case "AVOID":
			hit = ret < -0.05
		default: // HOLD
			hit = ret >= -0.10 && ret <= 0.10
		}
		if err := db.ResolvePrediction(id, hit, ret); err == nil {
			resolved++
		}
	}
	return resolved, nil
}

type closePoint struct {
	date  string
	close float64
}

// dailyCloses parses every stored daily payload into a date-sorted series.
func (db *DB) dailyCloses(ticker string) ([]closePoint, error) {
	rows, err := db.Query(`SELECT payload_json FROM snapshots WHERE ticker=? AND source='daily'`, ticker)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []closePoint
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		for _, b := range parseDailyBars(raw, "") {
			if b.Date != "" && b.Close > 0 {
				out = append(out, closePoint{b.Date, b.Close})
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].date < out[j].date })
	return out, nil
}

func firstOnOrAfter(series []closePoint, date string) (closePoint, bool) {
	for _, p := range series {
		if p.date >= date {
			return p, true
		}
	}
	return closePoint{}, false
}
