package agents

import (
	"context"
	"fmt"
	"strings"

	"flowsight/internal/model"
	"flowsight/internal/sectors"
)

// bullish/bearish keyword lists for the offline fallback path (no LLM key).
var bullishWords = []string{"laba naik", "profit up", "bullish", "upgrade", "buyback", "dividen naik", "akuisisi", "ekspansi", "rekor", "tumbuh", "naik", "positive", "growth", "record profit"}
var bearishWords = []string{"rugi", "turun", "bearish", "downgrade", "suspend", "gagal", "skandal", "fraud", "loss", "drop", "plunge", "warning", "penurunan"}

// AnalyzeSentiment (A3) aggregates news + filings + suspensions.
// Adaptive RAG: LLM triage when configured, keyword fallback offline.
// Rare tickers (fewer than 3 articles) force grounding: every claim cites.
func AnalyzeSentiment(ctx context.Context, d Deps, ticker string) model.AgentResult {
	ticker = strings.ToUpper(ticker)
	res := model.AgentResult{Summary: "no news snapshots available"}

	var news struct {
		Results []sectors.NewsArticle `json:"results"`
	}
	newsDate, newsOK := payload(d.DB, ticker, "news", &news)
	var filings struct {
		Results []sectors.Filing `json:"results"`
	}
	filDate, filOK := payload(d.DB, ticker, "filings", &filings)
	var susp struct {
		Results []sectors.Suspension `json:"results"`
	}
	suspDate, suspOK := payload(d.DB, ticker, "suspensions", &susp)
	if !newsOK && !filOK && !suspOK {
		// Fall back to derived news_items table (seed path).
		if arts, err := d.DB.NewsSince(ticker, "2000-01-01"); err == nil && len(arts) > 0 {
			return sentimentFromStored(ticker, arts)
		}
		return res
	}
	if newsOK {
		res.Citations = append(res.Citations, model.Cite("v2/news/", ticker, newsDate))
	}
	if filOK {
		res.Citations = append(res.Citations, model.Cite("v2/filings/", ticker, filDate))
	}
	if suspOK {
		res.Citations = append(res.Citations, model.Cite("v2/suspensions/", ticker, suspDate))
	}

	pos, neg, neu := 0, 0, 0
	var keyEvents []string
	useLLM := d.LLM != nil && d.LLM.Available()
	// Cap LLM triage calls: each is one HTTP round-trip to OmniRoute
	// (~5-15s). More than 5 articles and report generation exceeds the
	// 60s reverse-proxy read timeout -> 504 with a persisted-but-stale
	// report. Rules path covers the rest via keywordSentiment.
	const maxLLMTriage = 5
	for i, a := range news.Results {
		if i >= 20 {
			break
		}
		label, conf := "neutral", 0.5
		if useLLM && i < maxLLMTriage {
			label, conf = d.LLM.SentimentTriage(ctx, d.TriageModel, a.Title, a.Body)
		} else {
			label, conf = keywordSentiment(a.Title + " " + a.Body)
		}
		switch label {
		case "bullish":
			pos++
		case "bearish":
			neg++
		default:
			neu++
		}
		if len(keyEvents) < 5 && (label != "neutral" || len(news.Results) < 3) {
			keyEvents = append(keyEvents, fmt.Sprintf("%s [%s %.0f%%]", a.Title, label, conf*100))
		}
		_ = conf
	}

	insiderLine := "no insider filings"
	buys, sells := 0, 0
	for _, f := range filings.Results {
		switch strings.ToLower(f.TransactionType) {
		case "buy":
			buys++
		case "sell":
			sells++
		}
	}
	if buys+sells > 0 {
		insiderLine = fmt.Sprintf("%d buys vs %d sells", buys, sells)
	}

	suspLine := ""
	if len(susp.Results) > 0 {
		s := susp.Results[0]
		suspLine = fmt.Sprintf("SUSPENDED %s: %s", s.SuspensionDate, s.Reason)
		res.Flags = append(res.Flags, "suspended")
	}

	total := pos + neg + neu
	score := 0.0
	if total > 0 {
		score = float64(pos-neg) / float64(total)
	}
	// Insider tilt: net buys nudge positive.
	if buys > sells {
		score += 0.1
	} else if sells > buys {
		score -= 0.1
	}
	res.Score = clampScore(score, -1, 1)

	trend := "stable"
	switch {
	case score > 0.2:
		trend = "improving"
	case score < -0.2:
		trend = "deteriorating"
	}
	res.Values = []model.Value{{
		Label:     "sentiment",
		Display:   fmt.Sprintf("%s (bullish %d / bearish %d / neutral %d)", trend, pos, neg, neu),
		Citations: res.Citations,
	}, {
		Label:     "insider",
		Display:   insiderLine,
		Citations: res.Citations,
	}}
	if suspLine != "" {
		res.Values = append(res.Values, model.Value{Label: "suspension", Display: suspLine, Citations: res.Citations})
	}
	if len(keyEvents) > 0 {
		res.Values = append(res.Values, model.Value{
			Label:     "key events",
			Display:   strings.Join(keyEvents, " | "),
			Citations: res.Citations,
		})
	}
	res.Summary = fmt.Sprintf("%s: score %+.2f, %d articles, %s", trend, res.Score, total, insiderLine)
	res.Extra = map[string]any{
		"trend": trend, "bullish": pos, "bearish": neg, "neutral": neu,
		"insider_buys": buys, "insider_sells": sells, "key_events": keyEvents,
	}
	return res
}

// sentimentFromStored builds a result from the derived news_items table.
func sentimentFromStored(ticker string, arts []map[string]any) model.AgentResult {
	res := model.AgentResult{}
	pos, neg := 0, 0
	var keys []string
	for _, a := range arts {
		s, _ := a["sentiment"].(string)
		switch s {
		case "bullish":
			pos++
		case "bearish":
			neg++
		}
		if t, _ := a["title"].(string); t != "" && len(keys) < 5 {
			keys = append(keys, t)
		}
	}
	total := len(arts)
	score := 0.0
	if total > 0 {
		score = float64(pos-neg) / float64(total)
	}
	res.Score = clampScore(score, -1, 1)
	trend := "stable"
	if score > 0.2 {
		trend = "improving"
	} else if score < -0.2 {
		trend = "deteriorating"
	}
	res.Citations = []model.Citation{model.Cite("v2/news/", ticker, "stored")}
	res.Values = []model.Value{
		{Label: "sentiment", Display: fmt.Sprintf("%s (%d articles)", trend, total), Citations: res.Citations},
	}
	if len(keys) > 0 {
		res.Values = append(res.Values, model.Value{Label: "key events", Display: strings.Join(keys, " | "), Citations: res.Citations})
	}
	res.Summary = fmt.Sprintf("%s: score %+.2f from %d stored articles", trend, res.Score, total)
	res.Extra = map[string]any{"trend": trend, "key_events": keys}
	return res
}

// keywordSentiment is the offline fallback classifier.
func keywordSentiment(text string) (string, float64) {
	t := strings.ToLower(text)
	p, n := 0, 0
	for _, w := range bullishWords {
		if strings.Contains(t, w) {
			p++
		}
	}
	for _, w := range bearishWords {
		if strings.Contains(t, w) {
			n++
		}
	}
	switch {
	case p > n:
		return "bullish", 0.6
	case n > p:
		return "bearish", 0.6
	default:
		return "neutral", 0.5
	}
}
