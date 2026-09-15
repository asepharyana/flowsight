package alerts

import "testing"

// Accumulation fixture: 3 brokers net-buy + 1.6x volume fires.
func TestAccumulationFires(t *testing.T) {
	nets := map[string]float64{"MG": 5e11, "AK": 4e11, "CC": 3e11, "BK": -1e11}
	f, ok := Accumulation("BBCA", nets, 1.6)
	if !ok {
		t.Fatal("want accumulation fire")
	}
	if f.Rule != RuleAccumulation {
		t.Fatalf("rule = %s", f.Rule)
	}
}

func TestAccumulationSilent(t *testing.T) {
	nets := map[string]float64{"MG": 5e11, "BK": -4e11}
	if _, ok := Accumulation("BBCA", nets, 1.1); ok {
		t.Fatal("want silence: only 1 net-buy broker, low volume")
	}
}

// Foreign reversal: 5d outflow then strong inflow fires.
func TestForeignReversal(t *testing.T) {
	last6 := []float64{-1e11, -1e11, -1e11, -1e11, -1e11, 3.4e11}
	if _, ok := ForeignReversal("BBCA", last6); !ok {
		t.Fatal("want reversal fire")
	}
	if _, ok := ForeignReversal("BBCA", []float64{1e11, 1e11}); ok {
		t.Fatal("want silence on short series")
	}
}

// Insider spike both paths.
func TestInsiderSpike(t *testing.T) {
	if _, ok := InsiderSpike("BBCA", 3e6, 1e6, 1); !ok {
		t.Fatal("want volume-path fire")
	}
	if _, ok := InsiderSpike("BBCA", 0, 0, 3); !ok {
		t.Fatal("want cluster-path fire")
	}
	if _, ok := InsiderSpike("BBCA", 1e6, 1e6, 1); ok {
		t.Fatal("want silence")
	}
}

// Unusual volume suppressed on earnings dates.
func TestUnusualVolume(t *testing.T) {
	if _, ok := UnusualVolume("BBCA", "2026-09-11", 3.2, false); !ok {
		t.Fatal("want fire")
	}
	if _, ok := UnusualVolume("BBCA", "2026-09-11", 3.2, true); ok {
		t.Fatal("want silence on earnings date")
	}
}

// Rotation sign flip.
func TestRotation(t *testing.T) {
	if _, ok := SectorRotation("consumer", -3e8, 1.1e9); !ok {
		t.Fatal("want rotation fire")
	}
	if _, ok := SectorRotation("banks", 1e9, 2e9); ok {
		t.Fatal("want silence without flip")
	}
}

// Suspension watch: watchlist fires, others silent.
func TestSuspension(t *testing.T) {
	if _, ok := SuspensionWatch("TLKM", "2026-09-10", "volatilitas", true); !ok {
		t.Fatal("want fire on watchlist")
	}
	if _, ok := SuspensionWatch("TLKM", "2026-09-10", "volatilitas", false); ok {
		t.Fatal("want silence off-watchlist")
	}
}
