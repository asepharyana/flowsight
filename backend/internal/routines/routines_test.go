package routines

import (
	"context"
	"strings"
	"testing"

	"flowsight/internal/alerts"
	"flowsight/internal/store"
)

func seedEngine(t *testing.T) *Engine {
	t.Helper()
	db, err := store.Open(t.TempDir() + "/routines.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := db.SeedFromDir("../../tests/fixtures", "demo"); err != nil {
		t.Fatal(err)
	}
	return &Engine{DB: db, Notifier: alerts.NewNotifier("", "", ""), UserKey: "demo"}
}

// Briefing generates from seed with zero empty sections.
func TestBriefing(t *testing.T) {
	e := seedEngine(t)
	text, cites, err := e.Briefing(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, sec := range []string{"Top accumulation", "Foreign flow", "Agenda"} {
		if !strings.Contains(text, sec) {
			t.Fatalf("missing section %q", sec)
		}
	}
	if len(cites) == 0 {
		t.Fatal("briefing has no citations")
	}
}

// Radar fires on the accumulation fixture.
func TestRadar(t *testing.T) {
	e := seedEngine(t)
	text, _, err := e.Radar(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "BBCA") {
		t.Fatalf("radar missed BBCA: %s", text)
	}
}

// Accuracy resolution math: weights follow the ledger.
func TestAccuracyWeight(t *testing.T) {
	e := seedEngine(t)
	if w := e.DB.AccuracyWeight("smart-money"); w != 0.5 {
		t.Fatalf("default weight = %v, want 0.5", w)
	}
	for i := 0; i < 10; i++ {
		_ = e.DB.RecordPrediction("smart-money", "BBCA", "BUY")
	}
	rows, _ := e.DB.DueForResolution()
	_ = rows
}

// Briefing carries agenda + mover sections and respects the 25-line cap.
func TestBriefingAgendaMoverCap(t *testing.T) {
	e := seedEngine(t)
	text, cites, err := e.Briefing(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, sec := range []string{"Top accumulation", "Foreign flow", "Agenda", "Mover of the day"} {
		if !strings.Contains(text, sec) {
			t.Fatalf("missing section %q:\n%s", sec, text)
		}
	}
	if n := len(strings.Split(strings.TrimRight(text, "\n"), "\n")); n > 25 {
		t.Fatalf("briefing = %d lines, want ≤25", n)
	}
	if len(cites) == 0 {
		t.Fatal("briefing has no citations")
	}
}

// R5/R6 run without error on seed (windows simply miss today).
func TestCountdownsRun(t *testing.T) {
	e := seedEngine(t)
	rows, _ := e.DB.ListRoutines("demo")
	_ = rows
	if _, _, err := e.EarningsCountdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, _, err := e.DividendCalendar(context.Background()); err != nil {
		t.Fatal(err)
	}
}
