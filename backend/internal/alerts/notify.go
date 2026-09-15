package alerts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Target is one push destination resolved per user at send time.
type Target struct {
	TelegramToken  string
	TelegramChatID string
	DiscordURL     string
}

// Notifier delivers alert cards to Telegram + Discord webhooks with context +
// citations. Missing credentials degrade to a no-op (logged, not fatal) so
// offline/demo runs never fail on delivery.
type Notifier struct {
	TelegramToken  string
	TelegramChatID string
	DiscordURL     string
	http           *http.Client
	// Targets, when set, resolves per-owner push destinations. It is consulted
	// on every Send so destination CRUD takes effect immediately.
	Targets func(owner string) []Target
}

// NewNotifier builds a notifier; empty creds mean dry-run mode.
func NewNotifier(tgToken, tgChat, discordURL string) *Notifier {
	return &Notifier{TelegramToken: tgToken, TelegramChatID: tgChat,
		DiscordURL: discordURL, http: &http.Client{Timeout: 15 * time.Second}}
}

// DryRun reports whether no server-level channel is configured.
func (n *Notifier) DryRun() bool {
	return n.TelegramToken == "" && n.DiscordURL == ""
}

// DryRunFor reports whether an owner has no push target anywhere: neither
// the owner's enabled destinations nor the server fallback.
func (n *Notifier) DryRunFor(owner string) bool {
	return len(n.targetsFor(owner)) == 0
}

// targetsFor resolves push targets: per-owner destinations first, then the
// server-level env fallback. Owner "" means server fallback only.
func (n *Notifier) targetsFor(owner string) []Target {
	var out []Target
	if n.Targets != nil && owner != "" {
		out = append(out, n.Targets(owner)...)
	}
	if n.TelegramToken != "" && n.TelegramChatID != "" {
		out = append(out, Target{TelegramToken: n.TelegramToken, TelegramChatID: n.TelegramChatID})
	}
	if n.DiscordURL != "" {
		out = append(out, Target{DiscordURL: n.DiscordURL})
	}
	return out
}

// Card is the rendered alert text shared by both channels.
func Card(f Finding, cites string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "*%s* — %s\n%s", f.Ticker, f.Rule, f.Message)
	if len(f.Context) > 0 {
		raw, _ := json.Marshal(f.Context)
		fmt.Fprintf(&b, "\n`%s`", string(raw))
	}
	if cites != "" {
		fmt.Fprintf(&b, "\nCitations: %s", cites)
	}
	fmt.Fprintf(&b, "\nDetail: /report/%s", f.Ticker)
	return b.String()
}

// Send delivers one finding to all configured channels (server fallback).
// Prefer SendTo so delivery fans out to the finding owner's destinations.
func (n *Notifier) Send(ctx context.Context, f Finding, cites string) error {
	return n.SendTo(ctx, "", f, cites)
}

// SendTo delivers one finding to the owner's enabled destinations plus the
// server fallback. With no targets anywhere it is record-only (nil).
func (n *Notifier) SendTo(ctx context.Context, owner string, f Finding, cites string) error {
	targets := n.targetsFor(owner)
	if len(targets) == 0 {
		return nil // recorded in alert_events regardless
	}
	if cites == "" && len(f.Citations) > 0 {
		cc, _ := json.Marshal(f.Citations)
		cites = string(cc)
	}
	text := Card(f, cites)
	var firstErr error
	for _, t := range targets {
		if t.TelegramToken != "" && t.TelegramChatID != "" {
			if err := sendTelegram(ctx, n.http, t.TelegramToken, t.TelegramChatID, text); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		if t.DiscordURL != "" {
			if err := sendDiscord(ctx, n.http, t.DiscordURL, text); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

func (n *Notifier) telegram(ctx context.Context, text string) error {
	return sendTelegram(ctx, n.http, n.TelegramToken, n.TelegramChatID, text)
}

func sendTelegram(ctx context.Context, client *http.Client, token, chatID, text string) error {
	u := "https://api.telegram.org/bot" + token + "/sendMessage"
	body, _ := json.Marshal(map[string]any{
		"chat_id": chatID, "text": text, "parse_mode": "Markdown",
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("alerts: telegram: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("alerts: telegram HTTP %d", resp.StatusCode)
	}
	return nil
}

func (n *Notifier) discord(ctx context.Context, text string) error {
	return sendDiscord(ctx, n.http, n.DiscordURL, text)
}

func sendDiscord(ctx context.Context, client *http.Client, webhookURL, text string) error {
	if len(text) > 1900 {
		text = text[:1900] + "…"
	}
	body, _ := json.Marshal(map[string]any{"content": text})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("alerts: discord: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("alerts: discord HTTP %d", resp.StatusCode)
	}
	return nil
}
