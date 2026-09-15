package store

import "testing"

// Retention prune keeps newest per (ticker, source) and deletes old rows.
func TestPruneSnapshots(t *testing.T) {
	db, err := Open(t.TempDir() + "/prune.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_ = db.SaveSnapshot("OLD", "2020-01-01", "daily", `[{"close":1}]`)
	_ = db.SaveSnapshot("OLD", "2026-09-11", "daily", `[{"close":2}]`)
	_ = db.SaveSnapshot("NEW", "2026-09-11", "daily", `[{"close":3}]`)
	n, err := db.PruneSnapshots("2026-01-01")
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("pruned = %d, want 1", n)
	}
	if _, _, err := db.LatestSnapshot("OLD", "daily"); err != nil {
		t.Fatal("newest OLD row must survive")
	}
}

// DeleteRoutine + FilingsSince + ListReports parity.
func TestStoreParity(t *testing.T) {
	db, err := Open(t.TempDir() + "/parity.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	id, _ := db.CreateRoutine("demo", "weekend-review", "0 9 * * 6", nil)
	ok, err := db.DeleteRoutine(id, "demo")
	if err != nil || !ok {
		t.Fatalf("delete = %v, %v", ok, err)
	}
	if ok2, _ := db.DeleteRoutine(999999, "demo"); ok2 {
		t.Fatal("delete of missing id must report false")
	}
	rows, _ := db.ListRoutines("demo")
	for _, r := range rows {
		if r.ID == id {
			t.Fatal("routine not deleted")
		}
	}
	_ = db.InsertFiling("BBCA", "2026-09-10", "insider", "buy", 500000, 9850)
	fils, err := db.FilingsSince("BBCA", "2026-09-01", 10)
	if err != nil || len(fils) != 1 {
		t.Fatalf("filings = %v, %v", fils, err)
	}
	_, _ = db.SaveReport("BBCA", `{"a":1}`, `[]`)
	reps, err := db.ListReports("BBCA", 10)
	if err != nil || len(reps) != 1 {
		t.Fatalf("reports = %v, %v", reps, err)
	}
}

// Destination CRUD: masks nothing at store layer, scopes by owner, kind immutable via API.
func TestDestinationCRUD(t *testing.T) {
	db, err := Open(t.TempDir() + "/dest.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	id, err := db.CreateDestination(Destination{UserKey: "u1", Kind: DestTelegram, Label: "bot", BotToken: "tok", ChatID: "123", Enabled: true})
	if err != nil || id == 0 {
		t.Fatalf("create = %d, %v", id, err)
	}
	rows, _ := db.ListDestinations("u1")
	if len(rows) != 1 || rows[0].BotToken != "tok" {
		t.Fatalf("list = %+v", rows)
	}
	if other, _ := db.ListDestinations("u2"); len(other) != 0 {
		t.Fatal("cross-user leak")
	}
	en, _ := db.ListEnabledDestinations("u1")
	if len(en) != 1 {
		t.Fatal("enabled list must include it")
	}
	off := false
	ok, err := db.UpdateDestination(id, "u1", nil, &off, nil, nil, nil)
	if err != nil || !ok {
		t.Fatalf("disable = %v, %v", ok, err)
	}
	if en, _ := db.ListEnabledDestinations("u1"); len(en) != 0 {
		t.Fatal("disabled row must drop from enabled list")
	}
	// Omitted secrets keep stored values.
	ok, err = db.UpdateDestination(id, "u1", nil, nil, nil, nil, nil)
	if err != nil || !ok {
		t.Fatalf("noop patch = %v, %v", ok, err)
	}
	got, _ := db.GetDestination(id, "u1")
	if got == nil || got.BotToken != "tok" {
		t.Fatalf("secrets must persist on empty patch: %+v", got)
	}
	if ok, _ := db.UpdateDestination(id, "u2", nil, nil, nil, nil, nil); ok {
		t.Fatal("cross-user patch must report false")
	}
	ok, err = db.DeleteDestination(id, "u1")
	if err != nil || !ok {
		t.Fatalf("delete = %v, %v", ok, err)
	}
	if ok, _ := db.DeleteDestination(id, "u1"); ok {
		t.Fatal("double delete must report false")
	}
}
