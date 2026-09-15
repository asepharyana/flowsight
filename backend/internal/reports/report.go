// Package reports builds the 7-section One-Click Report (overview, valuation,
// institutional, earnings, risk, calendar, recommendation) with citations[]
// per section, and exports PDF/HTML/MD/JSON. Every section populates from
// live/seed data; nothing renders empty without saying which snapshot is
// missing.
package reports

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"flowsight/internal/agents"
	"flowsight/internal/model"
	"flowsight/internal/store"
)

// Section is one report section with its own citations.
type Section struct {
	Name      string           `json:"name"`
	Body      string           `json:"body"`
	Citations []model.Citation `json:"citations"`
}

// Report is the full 7-section payload.
type Report struct {
	Ticker       string              `json:"ticker"`
	GeneratedAt  string              `json:"generated_at"`
	Sections     []Section           `json:"sections"`
	Synthesis    agents.Synthesis    `json:"synthesis"`
	AgentScores  []model.AgentResult `json:"agent_scores"`
	AllCitations []model.Citation    `json:"citations"`
	NarasiAwam   string              `json:"narasi_awam,omitempty"`
}

// Builder assembles reports from agent runs over stored snapshots.
type Builder struct {
	DB   *store.DB
	Deps agents.Deps
	Risk RiskFn
}

// RiskFn computes the portfolio-risk lines reused in the risk section.
type RiskFn func(ticker string) (concentration, beta string)

// Build runs A1..A6 + A7 and assembles all 7 sections, returning the
// persisted report id for citation-scoped chat interrogation.
func (b *Builder) Build(ctx context.Context, ticker, profile string) (Report, int64, error) {
	ticker = strings.ToUpper(ticker)
	results := agents.RunAll(ctx, b.Deps, ticker)
	synth := agents.Synthesize(ctx, b.Deps, ticker, agents.RiskProfile(profile), results)

	byAgent := map[string]model.AgentResult{}
	for _, r := range results {
		byAgent[r.Agent] = r
	}
	sec := func(name, body string, cites []model.Citation) Section {
		if strings.TrimSpace(body) == "" {
			body = "no snapshots for this section yet"
		}
		return Section{Name: name, Body: body, Citations: cites}
	}
	sections := []Section{
		sec("overview", overviewLine(ticker, byAgent), citesOf(byAgent, "sentiment", "technical")),
		sec("valuation", byAgent["fundamental"].Summary, citesOf(byAgent, "fundamental")),
		sec("institutional", institutionalLine(byAgent), citesOf(byAgent, "smart-money", "broker-intel")),
		sec("earnings", byAgent["fundamental"].Summary+" | "+byAgent["catalyst"].Summary, citesOf(byAgent, "fundamental", "catalyst")),
		sec("risk", riskLine(ticker, b.Risk), citesOf(byAgent, "technical")),
		sec("calendar", byAgent["catalyst"].Summary, citesOf(byAgent, "catalyst")),
		sec("recommendation", synth.Thesis, synth.Citations),
	}
	all := []model.Citation{}
	for _, s := range sections {
		all = append(all, s.Citations...)
	}
	all = model.MarkStale(all, model.StaleSession(time.Now()))
	narasi := b.narasiAwam(ctx, ticker, synth, sections)
	raw, _ := json.Marshal(map[string]any{"ticker": ticker, "sections": sections, "synthesis": synth, "narasi_awam": narasi})
	citesRaw, _ := json.Marshal(all)
	id, err := b.DB.SaveReport(ticker, string(raw), string(citesRaw))
	if err != nil {
		return Report{}, 0, err
	}
	return Report{
		Ticker: ticker, GeneratedAt: todayStr(),
		Sections: sections, Synthesis: synth,
		AgentScores: results, AllCitations: all,
		NarasiAwam: narasi,
	}, id, nil
}

// narasiAwam polishes the rule-based synthesis into plain Indonesian for
// non-experts (verdict + 3 alasan + arti + risiko). Best-effort: any LLM
// failure returns "" and the FE falls back to raw sections.
func (b *Builder) narasiAwam(ctx context.Context, ticker string, synth agents.Synthesis, sections []Section) string {
	if b.Deps.LLM == nil || !b.Deps.LLM.Available() {
		return ""
	}
	var sb strings.Builder
	for _, s := range sections {
		if strings.TrimSpace(s.Body) != "" {
			sb.WriteString("- " + s.Name + ": " + s.Body + "\n")
		}
	}
	text, err := b.Deps.LLM.Complete(ctx, b.Deps.SynthModel,
		"Kamu asisten pasar saham Indonesia yang ramah. Dari data report berikut, tulis Bahasa Indonesia santai untuk orang awam dengan format:\n"+
			"Baris 1: KESIMPULAN: <Dilirik/Tahan/Dilepas> — 1 kalimat.\n"+
			"Baris 2-4: 3 alasan (masing-masing 1 kalimat, sebut angkanya).\n"+
			"Baris 5: ARTINYA BUAT SAYA: 1 kalimat.\n"+
			"Baris 6: RISIKO: 1 kalimat.\n"+
			"Setiap angka WAJIB dari data — jangan mengarang. Maksimal 150 kata.",
		"Ticker: "+ticker+"\nRekomendasi sistem: "+synth.Recommendation+
			" (conviction "+fmt.Sprint(synth.Conviction)+"/5)\nTesis: "+synth.Thesis+
			"\nData:\n"+head(sb.String(), 2500), 400)
	if err != nil || strings.TrimSpace(text) == "" {
		return ""
	}
	return strings.TrimSpace(text)
}

func head(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func citesOf(byAgent map[string]model.AgentResult, names ...string) []model.Citation {
	var out []model.Citation
	for _, n := range names {
		out = append(out, byAgent[n].Citations...)
	}
	return out
}

func overviewLine(ticker string, byAgent map[string]model.AgentResult) string {
	return fmt.Sprintf("%s — sentiment %s; technical %s.",
		ticker, byAgent["sentiment"].Summary, byAgent["technical"].Summary)
}

func institutionalLine(byAgent map[string]model.AgentResult) string {
	return fmt.Sprintf("smart-money %s; broker-intel %s.",
		byAgent["smart-money"].Summary, byAgent["broker-intel"].Summary)
}

func riskLine(ticker string, fn RiskFn) string {
	if fn == nil {
		return ticker + ": concentration/beta from /portfolio/risk (run screener first)."
	}
	c, b := fn(ticker)
	return fmt.Sprintf("%s: %s; %s.", ticker, c, b)
}
