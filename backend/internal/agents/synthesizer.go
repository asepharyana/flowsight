package agents

import (
	"context"
	"fmt"
	"strings"

	"flowsight/internal/model"
)

// RiskProfile shifts signal weights: conservative is fundamental-heavy,
// aggressive leans into technical + broker flows.
type RiskProfile string

const (
	Conservative RiskProfile = "conservative"
	Moderate     RiskProfile = "moderate"
	Aggressive   RiskProfile = "aggressive"
)

// Synthesis is the Master Synthesizer (A7) output.
type Synthesis struct {
	Recommendation string           `json:"recommendation"` // BUY | HOLD | AVOID
	Conviction     int              `json:"conviction"`     // 1..5
	Thesis         string           `json:"thesis"`
	PositionPct    float64          `json:"position_pct"`
	Conflict       bool             `json:"conflict"`
	ConflictNote   string           `json:"conflict_note,omitempty"`
	Scores         []string         `json:"scores"`
	Citations      []model.Citation `json:"citations"`
}

// Synthesize (A7) weights A1..A6 by risk profile x accuracy-ledger weights,
// adds agreement bonus / conflict flag, and sizes via capped Kelly (max 10%).
func Synthesize(ctx context.Context, d Deps, ticker string, profile RiskProfile, results []model.AgentResult) Synthesis {
	_ = ctx
	ticker = strings.ToUpper(ticker)
	byAgent := map[string]model.AgentResult{}
	for _, r := range results {
		byAgent[r.Agent] = r
	}

	base := map[string]float64{
		"smart-money": 0.22, "broker-intel": 0.13, "sentiment": 0.12,
		"fundamental": 0.25, "technical": 0.15, "catalyst": 0.13,
	}
	switch profile {
	case Conservative:
		base = map[string]float64{
			"smart-money": 0.15, "broker-intel": 0.10, "sentiment": 0.10,
			"fundamental": 0.40, "technical": 0.10, "catalyst": 0.15,
		}
	case Aggressive:
		base = map[string]float64{
			"smart-money": 0.27, "broker-intel": 0.15, "sentiment": 0.10,
			"fundamental": 0.13, "technical": 0.25, "catalyst": 0.10,
		}
	}

	// Ledger weights: 0.5 until an agent has >=10 resolved calls.
	weights := map[string]float64{}
	norm := func(score float64) float64 { return (score + 100) / 200 } // -100..100 -> 0..1
	get := func(name string, raw, lo, hi float64) float64 {
		v := raw
		if hi == 1 && lo == -1 { // sentiment -1..1
			v = raw * 100
		} else if hi == 100 && lo == 0 { // fundamental/catalyst 0..100
			v = raw*2 - 100
		}
		return clampScore(v, -100, 100)
	}
	_ = norm

	total, wsum := 0.0, 0.0
	var lines []string
	var cites []model.Citation
	for _, a := range []string{"smart-money", "broker-intel", "sentiment", "fundamental", "technical", "catalyst"} {
		r := byAgent[a]
		lw := d.DB.AccuracyWeight(a)
		w := base[a] * (0.5 + lw) // ledger blends in without zeroing anyone
		weights[a] = w
		var v float64
		switch a {
		case "sentiment":
			v = get(a, r.Score, -1, 1)
		case "fundamental", "catalyst":
			v = get(a, r.Score, 0, 100)
		default:
			v = get(a, r.Score, -100, 100)
		}
		total += w * v
		wsum += w
		lines = append(lines, fmt.Sprintf("%s %+.0f", a, v))
		cites = append(cites, r.Citations...)
	}
	score := 0.0
	if wsum > 0 {
		score = total / wsum
	}

	// Agreement bonus (>=3 aligned) / conflict flag (fundamental vs flows).
	align := 0
	for _, a := range []string{"smart-money", "fundamental", "technical", "sentiment"} {
		r := byAgent[a]
		v := r.Score
		if a == "sentiment" {
			v *= 100
		} else if a == "fundamental" {
			v = v*2 - 100
		}
		if (score > 0 && v > 0) || (score < 0 && v < 0) {
			align++
		}
	}
	if align >= 3 {
		if score > 0 {
			score += 5
		} else {
			score -= 5
		}
	}
	fund := byAgent["fundamental"].Score*2 - 100
	flow := byAgent["smart-money"].Score
	conflict := (fund > 20 && flow < -20) || (fund < -20 && flow > 20)
	conflictNote := ""
	if conflict {
		conflictNote = fmt.Sprintf("fundamental %+.0f opposes smart-money %+.0f", fund, flow)
	}

	rec := "HOLD"
	switch {
	case score >= 25 && !conflict:
		rec = "BUY"
	case score <= -25:
		rec = "AVOID"
	case conflict && score >= 25:
		rec = "HOLD" // good fundamental + broker selling => HOLD-or-lower, cited
	}
	conviction := 3
	switch {
	case score >= 50 || score <= -50:
		conviction = 5
	case score >= 35 || score <= -35:
		conviction = 4
	case score >= -15 && score <= 15:
		conviction = 2
	}
	if conflict && conviction > 3 {
		conviction = 3
	}

	// Capped Kelly: edge from score magnitude, max 10% single name.
	edge := (score / 100) * 0.5
	size := edge * 0.25 * 100
	if size < 0 {
		size = 0
	}
	if size > 10 {
		size = 10
	}
	if rec != "BUY" {
		size = 0
	}

	thesis := fmt.Sprintf("%s %s (conviction %d/5): weighted score %+.0f. %s.",
		ticker, rec, conviction, score, strings.Join(citedLines(byAgent), ", "))
	if conflict {
		thesis += " Conflict: " + conflictNote + citeStr(byAgent["smart-money"]) + citeStr(byAgent["fundamental"]) + "."
	}

	// Record predictions for the +30d accuracy ledger.
	_ = d.DB.RecordPrediction("synthesizer", ticker, rec)
	for _, a := range []string{"smart-money", "fundamental", "technical"} {
		_ = d.DB.RecordPrediction(a, ticker, rec)
	}

	return Synthesis{
		Recommendation: rec, Conviction: conviction, Thesis: thesis,
		PositionPct: size, Conflict: conflict, ConflictNote: conflictNote,
		Scores: lines, Citations: cites,
	}
}

// citedLines appends each agent's first citation marker to its score line so
// every thesis claim is individually traceable.
func citedLines(byAgent map[string]model.AgentResult) []string {
	var out []string
	for _, a := range []string{"smart-money", "broker-intel", "sentiment", "fundamental", "technical", "catalyst"} {
		r := byAgent[a]
		line := strings.TrimSpace(strings.Split(r.Summary, ";")[0])
		if line == "" {
			line = a
		}
		out = append(out, line+citeStr(r))
	}
	return out
}

// citeStr renders "[endpoint @ date]" for an agent's first citation.
func citeStr(r model.AgentResult) string {
	if len(r.Citations) == 0 {
		return " [no snapshot]"
	}
	c := r.Citations[0]
	return " [" + c.Endpoint + " @ " + c.SnapshotAt + "]"
}
