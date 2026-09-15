package store

import (
	"testing"
)

// Seed loads the historical week; row counts must match the fixture manifest.
func TestSeed(t *testing.T) {
	db, err := Open(t.TempDir() + "/seed.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	st, err := db.SeedFromDir("../../tests/fixtures", "demo")
	if err != nil {
		t.Fatal(err)
	}
	if st.Snapshots != 23 {
		t.Fatalf("snapshots = %d, want 23", st.Snapshots)
	}
	if st.Watchlist != 2 {
		t.Fatalf("watchlist = %d, want 2", st.Watchlist)
	}
	if got := db.SnapshotCount("daily"); got != 1 {
		t.Fatalf("daily snapshots = %d, want 1", got)
	}
	// Idempotent re-seed.
	st2, err := db.SeedFromDir("../../tests/fixtures", "demo")
	if err != nil {
		t.Fatal(err)
	}
	if st2.Snapshots != 0 {
		t.Fatalf("re-seed snapshots = %d, want 0", st2.Snapshots)
	}
}
