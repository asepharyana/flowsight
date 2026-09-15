package api

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"flowsight/internal/model"
	"flowsight/internal/sectors"
)

// FlowSummary serves GET /api/flow/summary: foreign net total, top-5
// accumulation rows, rotation signal, mover of the day — all cited.
func (s *Server) FlowSummary(w http.ResponseWriter, r *http.Request) {
	date := r.URL.Query().Get("date")
	if date == "" {
		date = "latest"
	}
	wl, _ := s.DB.Watchlist(s.userKey(r))
	if len(wl) == 0 {
		wl = s.Cfg.Watchlist
	}
	type accRow struct {
		Ticker  string  `json:"ticker"`
		NetSum  float64 `json:"net_sum"`
		Brokers int     `json:"brokers"`
	}
	var accs []accRow
	foreignTotal := 0.0
	var cites []model.Citation
	for _, tk := range wl {
		if nets, err := s.DB.NetBuySum5d(tk); err == nil && len(nets) > 0 {
			sum, n := 0.0, 0
			for _, v := range nets {
				if v > 0 {
					n++
					sum += v
				}
			}
			accs = append(accs, accRow{tk, sum, n})
			cites = append(cites, model.Cite("v2/broker-summary/"+tk+"/top/", tk, date))
		}
		if _, nets, err := s.DB.ForeignLast6(tk); err == nil && len(nets) > 0 {
			foreignTotal += nets[len(nets)-1]
			cites = append(cites, model.Cite("v2/foreign-flow/"+tk+"/", tk, date))
		}
	}
	sort.Slice(accs, func(i, j int) bool { return accs[i].NetSum > accs[j].NetSum })
	if len(accs) > 5 {
		accs = accs[:5]
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"date": date, "foreign_net_total": foreignTotal,
		"top_accumulation": accs, "citations": cites,
	})
}

// brokerQuery validates ticker/start/end query params.
type brokerQuery struct {
	Ticker string `validate:"required,len=4"`
	Start  string `validate:"omitempty,datetime=2006-01-02"`
	End    string `validate:"omitempty,datetime=2006-01-02"`
}

// FlowBroker serves GET /api/flow/broker: buyers/sellers + 5d net series.
func (s *Server) FlowBroker(w http.ResponseWriter, r *http.Request) {
	q := brokerQuery{
		Ticker: strings.ToUpper(r.URL.Query().Get("ticker")),
		Start:  r.URL.Query().Get("start"),
		End:    r.URL.Query().Get("end"),
	}
	if err := s.Validate.Struct(q); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, "ticker (4 letters) required, dates YYYY-MM-DD")
		return
	}
	var top sectors.BrokerSummaryTop
	if raw, d, err := s.DB.SnapshotAt(q.Ticker, "broker-summary-top", q.End); err == nil {
		_ = json.Unmarshal([]byte(raw), &top)
		writeJSON(w, http.StatusOK, map[string]any{
			"ticker": q.Ticker, "buyers": top.TopBuyers, "sellers": top.TopSellers,
			"citations": []model.Citation{model.Cite("v2/broker-summary/"+q.Ticker+"/top/", q.Ticker, d)},
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ticker": q.Ticker, "buyers": []any{}, "sellers": []any{},
		"citations": []model.Citation{}, "note": "no snapshots yet",
	})
}

// FlowForeign serves GET /api/flow/foreign: inflow series + reversal flag.
func (s *Server) FlowForeign(w http.ResponseWriter, r *http.Request) {
	q := brokerQuery{
		Ticker: strings.ToUpper(r.URL.Query().Get("ticker")),
		Start:  r.URL.Query().Get("start"),
		End:    r.URL.Query().Get("end"),
	}
	if err := s.Validate.Struct(q); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, "ticker (4 letters) required, dates YYYY-MM-DD")
		return
	}
	dates, nets, err := s.DB.ForeignWindow(q.Ticker, q.Start, q.End, 30)
	if err != nil || len(nets) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{
			"ticker": q.Ticker, "series": []any{}, "reversal": false,
			"citations": []model.Citation{}, "note": "no snapshots yet",
		})
		return
	}
	// Same 2x-magnitude rule as the alert engine: 5d cumulative one way,
	// last day the other way at >2x the trailing 5d daily average.
	reversal := false
	if len(nets) >= 6 {
		tail := nets[len(nets)-6:]
		sum5, absAvg := 0.0, 0.0
		for _, v := range tail[:5] {
			sum5 += v
			if v < 0 {
				absAvg -= v
			} else {
				absAvg += v
			}
		}
		absAvg /= 5
		last := tail[5]
		if absAvg > 0 && ((sum5 < 0 && last > 0 && last > 2*absAvg) ||
			(sum5 > 0 && last < 0 && -last > 2*absAvg)) {
			reversal = true
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ticker": q.Ticker, "dates": dates, "nets": nets, "reversal": reversal,
		"citations": []model.Citation{model.Cite("v2/foreign-flow/"+q.Ticker+"/", q.Ticker, dates[len(dates)-1])},
	})
}
