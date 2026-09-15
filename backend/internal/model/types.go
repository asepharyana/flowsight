// Package model holds shared domain types: citations, snapshots, agent
// results. Every number in user-visible output carries a Citation.
// API response shapes live in internal/sectors; agent inputs in internal/agents.
package model

import "time"

// Citation pins a value to its source endpoint + snapshot timestamp.
// Stale (>1 session old) citations are rendered marked, never hidden.
type Citation struct {
	Endpoint   string `json:"endpoint"`
	SnapshotAt string `json:"snapshot_at"`
	Ticker     string `json:"ticker,omitempty"`
	Stale      bool   `json:"stale,omitempty"`
}

// Value is a cited number or string shown to users.
type Value struct {
	Label     string     `json:"label"`
	Display   string     `json:"display"`
	Citations []Citation `json:"citations"`
}

// Snapshot is one raw Sectors payload row.
type Snapshot struct {
	ID        int64
	Ticker    string
	Date      string
	Source    string
	Payload   string
	FetchedAt time.Time
}

// AgentResult is the common output contract for A1..A7.
type AgentResult struct {
	Agent     string         `json:"agent"`
	Score     float64        `json:"score"`
	Summary   string         `json:"summary"`
	Values    []Value        `json:"values"`
	Flags     []string       `json:"flags"`
	Citations []Citation     `json:"citations"`
	Extra     map[string]any `json:"extra,omitempty"`
}

// Cite builds a Citation for one endpoint + snapshot date.
func Cite(endpoint, ticker, snapshotAt string) Citation {
	return Citation{Endpoint: endpoint, SnapshotAt: snapshotAt, Ticker: ticker}
}

// StaleSession returns the date before which snapshots count as stale:
// anything older than the session containing ref (WIB trading day).
func StaleSession(ref time.Time) time.Time {
	y, m, d := ref.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, ref.Location()).AddDate(0, 0, -1)
}

// MarkStale flags citations older than cutoff (one session back).
func MarkStale(citations []Citation, cutoff time.Time) []Citation {
	out := make([]Citation, len(citations))
	for i, c := range citations {
		if t, err := time.Parse("2006-01-02", head10(c.SnapshotAt)); err == nil && t.Before(cutoff) {
			c.Stale = true
		}
		out[i] = c
	}
	return out
}

func head10(s string) string {
	if len(s) > 10 {
		return s[:10]
	}
	return s
}
