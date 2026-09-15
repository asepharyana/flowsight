package agents

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"flowsight/internal/model"
	"flowsight/internal/sectors"
)

// AnalyzeBrokerIntel (A2) classifies broker behavior and emits sector
// rotation on week-over-week sign flips with evidence rows.
func AnalyzeBrokerIntel(ctx context.Context, d Deps, ticker string) model.AgentResult {
	_ = ctx
	ticker = strings.ToUpper(ticker)
	res := model.AgentResult{Summary: "no broker snapshots available"}

	var registry []sectors.BrokerRegistryRow
	regDate, regOK := payload(d.DB, "IDX", "brokers-registry", &registry)
	var top struct {
		Date    string                 `json:"date"`
		Results []sectors.TopBrokerRow `json:"results"`
	}
	topDate, topOK := payload(d.DB, "IDX", "brokers-top", &top)
	if !regOK && !topOK {
		return res
	}
	if regOK {
		res.Citations = append(res.Citations, model.Cite("v2/brokers/", "IDX", regDate))
	}
	if topOK {
		res.Citations = append(res.Citations, model.Cite("v2/brokers/top/", "IDX", topDate))
	}

	byCode := map[string]sectors.BrokerRegistryRow{}
	for _, r := range registry {
		byCode[r.Code] = r
	}
	accum, distrib := 0, 0
	var lines []string
	for _, b := range top.Results {
		row := byCode[b.BrokerCode]
		origin := "domestic"
		cohort := "unknown"
		if row.IsForeign {
			origin = "foreign"
		}
		if row.Cohort != nil && *row.Cohort != "" {
			cohort = *row.Cohort
		}
		class := "neutral"
		switch {
		case b.Net > 0:
			class, accum = "accumulating", accum+1
		case b.Net < 0:
			class, distrib = "distributing", distrib+1
		}
		if len(lines) < 5 {
			lines = append(lines, fmt.Sprintf("%s (%s/%s) %s %s",
				b.BrokerCode, origin, cohort, class, fmtIDR(float64(b.Net))))
		}
	}
	total := accum + distrib
	score := 0.0
	if total > 0 {
		score = float64(accum-distrib) / float64(total) * 100
	}
	res.Score = clampScore(score, -100, 100)

	// Rotation: week-over-week sign flip on stored sector nets.
	var flow struct {
		Week     string             `json:"week"`
		Current  map[string]float64 `json:"current"`
		Previous map[string]float64 `json:"previous"`
	}
	flowDate, flowOK := payload(d.DB, "IDX", "sector-flow", &flow)
	rotFrom, rotTo, rotDelta := "", "", 0.0
	if flowOK {
		res.Citations = append(res.Citations, model.Cite("v2/subsector/report/", "IDX", flowDate))
		type flip struct {
			sector string
			delta  float64
		}
		var flips []flip
		for s, cur := range flow.Current {
			prev := flow.Previous[s]
			if prev < 0 && cur > 0 {
				flips = append(flips, flip{s, cur - prev})
			}
		}
		var outflows []flip
		for s, cur := range flow.Current {
			prev := flow.Previous[s]
			if prev > 0 && cur < 0 {
				outflows = append(outflows, flip{s, prev - cur})
			}
		}
		sort.Slice(flips, func(i, j int) bool { return flips[i].delta > flips[j].delta })
		sort.Slice(outflows, func(i, j int) bool { return outflows[i].delta > outflows[j].delta })
		if len(flips) > 0 && len(outflows) > 0 {
			rotFrom, rotTo, rotDelta = outflows[0].sector, flips[0].sector, flips[0].delta
			res.Flags = append(res.Flags, "sector-rotation")
		}
	}

	res.Values = []model.Value{{
		Label:     "broker behavior",
		Display:   fmt.Sprintf("%d accumulating vs %d distributing", accum, distrib),
		Citations: res.Citations,
	}, {
		Label:     "top brokers",
		Display:   strings.Join(lines, "; "),
		Citations: res.Citations,
	}}
	if rotFrom != "" {
		res.Values = append(res.Values, model.Value{
			Label:     "sector rotation",
			Display:   fmt.Sprintf("%s -> %s (%s swing)", rotFrom, rotTo, fmtIDR(rotDelta)),
			Citations: res.Citations,
		})
		res.Summary = fmt.Sprintf("rotation %s -> %s; score %+.0f", rotFrom, rotTo, score)
	} else {
		res.Summary = fmt.Sprintf("no rotation flip; score %+.0f (%d vs %d)", score, accum, distrib)
	}
	res.Extra = map[string]any{
		"accumulating": accum, "distributing": distrib,
		"rotation_from": rotFrom, "rotation_to": rotTo, "rotation_delta": rotDelta,
	}
	return res
}
