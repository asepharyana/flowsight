package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// Snapshot helpers.

// SaveSnapshot stores one raw payload row.
func (db *DB) SaveSnapshot(ticker, date, source, payload string) error {
	_, err := db.Exec(`INSERT INTO snapshots(ticker,date,source,payload_json,fetched_at)
		VALUES(?,?,?,?,?)`, ticker, date, source, payload, time.Now().UTC().Format(time.RFC3339))
	return err
}

// LatestSnapshot returns the newest payload for (ticker, source).
func (db *DB) LatestSnapshot(ticker, source string) (payload, date string, err error) {
	err = db.QueryRow(`SELECT payload_json, date FROM snapshots
		WHERE ticker=? AND source=? ORDER BY date DESC, id DESC LIMIT 1`, ticker, source).Scan(&payload, &date)
	return payload, date, err
}

// SnapshotAt returns the newest payload for (ticker, source) on or before
// date. Empty date behaves like LatestSnapshot.
func (db *DB) SnapshotAt(ticker, source, date string) (payload, at string, err error) {
	if date == "" {
		return db.LatestSnapshot(ticker, source)
	}
	err = db.QueryRow(`SELECT payload_json, date FROM snapshots
		WHERE ticker=? AND source=? AND date <= ? ORDER BY date DESC, id DESC LIMIT 1`,
		ticker, source, date).Scan(&payload, &at)
	return payload, at, err
}

// LatestSnapshots returns the newest payload per ticker for a source.
func (db *DB) LatestSnapshots(source string, tickers []string) map[string]string {
	out := map[string]string{}
	for _, t := range tickers {
		if p, _, err := db.LatestSnapshot(t, source); err == nil {
			out[t] = p
		}
	}
	return out
}

// SnapshotCount counts rows for a source.
func (db *DB) SnapshotCount(source string) int {
	var n int
	_ = db.QueryRow(`SELECT COUNT(*) FROM snapshots WHERE source=?`, source).Scan(&n)
	return n
}

// Broker/foreign-flow derived rows.

type BrokerAgg struct {
	BrokerCode string
	Buy        float64
	Sell       float64
	Net        float64
	Lots       float64
	Freq       int
	AvgPrice   float64
}

// UpsertBrokerActivity inserts or replaces one derived row.
func (db *DB) UpsertBrokerActivity(broker, ticker, date string, buy, sell, net, lots float64, freq int, avg float64) error {
	_, err := db.Exec(`INSERT INTO broker_activity(broker_code,ticker,date,buy,sell,net,lots,freq,avg_price)
		VALUES(?,?,?,?,?,?,?,?,?)
		ON CONFLICT(broker_code,ticker,date) DO UPDATE SET buy=excluded.buy,sell=excluded.sell,
		net=excluded.net,lots=excluded.lots,freq=excluded.freq,avg_price=excluded.avg_price`,
		broker, ticker, date, buy, sell, net, lots, freq, avg)
	return err
}

// NetBuySum5d sums net per broker over the last 5 stored days for a ticker.
func (db *DB) NetBuySum5d(ticker string) (map[string]float64, error) {
	rows, err := db.Query(`SELECT broker_code, SUM(net) FROM broker_activity
		WHERE ticker=? AND date >= (SELECT MAX(date) FROM broker_activity WHERE ticker=?)
		GROUP BY broker_code`, ticker, ticker)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]float64{}
	for rows.Next() {
		var code string
		var net float64
		if err := rows.Scan(&code, &net); err != nil {
			return nil, err
		}
		out[code] = net
	}
	return out, rows.Err()
}

// UpsertForeignFlow inserts or replaces one foreign-flow row.
func (db *DB) UpsertForeignFlow(ticker, date string, net float64) error {
	_, err := db.Exec(`INSERT INTO foreign_flow(ticker,date,net_inflow) VALUES(?,?,?)
		ON CONFLICT(ticker,date) DO UPDATE SET net_inflow=excluded.net_inflow`, ticker, date, net)
	return err
}

// ForeignLast6 returns up to the last 6 stored days, oldest first.
func (db *DB) ForeignLast6(ticker string) (dates []string, nets []float64, err error) {
	rows, err := db.Query(`SELECT date, net_inflow FROM foreign_flow
		WHERE ticker=? ORDER BY date DESC LIMIT 6`, ticker)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var d string
		var n float64
		if err := rows.Scan(&d, &n); err != nil {
			return nil, nil, err
		}
		dates, nets = append([]string{d}, dates...), append([]float64{n}, nets...)
	}
	return dates, nets, rows.Err()
}

// ForeignWindow returns the date-filtered inflow series, oldest first,
// capped at the limit most-recent points within [start, end].
func (db *DB) ForeignWindow(ticker, start, end string, limit int) (dates []string, nets []float64, err error) {
	if limit <= 0 || limit > 90 {
		limit = 30
	}
	q := `SELECT date, net_inflow FROM foreign_flow WHERE ticker=?`
	var args []any
	args = append(args, ticker)
	if start != "" {
		q += ` AND date >= ?`
		args = append(args, start)
	}
	if end != "" {
		q += ` AND date <= ?`
		args = append(args, end)
	}
	q += ` ORDER BY date ASC`
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var d string
		var n float64
		if err := rows.Scan(&d, &n); err != nil {
			return nil, nil, err
		}
		dates, nets = append(dates, d), append(nets, n)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	if len(dates) > limit {
		dates, nets = dates[len(dates)-limit:], nets[len(nets)-limit:]
	}
	return dates, nets, nil
}

// ForeignAvg30 returns the mean daily net over the last 30 stored days.
func (db *DB) ForeignAvg30(ticker string) float64 {
	var avg *float64
	_ = db.QueryRow(`SELECT AVG(net_inflow) FROM
		(SELECT net_inflow FROM foreign_flow WHERE ticker=? ORDER BY date DESC LIMIT 30)`, ticker).Scan(&avg)
	if avg == nil {
		return 0
	}
	return *avg
}

// Events: news + filings.

// InsertNews stores one analyzed article.
func (db *DB) InsertNews(ticker, date, source, sentiment string, conf float64, url, title string) error {
	_, err := db.Exec(`INSERT INTO news_items(ticker,date,source,sentiment,confidence,url,title)
		VALUES(?,?,?,?,?,?,?)`, ticker, date, source, sentiment, conf, url, title)
	return err
}

// NewsSince returns articles for a ticker on/after date.
func (db *DB) NewsSince(ticker, since string) ([]map[string]any, error) {
	rows, err := db.Query(`SELECT title,source,sentiment,confidence,url,date FROM news_items
		WHERE ticker=? AND date>=? ORDER BY date DESC LIMIT 30`, ticker, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var title, source, sent, url, date string
		var conf float64
		if err := rows.Scan(&title, &source, &sent, &conf, &url, &date); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"title": title, "source": source,
			"sentiment": sent, "confidence": conf, "url": url, "date": date})
	}
	return out, rows.Err()
}

// InsertFiling stores one insider/institution transaction.
func (db *DB) InsertFiling(ticker, date, holderType, txnType string, volume, price float64, holderName ...string) error {
	name := ""
	if len(holderName) > 0 {
		name = holderName[0]
	}
	_, err := db.Exec(`INSERT INTO filings(ticker,date,holder_type,txn_type,volume,price,holder_name)
		VALUES(?,?,?,?,?,?,?)`, ticker, date, holderType, txnType, volume, price, name)
	return err
}

// DistinctInsiders7d counts distinct holder names buying in the last 7d
// (empty names collapse to one row, so named attribution matters).
func (db *DB) DistinctInsiders7d(ticker string) int {
	n := 0
	_ = db.QueryRow(`SELECT COUNT(DISTINCT holder_name) FROM filings
		WHERE ticker=? AND txn_type='buy' AND date >= date('now','-7 days')`, ticker).Scan(&n)
	return n
}

// LatestBuyVolume returns the newest buy volume (for the 2x-avg rule).
func (db *DB) LatestBuyVolume(ticker string) float64 {
	var v *float64
	_ = db.QueryRow(`SELECT volume FROM filings WHERE ticker=? AND txn_type='buy'
		ORDER BY date DESC LIMIT 1`, ticker).Scan(&v)
	if v == nil {
		return 0
	}
	return *v
}

// FilingAvg30 returns mean buy volume over the last 30 stored days.
func (db *DB) FilingAvg30(ticker string) float64 {
	var avg *float64
	_ = db.QueryRow(`SELECT AVG(volume) FROM
		(SELECT volume FROM filings WHERE ticker=? AND txn_type='buy' ORDER BY date DESC LIMIT 30)`, ticker).Scan(&avg)
	if avg == nil {
		return 0
	}
	return *avg
}

// Routines / runs.

// Routine is one subscribed routine row.
type Routine struct {
	ID       int64
	UserKey  string
	Type     string
	Schedule string
	Channels []string
	Enabled  bool
}

// ListRoutines returns all routines for a user.
func (db *DB) ListRoutines(userKey string) ([]Routine, error) {
	rows, err := db.Query(`SELECT id,user_key,type,schedule_cron,channels_json,enabled
		FROM routines WHERE user_key=? ORDER BY id`, userKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Routine
	for rows.Next() {
		var r Routine
		var ch string
		var en int
		if err := rows.Scan(&r.ID, &r.UserKey, &r.Type, &r.Schedule, &ch, &en); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(ch), &r.Channels)
		r.Enabled = en != 0
		out = append(out, r)
	}
	return out, rows.Err()
}

// CreateRoutine inserts a routine; returns its id.
func (db *DB) CreateRoutine(userKey, typ, schedule string, channels []string) (int64, error) {
	ch, _ := json.Marshal(channels)
	res, err := db.Exec(`INSERT INTO routines(user_key,type,schedule_cron,channels_json,enabled)
		VALUES(?,?,?,?,1)`, userKey, typ, schedule, string(ch))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateRoutine patches enabled/schedule/channels. It returns (true, nil)
// when the row exists and is owned by userKey, (false, nil) when the id is
// missing or owned by someone else.
func (db *DB) UpdateRoutine(id int64, userKey string, enabled *bool, schedule string, channels []string) (bool, error) {
	touched := false
	patch := func(query string, args ...any) error {
		args = append(args, id, userKey)
		res, err := db.Exec(query+` WHERE id=? AND user_key=?`, args...)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		if n > 0 {
			touched = true
		}
		return nil
	}
	if enabled != nil {
		en := 0
		if *enabled {
			en = 1
		}
		if err := patch(`UPDATE routines SET enabled=?`, en); err != nil {
			return false, err
		}
	}
	if schedule != "" {
		if err := patch(`UPDATE routines SET schedule_cron=?`, schedule); err != nil {
			return false, err
		}
	}
	if channels != nil {
		ch, _ := json.Marshal(channels)
		if err := patch(`UPDATE routines SET channels_json=?`, string(ch)); err != nil {
			return false, err
		}
	}
	if !touched {
		// No field to patch, or id missing/unowned: distinguish by existence.
		var owner string
		if err := db.QueryRow(`SELECT user_key FROM routines WHERE id=?`, id).Scan(&owner); err != nil {
			return false, nil
		}
		if owner != userKey {
			return false, nil
		}
	}
	return touched, nil
}

// RecordRun inserts a routine_runs row; returns its id.
func (db *DB) RecordRun(routineID int64, status, payload string, credits int, userKey ...string) (int64, error) {
	key := ""
	if len(userKey) > 0 {
		key = userKey[0]
	} else {
		_ = db.QueryRow(`SELECT user_key FROM routines WHERE id=?`, routineID).Scan(&key)
	}
	res, err := db.Exec(`INSERT INTO routine_runs(routine_id,started_at,status,payload_json,credits_used,user_key)
		VALUES(?,?,?,?,?,?)`, routineID, time.Now().UTC().Format(time.RFC3339), status, payload, credits, key)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// RunHistory returns recent runs, optionally filtered by routine and user.
func (db *DB) RunHistory(routineID int64, limit int, userKey ...string) ([]map[string]any, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	user := ""
	if len(userKey) > 0 {
		user = userKey[0]
	}
	var rows *Rows2
	var err error
	switch {
	case routineID > 0 && user != "":
		rows, err = q(db, `SELECT id,routine_id,started_at,status,payload_json,credits_used
			FROM routine_runs WHERE routine_id=? AND user_key=? ORDER BY id DESC LIMIT ?`, routineID, user, limit)
	case routineID > 0:
		rows, err = q(db, `SELECT id,routine_id,started_at,status,payload_json,credits_used
			FROM routine_runs WHERE routine_id=? ORDER BY id DESC LIMIT ?`, routineID, limit)
	case user != "":
		rows, err = q(db, `SELECT id,routine_id,started_at,status,payload_json,credits_used
			FROM routine_runs WHERE user_key=? ORDER BY id DESC LIMIT ?`, user, limit)
	default:
		rows, err = q(db, `SELECT id,routine_id,started_at,status,payload_json,credits_used
			FROM routine_runs ORDER BY id DESC LIMIT ?`, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id, rid int64
		var started, status, payload string
		var credits int
		if err := rows.Scan(&id, &rid, &started, &status, &payload, &credits); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"id": id, "routine_id": rid, "started_at": started,
			"status": status, "payload": payload, "credits_used": credits})
	}
	return out, rows.Err()
}

// Alerts.

// Alert is one user rule row.
type Alert struct {
	ID        int64
	UserKey   string
	Name      string
	Rule      string
	Channels  []string
	LastFired string
}

// ListAlerts returns all alerts for a user.
func (db *DB) ListAlerts(userKey string) ([]Alert, error) {
	rows, err := db.Query(`SELECT id,user_key,name,rule_json,channels_json,last_fired
		FROM alerts WHERE user_key=? ORDER BY id`, userKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Alert
	for rows.Next() {
		var a Alert
		var ch string
		if err := rows.Scan(&a.ID, &a.UserKey, &a.Name, &a.Rule, &ch, &a.LastFired); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(ch), &a.Channels)
		out = append(out, a)
	}
	return out, rows.Err()
}

// CreateAlert inserts an alert; returns its id.
func (db *DB) CreateAlert(userKey, name, rule string, channels []string) (int64, error) {
	ch, _ := json.Marshal(channels)
	res, err := db.Exec(`INSERT INTO alerts(user_key,name,rule_json,channels_json) VALUES(?,?,?,?)`,
		userKey, name, rule, string(ch))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// DeleteAlert removes one alert owned by the user.
func (db *DB) DeleteAlert(id int64, userKey string) (bool, error) {
	res, err := db.Exec(`DELETE FROM alerts WHERE id=? AND user_key=?`, id, userKey)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// MarkAlertFired stamps last_fired=now.
func (db *DB) MarkAlertFired(id int64) error {
	_, err := db.Exec(`UPDATE alerts SET last_fired=? WHERE id=?`,
		time.Now().UTC().Format(time.RFC3339), id)
	return err
}

// InsertAlertEvent records a fired event; returns its id.
func (db *DB) InsertAlertEvent(alertID int64, ticker, date, message, ctx, cites string, userKey ...string) (int64, error) {
	key := ""
	if len(userKey) > 0 {
		key = userKey[0]
	} else {
		_ = db.QueryRow(`SELECT user_key FROM alerts WHERE id=?`, alertID).Scan(&key)
	}
	res, err := db.Exec(`INSERT INTO alert_events(alert_id,ticker,date,message,context_json,citations_json,user_key)
		VALUES(?,?,?,?,?,?,?)`, alertID, ticker, date, message, ctx, cites, key)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// AlertEventsSince returns recent events, optionally filtered by ticker
// and user. Pass userKey to scope reads to one owner.
func (db *DB) AlertEventsSince(since, ticker string, limit int, userKey ...string) ([]map[string]any, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	user := ""
	if len(userKey) > 0 {
		user = userKey[0]
	}
	var rows *Rows2
	var err error
	switch {
	case ticker != "" && user != "":
		rows, err = q(db, `SELECT id,alert_id,ticker,date,message,context_json,citations_json
			FROM alert_events WHERE date>=? AND ticker=? AND user_key=? ORDER BY id DESC LIMIT ?`, since, ticker, user, limit)
	case ticker != "":
		rows, err = q(db, `SELECT id,alert_id,ticker,date,message,context_json,citations_json
			FROM alert_events WHERE date>=? AND ticker=? ORDER BY id DESC LIMIT ?`, since, ticker, limit)
	case user != "":
		rows, err = q(db, `SELECT id,alert_id,ticker,date,message,context_json,citations_json
			FROM alert_events WHERE date>=? AND user_key=? ORDER BY id DESC LIMIT ?`, since, user, limit)
	default:
		rows, err = q(db, `SELECT id,alert_id,ticker,date,message,context_json,citations_json
			FROM alert_events WHERE date>=? ORDER BY id DESC LIMIT ?`, since, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id, aid int64
		var t, d, m, c, ci string
		if err := rows.Scan(&id, &aid, &t, &d, &m, &c, &ci); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"id": id, "alert_id": aid, "ticker": t, "date": d,
			"message": m, "context": c, "citations": ci})
	}
	return out, rows.Err()
}

// Watchlists.

// Watchlist returns tickers for a user.
func (db *DB) Watchlist(userKey string) ([]string, error) {
	rows, err := db.Query(`SELECT ticker FROM watchlists WHERE user_key=? ORDER BY ticker`, userKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// AllTickers returns every ticker that has snapshot data, excluding pseudo
// tickers (IDX market-wide rows, ROE registry rows). This is the "semua"
// default universe: dashboard, screener fallback, and scheduler depth all use
// it when a user has no personal watchlist.
func (db *DB) AllTickers() ([]string, error) {
	rows, err := db.Query(`SELECT DISTINCT ticker FROM snapshots
		WHERE ticker NOT IN ('IDX','ROE') ORDER BY ticker`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// AddWatch inserts a ticker (idempotent).
func (db *DB) AddWatch(userKey, ticker string) error {
	_, err := db.Exec(`INSERT INTO watchlists(user_key,ticker,added_at) VALUES(?,?,?)
		ON CONFLICT(user_key,ticker) DO NOTHING`, userKey, ticker, time.Now().UTC().Format(time.RFC3339))
	return err
}

// RemoveWatch deletes a ticker.
func (db *DB) RemoveWatch(userKey, ticker string) error {
	_, err := db.Exec(`DELETE FROM watchlists WHERE user_key=? AND ticker=?`, userKey, ticker)
	return err
}

// DeleteRoutine removes one routine owned by the user.
func (db *DB) DeleteRoutine(id int64, userKey string) (bool, error) {
	res, err := db.Exec(`DELETE FROM routines WHERE id=? AND user_key=?`, id, userKey)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// FilingsSince returns filings for a ticker on/after date, newest first.
func (db *DB) FilingsSince(ticker, since string, limit int) ([]map[string]any, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	rows, err := db.Query(`SELECT date, holder_type, txn_type, volume, price FROM filings
		WHERE ticker=? AND date>=? ORDER BY date DESC LIMIT ?`, ticker, since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var d, h, t string
		var v, pr float64
		if err := rows.Scan(&d, &h, &t, &v, &pr); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"date": d, "holder_type": h,
			"txn_type": t, "volume": v, "price": pr})
	}
	return out, rows.Err()
}

// Reports reports / briefings.

// SaveReport stores a generated report; returns its id.
func (db *DB) SaveReport(ticker, payload, cites string) (int64, error) {
	res, err := db.Exec(`INSERT INTO reports(ticker,generated_at,payload_json,citations_json)
		VALUES(?,?,?,?)`, ticker, time.Now().UTC().Format(time.RFC3339), payload, cites)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ListReports returns recent report metadata for a ticker, newest first.
func (db *DB) ListReports(ticker string, limit int) ([]map[string]any, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := db.Query(`SELECT id, generated_at FROM reports
		WHERE ticker=? ORDER BY id DESC LIMIT ?`, ticker, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id int64
		var at string
		if err := rows.Scan(&id, &at); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"id": id, "ticker": ticker, "generated_at": at})
	}
	return out, rows.Err()
}

// LatestReport returns the newest report payload for a ticker.
func (db *DB) LatestReport(ticker string) (payload, cites, at string, err error) {
	err = db.QueryRow(`SELECT payload_json, citations_json, generated_at FROM reports
		WHERE ticker=? ORDER BY id DESC LIMIT 1`, ticker).Scan(&payload, &cites, &at)
	return payload, cites, at, err
}

// SaveBriefing upserts today's briefing.
func (db *DB) SaveBriefing(date, payload, cites string) error {
	_, err := db.Exec(`INSERT INTO briefings(date,payload_json,citations_json) VALUES(?,?,?)
		ON CONFLICT(date) DO UPDATE SET payload_json=excluded.payload_json,
		citations_json=excluded.citations_json`, date, payload, cites)
	return err
}

// SaveNarasi caches the LLM-polished narration for a briefing date
// (best-effort; empty string clears it).
func (db *DB) SaveNarasi(date, narasi string) error {
	_, err := db.Exec(`UPDATE briefings SET narasi=? WHERE date=?`, narasi, date)
	return err
}

// LatestBriefing returns the newest briefing (plus cached narration).
func (db *DB) LatestBriefing() (date, payload, cites, narasi string, err error) {
	err = db.QueryRow(`SELECT date, payload_json, citations_json,
		COALESCE(narasi,'') FROM briefings
		ORDER BY date DESC LIMIT 1`).Scan(&date, &payload, &cites, &narasi)
	return date, payload, cites, narasi, err
}

// Accuracy ledger.

// RecordPrediction logs one agent call for later +30d resolution.
func (db *DB) RecordPrediction(agent, ticker, prediction string) error {
	_, err := db.Exec(`INSERT INTO agent_accuracy(agent,ticker,prediction,predict_date)
		VALUES(?,?,?,?)`, agent, ticker, prediction, time.Now().UTC().Format("2006-01-02"))
	return err
}

// ResolvePrediction marks a prediction hit/miss with actual return.
func (db *DB) ResolvePrediction(id int64, hit bool, actualReturn float64) error {
	h := 0
	if hit {
		h = 1
	}
	_, err := db.Exec(`UPDATE agent_accuracy SET resolved=1, hit=?, actual_return=? WHERE id=?`,
		h, actualReturn, id)
	return err
}

// DueForResolution returns unresolved predictions older than 30d.
func (db *DB) DueForResolution() ([]map[string]any, error) {
	rows, err := db.Query(`SELECT id, agent, ticker, prediction, predict_date FROM agent_accuracy
		WHERE resolved=0 AND predict_date <= date('now','-30 days')`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id int64
		var a, t, p, d string
		if err := rows.Scan(&id, &a, &t, &p, &d); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"id": id, "agent": a, "ticker": t, "prediction": p, "predict_date": d})
	}
	return out, rows.Err()
}

// AccuracyStats returns per-agent {calls, resolved, hits, hit_rate}.
func (db *DB) AccuracyStats() ([]map[string]any, error) {
	rows, err := db.Query(`SELECT agent, COUNT(*),
		SUM(CASE WHEN resolved=1 THEN 1 ELSE 0 END),
		SUM(CASE WHEN resolved=1 AND hit=1 THEN 1 ELSE 0 END)
		FROM agent_accuracy GROUP BY agent`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var agent string
		var calls, resolved, hits int
		if err := rows.Scan(&agent, &calls, &resolved, &hits); err != nil {
			return nil, err
		}
		rate := 0.0
		if resolved > 0 {
			rate = float64(hits) / float64(resolved)
		}
		out = append(out, map[string]any{"agent": agent, "calls": calls,
			"resolved": resolved, "hits": hits, "hit_rate": rate})
	}
	return out, rows.Err()
}

// AccuracyWeight returns the ledger weight for an agent (0.5 default, hit-rate
// blended once ≥10 resolved calls exist).
func (db *DB) AccuracyWeight(agent string) float64 {
	var resolved, hits int
	_ = db.QueryRow(`SELECT COUNT(*), SUM(CASE WHEN hit=1 THEN 1 ELSE 0 END)
		FROM agent_accuracy WHERE agent=? AND resolved=1`, agent).Scan(&resolved, &hits)
	if resolved < 10 {
		return 0.5
	}
	return 0.3 + 0.7*float64(hits)/float64(resolved)
}

// Credit ledger.

// AddSpend accumulates one counted call into credit_ledger.
func (db *DB) AddSpend(date, endpoint string, calls, credits int) {
	_, _ = db.Exec(`INSERT INTO credit_ledger(date,endpoint,calls,credits) VALUES(?,?,?,?)
		ON CONFLICT(date,endpoint) DO UPDATE SET calls=calls+excluded.calls,
		credits=credits+excluded.credits`, date, endpoint, calls, credits)
}

// CreditsToday sums today's credits.
func (db *DB) CreditsToday(date string) int {
	var n *int
	_ = db.QueryRow(`SELECT SUM(credits) FROM credit_ledger WHERE date=?`, date).Scan(&n)
	if n == nil {
		return 0
	}
	return *n
}

// PruneSnapshots deletes snapshot rows older than cutoff (YYYY-MM-DD),
// keeping the newest row per (ticker, source) regardless of age so offline
// seeds and last-good fallbacks always survive. Returns rows deleted.
func (db *DB) PruneSnapshots(cutoff string) (int64, error) {
	res, err := db.Exec(`DELETE FROM snapshots WHERE date < ? AND id NOT IN
		(SELECT MAX(id) FROM snapshots GROUP BY ticker, source)`, cutoff)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// Daily volumes for technical/volume rules.

// DailyVolumes returns date-ordered volumes for a ticker from snapshots.
func (db *DB) DailyVolumes(ticker string, limit int) ([]float64, []string, error) {
	rows, err := db.Query(`SELECT date, payload_json FROM snapshots
		WHERE ticker=? AND source='daily' ORDER BY date DESC LIMIT ?`, ticker, limit)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var vols []float64
	var dates []string
	for rows.Next() {
		var d, p string
		if err := rows.Scan(&d, &p); err != nil {
			return nil, nil, err
		}
		for _, b := range parseDailyBars(p, d) {
			if b.Volume > 0 {
				vols, dates = append(vols, b.Volume), append(dates, b.Date)
			}
		}
	}
	return vols, dates, rows.Err()
}

// LatestClose returns the most recent close for a ticker.
func (db *DB) LatestClose(ticker string) (float64, string, error) {
	var p, d string
	err := db.QueryRow(`SELECT payload_json, date FROM snapshots
		WHERE ticker=? AND source='daily' ORDER BY date DESC LIMIT 1`, ticker).Scan(&p, &d)
	if err != nil {
		return 0, "", err
	}
	bars := parseDailyBars(p, d)
	if len(bars) == 0 {
		return 0, "", fmt.Errorf("store: decode daily close: no bars")
	}
	last := bars[len(bars)-1]
	return last.Close, last.Date, nil
}

// dailyBar is one normalized OHLCV row from any stored daily payload shape.
type dailyBar struct {
	Date   string
	Close  float64
	Volume float64
}

// parseDailyBars normalizes both stored shapes: a JSON array of bars or a
// single bar object. Missing bar dates fall back to the snapshot row date.
func parseDailyBars(raw, rowDate string) []dailyBar {
	var bars []struct {
		Date   string  `json:"date"`
		Close  float64 `json:"close"`
		Volume float64 `json:"volume"`
	}
	if json.Unmarshal([]byte(raw), &bars) == nil && len(bars) > 0 {
		out := make([]dailyBar, 0, len(bars))
		for _, b := range bars {
			if b.Date == "" {
				b.Date = rowDate
			}
			out = append(out, dailyBar{b.Date, b.Close, b.Volume})
		}
		return out
	}
	var one struct {
		Date   string  `json:"date"`
		Close  float64 `json:"close"`
		Volume float64 `json:"volume"`
	}
	if json.Unmarshal([]byte(raw), &one) == nil && (one.Close > 0 || one.Volume > 0) {
		if one.Date == "" {
			one.Date = rowDate
		}
		return []dailyBar{{one.Date, one.Close, one.Volume}}
	}
	return nil
}

// Rows2 aliases sql.Rows so helpers stay testable without exporting internals.
type Rows2 = sql.Rows

// q runs a query against the embedded DB.
func q(db *DB, query string, args ...any) (*Rows2, error) {
	return db.Query(query, args...)
}

// Destinations: per-user push targets (CRUD, dynamic per login).

// Destination kinds.
const (
	DestTelegram = "telegram"
	DestDiscord  = "discord"
)

// Destination is one user push target. Secrets are stored server-side and
// never included in API responses (see ListDestinations masking).
type Destination struct {
	ID         int64
	UserKey    string
	Kind       string
	Label      string
	BotToken   string
	ChatID     string
	WebhookURL string
	Enabled    bool
	CreatedAt  string
}

// ListDestinations returns all push targets for a user, newest last.
func (db *DB) ListDestinations(userKey string) ([]Destination, error) {
	rows, err := db.Query(`SELECT id,user_key,kind,label,bot_token,chat_id,webhook_url,enabled,created_at
		FROM notification_destinations WHERE user_key=? ORDER BY id`, userKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Destination
	for rows.Next() {
		var d Destination
		var en int
		if err := rows.Scan(&d.ID, &d.UserKey, &d.Kind, &d.Label, &d.BotToken, &d.ChatID, &d.WebhookURL, &en, &d.CreatedAt); err != nil {
			return nil, err
		}
		d.Enabled = en != 0
		out = append(out, d)
	}
	return out, rows.Err()
}

// ListEnabledDestinations returns only enabled push targets for a user.
func (db *DB) ListEnabledDestinations(userKey string) ([]Destination, error) {
	all, err := db.ListDestinations(userKey)
	if err != nil {
		return nil, err
	}
	var out []Destination
	for _, d := range all {
		if d.Enabled {
			out = append(out, d)
		}
	}
	return out, nil
}

// CreateDestination inserts a push target; returns its id.
func (db *DB) CreateDestination(d Destination) (int64, error) {
	en := 0
	if d.Enabled {
		en = 1
	}
	res, err := db.Exec(`INSERT INTO notification_destinations
		(user_key,kind,label,bot_token,chat_id,webhook_url,enabled,created_at)
		VALUES(?,?,?,?,?,?,?,?)`,
		d.UserKey, d.Kind, d.Label, d.BotToken, d.ChatID, d.WebhookURL, en,
		time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateDestination patches label/enabled/secrets. Kind is immutable.
// A nil enabled leaves the flag untouched; empty secret strings mean keep.
func (db *DB) UpdateDestination(id int64, userKey string, label *string, enabled *bool, botToken, chatID, webhookURL *string) (bool, error) {
	cur, err := db.GetDestination(id, userKey)
	if err != nil {
		return false, err
	}
	if cur == nil {
		return false, nil
	}
	if label != nil {
		cur.Label = *label
	}
	if enabled != nil {
		cur.Enabled = *enabled
	}
	if botToken != nil && *botToken != "" {
		cur.BotToken = *botToken
	}
	if chatID != nil && *chatID != "" {
		cur.ChatID = *chatID
	}
	if webhookURL != nil && *webhookURL != "" {
		cur.WebhookURL = *webhookURL
	}
	en := 0
	if cur.Enabled {
		en = 1
	}
	res, err := db.Exec(`UPDATE notification_destinations
		SET label=?, bot_token=?, chat_id=?, webhook_url=?, enabled=? WHERE id=? AND user_key=?`,
		cur.Label, cur.BotToken, cur.ChatID, cur.WebhookURL, en, id, userKey)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// GetDestination returns one push target owned by the user, or nil.
func (db *DB) GetDestination(id int64, userKey string) (*Destination, error) {
	all, err := db.ListDestinations(userKey)
	if err != nil {
		return nil, err
	}
	for _, d := range all {
		if d.ID == id {
			c := d
			return &c, nil
		}
	}
	return nil, nil
}

// DeleteDestination removes one push target owned by the user.
func (db *DB) DeleteDestination(id int64, userKey string) (bool, error) {
	res, err := db.Exec(`DELETE FROM notification_destinations WHERE id=? AND user_key=?`, id, userKey)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// DistinctUsers lists user_keys owning rows in table (alerts or routines).
func (db *DB) DistinctUsers(table string) []string {
	if table != "alerts" && table != "routines" {
		return nil
	}
	rows, err := db.Query(`SELECT DISTINCT user_key FROM ` + table + ` ORDER BY user_key`)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var u string
		if err := rows.Scan(&u); err != nil {
			return out
		}
		if u != "" {
			out = append(out, u)
		}
	}
	return out
}
