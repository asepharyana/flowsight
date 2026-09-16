package api

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"flowsight/internal/model"
)

// ScreenRequest is POST /api/screen body.
type ScreenRequest struct {
	Where         string `json:"where"`
	Q             string `json:"q"`
	Institutional *struct {
		BrokerScoreMin float64 `json:"broker_score_min"`
		ForeignTrend   string  `json:"foreign_trend"`
		InsiderBuying  bool    `json:"insider_buying"`
		VolumeAnomaly  bool    `json:"volume_anomaly"`
	} `json:"institutional"`
	Limit int `json:"limit"`
}

// ScreenRow is one ranked result with per-row signal breakdown.
type ScreenRow struct {
	Symbol    string           `json:"symbol"`
	Name      string           `json:"name"`
	Composite float64          `json:"composite"`
	Breakdown map[string]any   `json:"breakdown"`
	Citations []model.Citation `json:"citations"`
}

// Screen serves POST /api/screen: companies/ base filter enriched with
// broker score + foreign trend + insider flag, ranked composite.
func (s *Server) Screen(w http.ResponseWriter, r *http.Request) {
	var req ScreenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	limit := req.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	// Base universe: live screener when keyed, else stored watchlist.
	var universe []string
	names := map[string]string{}
	// Free-text q → local universe search (ticker + company_name, offline).
	// The Sectors companies/ endpoint is not on this subscription, so remote
	// screening is skipped entirely to avoid a silent full-universe fallback.
	if req.Q != "" {
		if hits, err := s.DB.SearchTickers(req.Q, limit*2); err == nil {
			for _, h := range hits {
				universe = append(universe, h.Symbol)
				if h.CompanyName != "" {
					names[h.Symbol] = h.CompanyName
				}
			}
		}
	} else if s.Cfg.HasSectorsKey() && req.Where != "" {
		if rows, err := s.Sectors.Screen(r.Context(), req.Where, req.Q, limit*2, 0); err == nil {
			for _, row := range rows {
				tk := strings.ToUpper(strings.TrimSuffix(row.Symbol, ".JK"))
				universe = append(universe, tk)
				if row.CompanyName != "" {
					names[tk] = row.CompanyName
				}
			}
		}
	}
	if len(universe) == 0 {
		universe, _ = s.DB.Watchlist(s.userKey(r))
		if len(universe) == 0 {
			if all, err := s.DB.AllTickers(); err == nil && len(all) > 0 {
				universe = all
			} else {
				universe = s.Cfg.Watchlist
			}
		}
		// Fall back to stored company names when offline (universe table).
		storedNames, _ := s.DB.CompanyNames(universe)
		for tk, nm := range storedNames {
			names[tk] = nm
		}
	}
	var rows []ScreenRow
	for _, tk := range universe {
		row := s.scoreTicker(tk)
		if nm := names[tk]; nm != "" {
			row.Name = nm
		}
		if req.Institutional != nil {
			inst := req.Institutional
			if b, _ := row.Breakdown["broker_score"].(float64); b < inst.BrokerScoreMin {
				continue
			}
			if inst.ForeignTrend != "" {
				if t, _ := row.Breakdown["foreign_trend"].(string); t != inst.ForeignTrend {
					continue
				}
			}
			if inst.InsiderBuying {
				if b, _ := row.Breakdown["insider_buying"].(bool); !b {
					continue
				}
			}
			if inst.VolumeAnomaly {
				if b, _ := row.Breakdown["volume_anomaly"].(bool); !b {
					continue
				}
			}
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Composite > rows[j].Composite })
	if len(rows) > limit {
		rows = rows[:limit]
	}
	writeJSON(w, http.StatusOK, map[string]any{"rows": rows, "count": len(rows)})
}

// scoreTicker computes the composite (broker 40 + foreign 25 + insider 15 + volume 20).
func (s *Server) scoreTicker(tk string) ScreenRow {
	tk = strings.ToUpper(tk)
	row := ScreenRow{Symbol: tk, Name: tk, Breakdown: map[string]any{}}
	// Broker score from 5d net imbalance.
	brokerScore := 0.0
	if nets, err := s.DB.NetBuySum5d(tk); err == nil && len(nets) > 0 {
		pos, neg := 0.0, 0.0
		for _, v := range nets {
			if v > 0 {
				pos += v
			} else {
				neg -= v
			}
		}
		if tot := pos + neg; tot > 0 {
			brokerScore = (pos - neg) / tot * 100
		}
		row.Citations = append(row.Citations, model.Cite("v2/broker-summary/"+tk+"/top/", tk, "stored"))
	}
	// Foreign trend from last-6 series.
	foreignScore, trend := 0.0, "flat"
	if dates, nets, err := s.DB.ForeignLast6(tk); err == nil && len(nets) > 0 {
		last := nets[len(nets)-1]
		if last > 0 {
			foreignScore, trend = 50, "inflow"
		} else if last < 0 {
			foreignScore, trend = -50, "outflow"
		}
		row.Citations = append(row.Citations, model.Cite("v2/foreign-flow/"+tk+"/", tk, dates[len(dates)-1]))
	}
	// Insider flag from filings average.
	insider := s.DB.FilingAvg30(tk) > 0
	// Volume anomaly from stored daily bars.
	volAnom, volMult := false, 0.0
	if vols, _, err := s.DB.DailyVolumes(tk, 21); err == nil && len(vols) >= 2 {
		n := len(vols)
		if a := avgF(vols[:n-1]); a > 0 {
			volMult = vols[n-1] / a
			volAnom = volMult > 2
		}
		row.Citations = append(row.Citations, model.Cite("v2/daily/"+tk+"/", tk, "stored"))
	}
	volScore := 0.0
	if volAnom {
		volScore = 50
	}
	row.Composite = brokerScore*0.4 + foreignScore*0.25 + volScore*0.2
	if insider {
		row.Composite += 7.5
	}
	row.Breakdown = map[string]any{
		"broker": brokerScore, "broker_score": brokerScore,
		"foreign": foreignScore, "foreign_trend": trend,
		"insider": insider, "insider_buying": insider,
		"volume_mult": volMult, "volume_anomaly": volAnom,
	}
	return row
}

func avgF(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	sum := 0.0
	for _, x := range xs {
		sum += x
	}
	return sum / float64(len(xs))
}
