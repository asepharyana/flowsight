package alerts

import (
	"context"
	"testing"

	"flowsight/internal/store"
)

func seedEvalDB(t *testing.T) *store.DB {
	t.Helper()
	db, err := store.Open(t.TempDir() + "/eval.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.SeedFromDir("../../tests/fixtures", "demo"); err != nil {
		t.Fatal(err)
	}
	return db
}

// Accumulation fixture fires with real citations, then first-fire suppresses.
func TestEvaluateAccumulationFirstFire(t *testing.T) {
	db := seedEvalDB(t)
	n := NewNotifier("", "", "")
	id, _ := db.CreateAlert("demo", "all", `{"all":true}`, nil)
	got := Evaluate(context.Background(), db, n, id, `{"all":true}`, []string{"BBCA"})
	found := false
	for _, f := range got {
		if f.Rule == RuleAccumulation {
			found = true
			if len(f.Citations) == 0 {
				t.Fatal("accumulation finding has no citations")
			}
		}
	}
	if !found {
		t.Fatalf("no accumulation finding in %v", got)
	}
	again := Evaluate(context.Background(), db, n, id, `{"all":true}`, []string{"BBCA"})
	for _, f := range again {
		if f.Rule == RuleAccumulation {
			t.Fatal("accumulation re-fired same day")
		}
	}
}

// Rotation is reachable: sector-flow flip yields a cited finding.
func TestEvaluateRotationReachable(t *testing.T) {
	db := seedEvalDB(t)
	n := NewNotifier("", "", "")
	id, _ := db.CreateAlert("demo", "all", `{"all":true}`, nil)
	got := Evaluate(context.Background(), db, n, id, `{"all":true}`, []string{"BBCA"})
	found := false
	for _, f := range got {
		if f.Rule == RuleRotation {
			found = true
		}
	}
	if !found {
		t.Fatalf("no rotation finding in %v", ruleIDs(got))
	}
}

func ruleIDs(fs []Finding) []string {
	var out []string
	for _, f := range fs {
		out = append(out, f.Rule+":"+f.Ticker)
	}
	return out
}

// Per-owner delivery: finding fans out to the owner's destinations, and
// history rows carry the owner key (no cross-user leakage on alert_id reuse).
func TestEvaluateForOwnerDelivery(t *testing.T) {
	db := seedEvalDB(t)
	var sentTo string
	var sent int
	n := NewNotifier("", "", "")
	n.Targets = func(owner string) []Target {
		sentTo = owner
		sent++
		return nil // record-only; just observe routing
	}
	id, _ := db.CreateAlert("alice", "all", `{"all":true}`, nil)
	got := EvaluateFor(context.Background(), db, n, id, "alice", `{"all":true}`, []string{"BBCA"})
	if len(got) == 0 {
		t.Fatal("want findings on seed")
	}
	if sent == 0 || sentTo != "alice" {
		t.Fatalf("delivery not routed to owner: sent=%d to=%q", sent, sentTo)
	}
	evts, _ := db.AlertEventsSince("2000-01-01", "", 50, "alice")
	if len(evts) == 0 {
		t.Fatal("history rows must carry the owner key")
	}
	if evts2, _ := db.AlertEventsSince("2000-01-01", "", 50, "bob"); len(evts2) != 0 {
		t.Fatal("bob must not see alice events")
	}
}
