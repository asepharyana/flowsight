// Package routines turns agent output into scheduled deliveries (R1..R7).
// Each run records a routine_runs row; each delivery carries citations.
// Routines read snapshots, never live-fetch inside delivery.
package routines

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"flowsight/internal/alerts"
	"flowsight/internal/model"
	"flowsight/internal/store"
)

// Routine types R1..R7.
const (
	RBriefing = "morning-briefing"
	RRadar    = "accumulation-radar"
	RReversal = "foreign-reversal"
	RInsider  = "insider-tape"
	REarnings = "earnings-countdown"
	RDividend = "dividend-calendar"
	RWeekend  = "weekend-review"
)

// KnownType reports whether typ is one of R1..R7.
func KnownType(typ string) bool {
	switch typ {
	case RBriefing, RRadar, RReversal, RInsider, REarnings, RDividend, RWeekend:
		return true
	}
	return false
}

// DefaultSchedule returns the spec cron for a routine type (WIB).
func DefaultSchedule(typ string) string {
	switch typ {
	case RBriefing:
		return "30 7 * * *" // 07:30 daily
	case RRadar, RReversal, RInsider:
		return "*/30 9-16 * * 1-5" // 30-min market hours
	case REarnings, RDividend:
		return "0 8 * * *" // 08:00 daily
	case RWeekend:
		return "0 9 * * 6" // Sat 09:00
	default:
		return "0 8 * * *"
	}
}

// Engine runs routines on demand (called by the scheduler) and by hand.
type Engine struct {
	DB       *store.DB
	Notifier *alerts.Notifier
	UserKey  string
	// Publish, when set, receives ("activity", text) per completed run so
	// the dashboard SSE feed shows routine activity live.
	Publish func(channel, data string)
}

// Run executes one routine row and records the run.
// Prefer RunFor: it scopes the run to the row owner (watchlist + delivery).
func (e *Engine) Run(ctx context.Context, r store.Routine) (string, error) {
	return e.RunFor(ctx, r.UserKey, r)
}

// RunFor executes one routine row owned by owner: tickers come from the
// owner's watchlist and delivery fans out to the owner's destinations.
func (e *Engine) RunFor(ctx context.Context, owner string, r store.Routine) (string, error) {
	// Scope this run to the owner without mutating shared engine state.
	scoped := *e
	if owner != "" {
		scoped.UserKey = owner
	}
	se := &scoped
	var (
		payload string
		cites   []model.Citation
		err     error
	)
	switch r.Type {
	case RBriefing:
		payload, cites, err = se.Briefing(ctx)
	case RRadar:
		payload, cites, err = se.Radar(ctx)
	case RReversal:
		payload, cites, err = se.ReversalWatch(ctx)
	case RInsider:
		payload, cites, err = se.InsiderTape(ctx)
	case REarnings:
		payload, cites, err = se.EarningsCountdown(ctx)
	case RDividend:
		payload, cites, err = se.DividendCalendar(ctx)
	case RWeekend:
		payload, cites, err = se.WeekendReview(ctx)
	default:
		err = fmt.Errorf("routines: unknown type %q", r.Type)
	}
	status := "ok"
	if err != nil {
		status = "error"
		payload = err.Error()
	}
	cc, _ := json.Marshal(cites)
	if _, rerr := se.DB.RecordRun(r.ID, status, summarize(payload, cc), 0, scoped.UserKey); rerr != nil && err == nil {
		err = rerr
	}
	if err == nil && status == "ok" {
		_ = se.deliver(ctx, r, payload, cites)
	}
	if e.Publish != nil {
		e.Publish("activity", r.Type+" "+status+": "+head(payload, 160))
	}
	return payload, err
}

func head(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func summarize(payload string, cites []byte) string {
	return `{"payload":` + strconv(payload) + `,"citations":` + string(cites) + `}`
}

func strconv(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// deliver pushes the payload to the routine's channels via the notifier,
// WITH citations (alert cards render context + citations + report link).
// Delivery is scoped to the routine owner: the owner's enabled destinations
// (managed at /alerts → Push destinations, i.e. POST /api/destinations)
// plus the server env fallback. With no targets anywhere it is record-only.
func (e *Engine) deliver(ctx context.Context, r store.Routine, payload string, cites []model.Citation) error {
	if len(r.Channels) == 0 || e.Notifier.DryRunFor(e.UserKey) {
		return nil
	}
	cc, _ := json.Marshal(cites)
	f := alerts.Finding{Rule: r.Type, Ticker: "IDX", Message: payload, Citations: cites}
	return e.Notifier.SendTo(ctx, e.UserKey, f, string(cc))
}

// BriefingFor runs the morning briefing scoped to owner (watchlist +
// delivery) without needing a routines row.
func (e *Engine) BriefingFor(ctx context.Context, owner string) (string, []model.Citation, error) {
	scoped := *e
	if owner != "" {
		scoped.UserKey = owner
	}
	return scoped.Briefing(ctx)
}

// tickers returns the user's watchlist (never empty: falls back to demo set).
func (e *Engine) tickers() []string {
	wl, err := e.DB.Watchlist(e.UserKey)
	if err == nil && len(wl) > 0 {
		return wl
	}
	return []string{"BBCA"}
}

// todayStr is overridable in tests via Now.
var nowFn = time.Now

func today() string { return nowFn().Format("2006-01-02") }
