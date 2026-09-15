package store

import (
	"testing"
	"time"
)

// ResolveDue: BUY older than 30d resolves against stored closes.
func TestResolveDue(t *testing.T) {
	db, err := Open(t.TempDir() + "/acc.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	old := time.Now().AddDate(0, 0, -40).Format("2006-01-02")
	if _, err := db.Exec(`INSERT INTO agent_accuracy(agent,ticker,prediction,predict_date)
		VALUES('synthesizer','BBCA','BUY',?)`, old); err != nil {
		t.Fatal(err)
	}
	// Rising closes: first bar 9000 -> last 9950 (+10.5%) => BUY hits.
	payload := `[{"date":"` + old + `","close":9000,"volume":1},{"date":"` +
		time.Now().Format("2006-01-02") + `","close":9950,"volume":1}]`
	if err := db.SaveSnapshot("BBCA", time.Now().Format("2006-01-02"), "daily", payload); err != nil {
		t.Fatal(err)
	}
	n, err := db.ResolveDue()
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("resolved = %d, want 1", n)
	}
	stats, _ := db.AccuracyStats()
	if len(stats) != 1 || stats[0]["hits"] != 1 {
		t.Fatalf("stats = %v, want 1 hit", stats)
	}
	// Second run: nothing due.
	n, _ = db.ResolveDue()
	if n != 0 {
		t.Fatalf("resolved = %d, want 0", n)
	}
}
