// Package scheduler wires the 30-min ingestion cycle (09:00-16:00 WIB market
// hours) plus per-routine cron dispatch into one in-process robfig/cron.
// Order per cycle: reference cache -> universe sweep -> market context ->
// per-watchlist depth -> incremental events -> quarterly freshness ->
// rule evaluation -> routine dispatch. Credit spend aborts past the cap.
package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"time"

	"github.com/robfig/cron/v3"

	"flowsight/internal/alerts"
	"flowsight/internal/config"
	"flowsight/internal/routines"
	"flowsight/internal/sectors"
	"flowsight/internal/store"
)

// Scheduler owns ingestion + routine dispatch.
type Scheduler struct {
	Cfg      config.Config
	DB       *store.DB
	Cache    *store.Cache
	Sectors  *sectors.Client
	Notifier *alerts.Notifier
	Engine   *routines.Engine
	// Publish, when set, feeds routine activity into the SSE hub.
	Publish func(channel, data string)
	cron    *cron.Cron
	lastRun time.Time
	ok      bool
}

// New wires dependencies; the sectors OnSpend hook persists credit_ledger rows.
func New(cfg config.Config, db *store.DB, cache *store.Cache, s *sectors.Client) *Scheduler {
	sch := &Scheduler{Cfg: cfg, DB: db, Cache: cache, Sectors: s,
		Notifier: alerts.NewNotifier(cfg.TelegramBotToken, cfg.TelegramChatID, cfg.DiscordWebhookURL),
	}
	sch.Notifier.Targets = func(owner string) []alerts.Target {
		dests, err := db.ListEnabledDestinations(owner)
		if err != nil {
			return nil
		}
		var out []alerts.Target
		for _, d := range dests {
			switch d.Kind {
			case store.DestTelegram:
				if d.BotToken != "" && d.ChatID != "" {
					out = append(out, alerts.Target{TelegramToken: d.BotToken, TelegramChatID: d.ChatID})
				}
			case store.DestDiscord:
				if d.WebhookURL != "" {
					out = append(out, alerts.Target{DiscordURL: d.WebhookURL})
				}
			}
		}
		return out
	}
	sch.Engine = &routines.Engine{DB: db, Notifier: sch.Notifier, UserKey: cfg.DemoUserKey}
	sch.Engine.Publish = func(channel, data string) {
		if sch.Publish != nil {
			sch.Publish(channel, data)
		}
	}
	s.OnSpend(func(endpoint string, calls, credits int) {
		db.AddSpend(time.Now().Format("2006-01-02"), endpoint, calls, credits)
	})
	return sch
}

// Start registers ingestion + routine crons (WIB = UTC+7) and begins ticking.
func (s *Scheduler) Start() {
	c := cron.New(cron.WithLocation(wib()))
	// 30-min ingestion, weekdays 09:00-16:00 WIB.
	_, _ = c.AddFunc("*/30 9-16 * * 1-5", func() { s.RunCycle(context.Background()) })
	// Routines: briefing 07:30 daily, countdowns 08:00, weekend Sat 09:00,
	// watchtower R2..R4 every 30 min in market hours.
	_, _ = c.AddFunc("30 7 * * *", func() { s.runType(context.Background(), routines.RBriefing) })
	_, _ = c.AddFunc("*/30 9-16 * * 1-5", func() {
		s.runType(context.Background(), routines.RRadar)
		s.runType(context.Background(), routines.RReversal)
		s.runType(context.Background(), routines.RInsider)
	})
	_, _ = c.AddFunc("0 8 * * *", func() { s.runType(context.Background(), routines.REarnings) })
	_, _ = c.AddFunc("5 8 * * *", func() { s.runType(context.Background(), routines.RDividend) })
	// Accuracy resolution on its own daily cadence (was piggybacked on RunCycle).
	_, _ = c.AddFunc("15 6 * * *", func() {
		if n, err := s.DB.ResolveDue(); err != nil {
			log.Printf("scheduler: resolve: %v", err)
		} else if n > 0 {
			log.Printf("scheduler: resolved %d predictions", n)
		}
	})
	// Snapshot retention: weekly prune of rows older than 180d (keeps newest
	// per ticker/source for seeds and last-good fallback).
	_, _ = c.AddFunc("30 6 * * 0", func() {
		cutoff := time.Now().AddDate(0, 0, -180).Format("2006-01-02")
		if n, err := s.DB.PruneSnapshots(cutoff); err != nil {
			log.Printf("scheduler: prune: %v", err)
		} else if n > 0 {
			log.Printf("scheduler: pruned %d snapshots older than %s", n, cutoff)
		}
	})
	_, _ = c.AddFunc("0 9 * * 6", func() { s.runType(context.Background(), routines.RWeekend) })
	s.cron = c
	c.Start()
	s.ok = true
}

// Stop halts all crons.
func (s *Scheduler) Stop() {
	if s.cron != nil {
		s.cron.Stop()
	}
	s.ok = false
}

// Status reports scheduler health for /api/health.
func (s *Scheduler) Status() (lastCycle string, ok bool) {
	if s.lastRun.IsZero() {
		return "", s.ok
	}
	return s.lastRun.UTC().Format(time.RFC3339), s.ok
}

func wib() *time.Location {
	return time.FixedZone("WIB", 7*3600)
}

// RunCycle executes one full ingestion cycle. Offline (no key) it returns
// early after marking scheduler health — seed data keeps the demo alive.
func (s *Scheduler) RunCycle(ctx context.Context) error {
	if !s.Cfg.HasSectorsKey() {
		s.lastRun = time.Now()
		return nil
	}
	today := time.Now().Format("2006-01-02")
	before := s.DB.CreditsToday(today)
	guard := func() error {
		if spent := s.DB.CreditsToday(today) - before; spent > s.Cfg.CreditCapPerCycle {
			return fmt.Errorf("scheduler: credit cap %d exceeded (spent %d), aborting cycle",
				s.Cfg.CreditCapPerCycle, spent)
		}
		return nil
	}
	if err := s.reference(ctx); err != nil {
		log.Printf("scheduler: reference: %v", err)
	}
	if err := guard(); err != nil {
		return err
	}
	if err := s.universe(ctx); err != nil {
		log.Printf("scheduler: universe: %v", err)
	}
	if err := guard(); err != nil {
		return err
	}
	if err := s.market(ctx); err != nil {
		log.Printf("scheduler: market: %v", err)
	}
	if err := guard(); err != nil {
		return err
	}
	for _, t := range s.watchlist() {
		if err := s.tickerDepth(ctx, t); err != nil {
			log.Printf("scheduler: depth %s: %v", t, err)
		}
		if err := guard(); err != nil {
			return err
		}
	}
	if err := s.events(ctx); err != nil {
		log.Printf("scheduler: events: %v", err)
	}
	if err := s.freshness(ctx); err != nil {
		log.Printf("scheduler: freshness: %v", err)
	}
	s.evaluate(ctx)
	s.lastRun = time.Now()
	return nil
}

func (s *Scheduler) watchlist() []string {
	return s.watchlistFor(s.Cfg.DemoUserKey)
}

// watchlistFor resolves one owner's watchlist (demo seed fallback kept).
func (s *Scheduler) watchlistFor(owner string) []string {
	wl, err := s.DB.Watchlist(owner)
	if err == nil && len(wl) > 0 {
		return wl
	}
	if owner == s.Cfg.DemoUserKey && len(s.Cfg.Watchlist) > 0 {
		return s.Cfg.Watchlist
	}
	return []string{"BBCA"}
}

// reference refreshes registry/taxonomy caches (24h TTL, fallback last good).
func (s *Scheduler) reference(ctx context.Context) error {
	if raw := s.Cache.Get(ctx, "registry"); raw != "" {
		return nil // fresh enough; TTL governs refresh
	}
	reg, err := s.Sectors.BrokersRegistry(ctx, "", "")
	if err != nil {
		if raw, _, lerr := s.DB.LatestSnapshot("IDX", "brokers-registry"); lerr == nil && raw != "" {
			var cached []sectors.BrokerRegistryRow
			if jerr := json.Unmarshal([]byte(raw), &cached); jerr == nil && len(cached) > 0 {
				s.Cache.SetJSON(ctx, "registry", cached, 24*time.Hour)
				return nil
			}
		}
		return err
	}
	s.Cache.SetJSON(ctx, "registry", reg, 24*time.Hour)
	if raw, err := json.Marshal(reg); err == nil {
		_ = s.DB.SaveSnapshot("IDX", time.Now().Format("2006-01-02"), "brokers-registry", string(raw))
	}
	for _, kind := range []string{"subsectors", "industries", "subindustries", "tags"} {
		if rows, err := s.Sectors.Taxonomy(ctx, kind); err == nil {
			s.Cache.SetJSON(ctx, "tax:"+kind, rows, 24*time.Hour)
		}
	}
	return nil
}

// universe sweeps close/ pages for the latest trading day.
func (s *Scheduler) universe(ctx context.Context) error {
	offset := 0
	for page := 0; page < 12; page++ {
		rows, total, err := s.Sectors.ClosePage(ctx, "", 30, offset)
		if err != nil {
			return err
		}
		if raw, err := json.Marshal(rows); err == nil && len(rows) > 0 {
			_ = s.DB.SaveSnapshot("IDX", rows[0].Date, "close", string(raw))
		}
		offset += len(rows)
		if offset >= total || len(rows) == 0 {
			break
		}
	}
	return nil
}

// market pulls top-changes (1 class x 2 periods), most-traded, idx-total,
// brokers/top — the cheap context block of the cycle.
func (s *Scheduler) market(ctx context.Context) error {
	if g, l, err := s.Sectors.TopChanges(ctx,
		[]string{"top_gainers"}, []string{"1d", "7d"}, "", 5); err == nil {
		if raw, err := json.Marshal(map[string]any{"top_gainers": g, "top_losers": l}); err == nil {
			_ = s.DB.SaveSnapshot("IDX", time.Now().Format("2006-01-02"), "top-changes", string(raw))
		}
	} else {
		return err
	}
	if mt, err := s.Sectors.MostTraded(ctx, "", "", "", 5); err == nil {
		if raw, err := json.Marshal(mt); err == nil {
			_ = s.DB.SaveSnapshot("IDX", time.Now().Format("2006-01-02"), "most-traded", string(raw))
		}
	}
	if it, err := s.Sectors.IdxTotal(ctx, "", ""); err == nil {
		if raw, err := json.Marshal(it); err == nil {
			_ = s.DB.SaveSnapshot("IDX", time.Now().Format("2006-01-02"), "idx-total", string(raw))
		}
	}
	if bt, err := s.Sectors.BrokersTop(ctx, "", "net", 20, "", ""); err == nil {
		if raw, err := json.Marshal(map[string]any{"results": bt}); err == nil {
			_ = s.DB.SaveSnapshot("IDX", time.Now().Format("2006-01-02"), "brokers-top", string(raw))
		}
	}
	return nil
}

// tickerDepth pulls broker-summary/top + foreign-flow + daily per ticker.
func (s *Scheduler) tickerDepth(ctx context.Context, ticker string) error {
	end := time.Now().Format("2006-01-02")
	start5 := time.Now().AddDate(0, 0, -6).Format("2006-01-02")
	if top, err := s.Sectors.BrokerSummaryTop(ctx, ticker, start5, end, 10, "", ""); err == nil {
		if raw, err := json.Marshal(top); err == nil {
			_ = s.DB.SaveSnapshot(ticker, end, "broker-summary-top", string(raw))
			for _, b := range top.TopBuyers {
				_ = s.DB.UpsertBrokerActivity(b.BrokerCode, ticker, end,
					float64(b.BuyIDR), float64(b.SellIDR), float64(b.NetIDR), 0, 0, 0)
			}
			for _, x := range top.TopSellers {
				_ = s.DB.UpsertBrokerActivity(x.BrokerCode, ticker, end,
					float64(x.BuyIDR), float64(x.SellIDR), float64(x.NetIDR), 0, 0, 0)
			}
		}
	} else {
		return err
	}
	if ff, err := s.Sectors.ForeignFlow(ctx, ticker,
		time.Now().AddDate(0, 0, -7).Format("2006-01-02"), end); err == nil {
		if raw, err := json.Marshal(ff); err == nil {
			_ = s.DB.SaveSnapshot(ticker, end, "foreign-flow", string(raw))
			for _, d := range ff.Data {
				_ = s.DB.UpsertForeignFlow(ticker, d.Date, float64(d.NetForeignInflow))
			}
		}
	}
	if bars, err := s.Sectors.Daily(ctx, ticker,
		time.Now().AddDate(0, 0, -30).Format("2006-01-02"), end); err == nil {
		if raw, err := json.Marshal(bars); err == nil && len(bars) > 0 {
			_ = s.DB.SaveSnapshot(ticker, end, "daily", string(raw))
		}
	}
	return nil
}

// events polls news/filings/suspensions incrementally via meta cursors.
func (s *Scheduler) events(ctx context.Context) error {
	today := time.Now().Format("2006-01-02")
	newsSince := s.DB.GetMeta("news_since")
	if news, err := s.Sectors.News(ctx, "", newsSince, today, "", 20); err == nil {
		if raw, err := json.Marshal(map[string]any{"results": news}); err == nil {
			_ = s.DB.SaveSnapshot("IDX", today, "news", string(raw))
		}
		s.DB.SetMeta("news_since", today)
	}
	filSince := s.DB.GetMeta("filings_since")
	if fils, err := s.Sectors.Filings(ctx, "", "", "", filSince, today); err == nil {
		if raw, err := json.Marshal(map[string]any{"results": fils}); err == nil {
			_ = s.DB.SaveSnapshot("IDX", today, "filings", string(raw))
		}
		s.DB.SetMeta("filings_since", today)
	}
	if susp, err := s.Sectors.Suspensions(ctx, "", newsSince, today); err == nil {
		if raw, err := json.Marshal(map[string]any{"results": susp}); err == nil {
			_ = s.DB.SaveSnapshot("IDX", today, "suspensions", string(raw))
		}
	}
	return nil
}

// freshness polls quarterly-dates with since= (never a full sweep).
func (s *Scheduler) freshness(ctx context.Context) error {
	since := s.DB.GetMeta("quarterly_since")
	rows, err := s.Sectors.QuarterlyDatesSince(ctx, since, 30, 0)
	if err != nil {
		return err
	}
	if len(rows) > 0 {
		if raw, err := json.Marshal(rows); err == nil {
			_ = s.DB.SaveSnapshot("IDX", time.Now().Format("2006-01-02"), "quarterly-dates", string(raw))
		}
		s.DB.SetMeta("quarterly_since", time.Now().Format("2006-01-02"))
	}
	return nil
}

// evaluate runs rule evaluation for every user that owns alerts, each
// against that owner's own watchlist and destinations.
func (s *Scheduler) evaluate(ctx context.Context) {
	for _, owner := range s.alertOwners() {
		alertsList, err := s.DB.ListAlerts(owner)
		if err != nil {
			continue
		}
		if len(alertsList) == 0 {
			continue
		}
		wl := s.watchlistFor(owner)
		for _, a := range alertsList {
			alerts.EvaluateFor(ctx, s.DB, s.Notifier, a.ID, owner, a.Rule, wl)
		}
	}
}

// runType executes all enabled routines of one type, per owning user: each
// run reads that user's watchlist and pushes to that user's destinations.
func (s *Scheduler) runType(ctx context.Context, typ string) {
	for _, owner := range s.routineOwners() {
		rows, err := s.DB.ListRoutines(owner)
		if err != nil {
			continue
		}
		for _, r := range rows {
			if r.Type == typ && r.Enabled {
				s.Engine.RunFor(ctx, owner, r)
			}
		}
	}
}

// alertOwners lists users owning at least one alert.
func (s *Scheduler) alertOwners() []string {
	return s.DB.DistinctUsers("alerts")
}

// routineOwners lists users owning at least one enabled routine.
func (s *Scheduler) routineOwners() []string {
	owners := s.DB.DistinctUsers("routines")
	if len(owners) == 0 {
		return []string{s.Cfg.DemoUserKey}
	}
	return owners
}

// TriggerCycle runs one cycle synchronously (used by tests and the
// ?force=1 health probe); q carries no Sectors params.
var _ = url.Values{}
