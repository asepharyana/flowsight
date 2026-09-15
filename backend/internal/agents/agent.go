// Package agents implements the 7 FlowSight specialists (A1..A6) plus the
// Master Synthesizer (A7). Contract per docs/AGENT-SPECS.md:
//
//	analyze(ticker, snapshots) -> AgentResult(values[], score, citations[])
//
// Agents never fetch live; they read snapshots from the store. Detection
// rules and scores are computed locally; the LLM only refines prose and
// never invents numbers (every value carries citations).
package agents

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"flowsight/internal/llm"
	"flowsight/internal/model"
	"flowsight/internal/store"
)

// Deps wires one analysis run.
type Deps struct {
	DB          *store.DB
	LLM         *llm.Client
	TriageModel string
	SynthModel  string
	Now         time.Time
}

// Analyzer is one specialist.
type Analyzer func(ctx context.Context, d Deps, ticker string) model.AgentResult

// Registry runs in fixed order A1..A6; the synthesizer (A7) runs after.
var Registry = []struct {
	Name string
	Fn   Analyzer
}{
	{"smart-money", AnalyzeSmartMoney},
	{"broker-intel", AnalyzeBrokerIntel},
	{"sentiment", AnalyzeSentiment},
	{"fundamental", AnalyzeFundamental},
	{"technical", AnalyzeTechnical},
	{"catalyst", AnalyzeCatalyst},
}

// RunAll executes A1..A6 in parallel via goroutines and returns results in
// registry order. One failing agent yields a zero-score result with the error
// in Summary — it never aborts the other five.
func RunAll(ctx context.Context, d Deps, ticker string) []model.AgentResult {
	out := make([]model.AgentResult, len(Registry))
	var wg sync.WaitGroup
	for i, a := range Registry {
		wg.Add(1)
		go func(i int, name string, fn Analyzer) {
			defer wg.Done()
			res := fn(ctx, d, ticker)
			res.Agent = name
			out[i] = res
		}(i, a.Name, a.Fn)
	}
	wg.Wait()
	return out
}

// payload loads the newest snapshot for (ticker, source) and unmarshals it.
// ok=false when no snapshot exists (agents treat missing input as neutral,
// never as an error — the Citations list simply stays short).
func payload(db *store.DB, ticker, source string, v any) (date string, ok bool) {
	raw, d, err := db.LatestSnapshot(ticker, source)
	if err != nil || raw == "" {
		return "", false
	}
	if err := json.Unmarshal([]byte(raw), v); err != nil {
		return "", false
	}
	return d, true
}

// cite builds one citation for (source-as-endpoint, ticker, snapshot date).
func cite(source, ticker, date string) model.Citation {
	return model.Cite("v2/"+source+"/", ticker, date)
}

// avg returns the mean of xs (0 on empty).
func avg(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	sum := 0.0
	for _, x := range xs {
		sum += x
	}
	return sum / float64(len(xs))
}

// clampScore bounds a score to [lo, hi].
func clampScore(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// fail builds an error result that keeps the pipeline green.
func fail(agent, msg string) model.AgentResult {
	return model.AgentResult{Agent: agent, Summary: "error: " + msg}
}
