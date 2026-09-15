// Package llm calls an OpenAI-compatible chat endpoint for synthesis, report
// interrogation, and sentiment triage. Two models: LLM_MODEL_TRIAGE (cheap)
// and LLM_MODEL_SYNTH (strong), both overridden via env. No LLM call is on
// the critical detection path — rules and scores are computed locally first.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Client talks to an OpenAI-compatible /chat/completions endpoint.
type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

// New builds a client; baseURL is like https://api.openai.com/v1.
func New(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		apiKey:  apiKey,
		http:    &http.Client{Timeout: 60 * time.Second},
	}
}

// Available reports whether LLM calls are configured.
func (c *Client) Available() bool { return c.baseURL != "" && c.apiKey != "" }

type chatMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Complete sends one chat completion and returns the text content.
func (c *Client) Complete(ctx context.Context, model, system, user string, maxTokens int) (string, error) {
	if !c.Available() {
		return "", fmt.Errorf("llm: LLM_BASE_URL/LLM_API_KEY not configured")
	}
	if maxTokens <= 0 {
		maxTokens = 800
	}
	body, _ := json.Marshal(map[string]any{
		"model":      model,
		"messages":   []chatMsg{{Role: "system", Content: system}, {Role: "user", Content: user}},
		"max_tokens": maxTokens,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("llm: %w", err)
	}
	defer resp.Body.Close()
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("llm: decode: %w", err)
	}
	if out.Error != nil {
		return "", fmt.Errorf("llm: %s", out.Error.Message)
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("llm: empty response")
	}
	return out.Choices[0].Message.Content, nil
}

// SentimentTriage classifies one article; falls back to neutral on any error
// so sentiment never blocks the pipeline.
func (c *Client) SentimentTriage(ctx context.Context, model, title, body string) (string, float64) {
	if !c.Available() {
		return "neutral", 0.5
	}
	text, err := c.Complete(ctx, model,
		`Classify Indonesian stock news as bullish, bearish, or neutral. Reply with exactly: <label> <confidence 0-1>. No other text.`,
		"Title: "+title+"\nBody: "+head(body, 1500), 20)
	if err != nil {
		return "neutral", 0.5
	}
	parts := strings.Fields(strings.ToLower(text))
	if len(parts) == 0 {
		return "neutral", 0.5
	}
	label := parts[0]
	if label != "bullish" && label != "bearish" {
		label = "neutral"
	}
	var conf float64 = 0.6
	if len(parts) > 1 {
		fmt.Sscanf(parts[1], "%f", &conf)
	}
	if conf < 0 || conf > 1 {
		conf = 0.6
	}
	return label, conf
}

func head(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
