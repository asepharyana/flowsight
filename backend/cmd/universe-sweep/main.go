package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"flowsight/internal/sectors"
	"flowsight/internal/store"
)

// one-shot: sweep the full IDX universe (close/ pages) and persist it into
// the universe table. Usage: BWS_PROJECT_ID=... sh scripts/bws-run.sh go run ./cmd/universe-sweep
func main() {
	// env DB_PATH defaults to data/flowsight.db if unset.
	db, err := store.Open(os.Getenv("DB_PATH"))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	c := sectors.New("https://api.sectors.app/v2/", os.Getenv("SECTORS_API_KEY"))
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	c.OnSpend(func(endpoint string, calls, credits int) { fmt.Printf("spend %d credits %s\n", credits, endpoint) })

	offset := 0
	var all []sectors.CloseRow
	for page := 0; page < 60; page++ {
		rows, _, err := c.ClosePage(ctx, "", 30, offset)
		if err != nil {
			log.Printf("page %d err: %v (keeping %d so far)", page, err, len(all))
			break
		}
		all = append(all, rows...)
		offset += len(rows)
		fmt.Printf("page %d: +%d (total %d)\n", page, len(rows), len(all))
		if len(rows) == 0 {
			break
		}
		time.Sleep(1500 * time.Millisecond) // stay well under the 429 pace
	}
	fmt.Println("TOTAL:", len(all))
	if len(all) == 0 {
		log.Fatal("no rows collected")
	}
	date := all[0].Date
	for _, r := range all {
		if r.Date > date {
			date = r.Date
		}
	}
	raw, _ := json.Marshal(all)
	_ = db.SaveSnapshot("IDX", date, "close", string(raw))
	uni := make([]store.UniverseRow, 0, len(all))
	for _, r := range all {
		uni = append(uni, store.UniverseRow{Symbol: r.Symbol, Close: r.Close, Date: r.Date})
	}
	if err := db.SaveUniverse(uni); err != nil {
		log.Fatal(err)
	}
	fmt.Println("universe saved:", len(uni), "tickers @", date)
}