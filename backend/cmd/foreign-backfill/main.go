package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"time"

	"flowsight/internal/sectors"
	"flowsight/internal/store"
)

// one-shot foreign-flow backfill for the top-N universe tickers (by close),
// foreign-only (1 credit each) so the dashboard chart picker has data for
// hundreds of tickers immediately. Usage:
// BWS_PROJECT_ID=... sh scripts/bws-run.sh go run ./cmd/foreign-backfill <N>
func main() {
	n := 400
	if len(os.Args) > 1 {
		if v := os.Args[1]; v != "" {
			var t int
			if _, err := fmtSscan(v, &t); err == nil && t > 0 {
				n = t
			}
		}
	}
	db, err := store.Open(os.Getenv("DB_PATH"))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	c := sectors.New("https://api.sectors.app/v2/", os.Getenv("SECTORS_API_KEY"))
	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Second)
	defer cancel()
	c.OnSpend(func(endpoint string, calls, credits int) { logf("spend %d credits %s\n", credits, endpoint) })

	rows, err := db.Query(`SELECT ticker FROM universe ORDER BY close DESC LIMIT ?`, n)
	if err != nil {
		log.Fatal(err)
	}
	var tickers []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			log.Fatal(err)
		}
		tickers = append(tickers, t)
	}
	rows.Close()
	logf("backfilling foreign flow for %d tickers\n", len(tickers))

	done := 0
	now := time.Now().UTC()
	end := now.Format("2006-01-02")
	start := now.AddDate(0, 0, -30).Format("2006-01-02")
	for _, t := range tickers {
		ff, err := c.ForeignFlow(ctx, t, start, end)
		if err != nil {
			logf("  %s err: %v\n", t, err)
			time.Sleep(2 * time.Second)
			continue
		}
		if raw, err := json.Marshal(ff); err == nil && len(ff.Data) > 0 {
			_ = db.SaveSnapshot(t, end, "foreign-flow", string(raw))
			for _, d := range ff.Data {
				_ = db.UpsertForeignFlow(t, d.Date, float64(d.NetForeignInflow))
			}
			done++
		}
		time.Sleep(1200 * time.Millisecond) // stay under the 429 pace
	}
	logf("done: %d/%d tickers have foreign data now\n", done, len(tickers))
}

func logf(f string, a ...any) { log.Printf(f, a...) }

func fmtSscan(s string, v *int) (int, error) {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, errBad
		}
		n = n*10 + int(r-'0')
	}
	*v = n
	return 1, nil
}

var errBad = errT("bad number")

type errT string

func (e errT) Error() string { return string(e) }