// Seed loads one historical trading week into snapshots + derived tables so
// the full demo (briefing → radar → report → interrogation) runs offline.
// Fixtures live in tests/fixtures/ as JSON exports of real API shapes;
// seed/manifest.json maps each file to (ticker, date, source).
package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Manifest describes one historical-replay seed bundle.
type Manifest struct {
	Week      string            `json:"week"`
	Snapshots []ManifestSnap    `json:"snapshots"`
	Watchlist []string          `json:"watchlist"`
	Routines  []ManifestRoutine `json:"routines"`
}

// ManifestSnap maps one fixture file to a snapshots row.
type ManifestSnap struct {
	Ticker string `json:"ticker"`
	Date   string `json:"date"`
	Source string `json:"source"`
	File   string `json:"file"`
}

// ManifestRoutine seeds one subscribed routine.
type ManifestRoutine struct {
	UserKey  string   `json:"user_key"`
	Type     string   `json:"type"`
	Schedule string   `json:"schedule"`
	Channels []string `json:"channels"`
}

// SeedStats counts what the seed loaded.
type SeedStats struct {
	Snapshots int
	Watchlist int
	Routines  int
}

// NeedsSeed reports whether the DB has no snapshots yet.
func (db *DB) NeedsSeed() bool {
	var n int
	_ = db.QueryRow(`SELECT COUNT(*) FROM snapshots`).Scan(&n)
	return n == 0
}

// SeedFromDir loads a fixture bundle. It is idempotent for snapshots
// (skips exact ticker/date/source duplicates) so re-runs are safe.
func (db *DB) SeedFromDir(dir, userKey string) (SeedStats, error) {
	var st SeedStats
	raw, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return st, fmt.Errorf("store: read manifest: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return st, fmt.Errorf("store: decode manifest: %w", err)
	}
	for _, s := range m.Snapshots {
		body, err := os.ReadFile(filepath.Join(dir, s.File))
		if err != nil {
			return st, fmt.Errorf("store: read fixture %s: %w", s.File, err)
		}
		if !validJSON(body) {
			return st, fmt.Errorf("store: fixture %s is not valid JSON", s.File)
		}
		var n int
		_ = db.QueryRow(`SELECT COUNT(*) FROM snapshots WHERE ticker=? AND date=? AND source=?`,
			s.Ticker, s.Date, s.Source).Scan(&n)
		if n > 0 {
			continue
		}
		if err := db.SaveSnapshot(s.Ticker, s.Date, s.Source, string(body)); err != nil {
			return st, err
		}
		st.Snapshots++
		if err := db.derive(s.Ticker, s.Date, s.Source, body); err != nil {
			return st, err
		}
	}
	for _, t := range m.Watchlist {
		if err := db.AddWatch(userKey, t); err != nil {
			return st, err
		}
		st.Watchlist++
	}
	existing, _ := db.ListRoutines(userKey)
	have := map[string]bool{}
	for _, r := range existing {
		have[r.Type] = true
	}
	for _, r := range m.Routines {
		if have[r.Type] {
			continue
		}
		key := r.UserKey
		if key == "" {
			key = userKey
		}
		if _, err := db.CreateRoutine(key, r.Type, r.Schedule, r.Channels); err != nil {
			return st, err
		}
		st.Routines++
	}
	return st, nil
}

func validJSON(b []byte) bool {
	var v any
	return json.Unmarshal(b, &v) == nil
}

// derive populates typed tables from raw snapshot payloads so rules and
// agents can query without re-parsing JSON on every cycle.
func (db *DB) derive(ticker, date, source string, body []byte) error {
	switch source {
	case "broker-summary-top":
		var resp struct {
			TopBuyers []struct {
				BrokerCode string  `json:"broker_code"`
				NetIDR     float64 `json:"net_idr"`
				BuyIDR     float64 `json:"buy_idr"`
				SellIDR    float64 `json:"sell_idr"`
			} `json:"top_buyers"`
			TopSellers []struct {
				BrokerCode string  `json:"broker_code"`
				NetIDR     float64 `json:"net_idr"`
				BuyIDR     float64 `json:"buy_idr"`
				SellIDR    float64 `json:"sell_idr"`
			} `json:"top_sellers"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			return err
		}
		for _, b := range resp.TopBuyers {
			if err := db.UpsertBrokerActivity(b.BrokerCode, ticker, date,
				b.BuyIDR, b.SellIDR, b.NetIDR, 0, 0, 0); err != nil {
				return err
			}
		}
		for _, s := range resp.TopSellers {
			if err := db.UpsertBrokerActivity(s.BrokerCode, ticker, date,
				s.BuyIDR, s.SellIDR, s.NetIDR, 0, 0, 0); err != nil {
				return err
			}
		}
	case "foreign-flow":
		var resp struct {
			Data []struct {
				Date             string  `json:"date"`
				NetForeignInflow float64 `json:"net_foreign_inflow"`
			} `json:"data"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			return err
		}
		for _, d := range resp.Data {
			if err := db.UpsertForeignFlow(ticker, d.Date, d.NetForeignInflow); err != nil {
				return err
			}
		}
	case "news":
		var resp struct {
			Results []struct {
				Title     string `json:"title"`
				Source    string `json:"source"`
				Timestamp string `json:"timestamp"`
			} `json:"results"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			return err
		}
		for _, a := range resp.Results {
			d := date
			if len(a.Timestamp) >= 10 {
				d = a.Timestamp[:10]
			}
			if err := db.InsertNews(ticker, d, a.Source, "neutral", 0.5, a.Source, a.Title); err != nil {
				return err
			}
		}
	case "filings":
		var resp struct {
			Results []struct {
				Timestamp         string  `json:"timestamp"`
				TransactionType   string  `json:"transaction_type"`
				HolderType        string  `json:"holder_type"`
				HolderName        string  `json:"holder_name"`
				AmountTransaction float64 `json:"amount_transaction"`
				Price             float64 `json:"price"`
			} `json:"results"`
		}
		if err := json.Unmarshal(body, &resp); err != nil {
			return err
		}
		for _, f := range resp.Results {
			d := date
			if len(f.Timestamp) >= 10 {
				d = f.Timestamp[:10]
			}
			if err := db.InsertFiling(ticker, d, f.HolderType, f.TransactionType,
				f.AmountTransaction, f.Price, f.HolderName); err != nil {
				return err
			}
		}
	}
	return nil
}
