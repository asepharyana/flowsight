// Package config loads FlowSight runtime configuration from the environment.
// Secrets come only from env vars, never from files.
package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds all runtime settings for the FlowSight backend.
type Config struct {
	Port              string
	SectorsAPIKey     string
	SectorsBaseURL    string
	DBPath            string
	RedisURL          string
	DemoUserKey       string
	LLMBaseURL        string
	LLMAPIKey         string
	LLMTriage         string
	LLMSynth          string
	TelegramBotToken  string
	TelegramChatID    string
	DiscordWebhookURL string
	CreditCapPerCycle int
	Watchlist         []string
	// StaticDir serves the prebuilt web dist (WEB_DIST_DIR). Empty = API only.
	StaticDir string
	// Google OAuth (login). Empty client ID = auth endpoints 404.
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string
}

// HasGoogle reports whether Google OAuth login is configured.
func (c Config) HasGoogle() bool {
	return strings.TrimSpace(c.GoogleClientID) != "" &&
		strings.TrimSpace(c.GoogleClientSecret) != "" &&
		strings.TrimSpace(c.GoogleRedirectURL) != ""
}

// HasSectorsKey reports whether live Sectors API calls are possible.
// Without a key the server runs in offline/seed mode.
func (c Config) HasSectorsKey() bool { return strings.TrimSpace(c.SectorsAPIKey) != "" }

// HasLLM reports whether LLM-backed refinement is available.
func (c Config) HasLLM() bool {
	return strings.TrimSpace(c.LLMBaseURL) != "" && strings.TrimSpace(c.LLMAPIKey) != ""
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

// Load reads configuration from the environment (.env supported for dev).
func Load() Config {
	_ = godotenv.Load()
	watch := getenv("WATCHLIST", "BBCA,BBRI,BMRI,TLKM,ASII")
	tickers := make([]string, 0, 8)
	for _, t := range strings.Split(watch, ",") {
		if t = strings.ToUpper(strings.TrimSpace(t)); t != "" {
			tickers = append(tickers, t)
		}
	}
	return Config{
		Port:              getenv("PORT", "8080"),
		SectorsAPIKey:     os.Getenv("SECTORS_API_KEY"),
		SectorsBaseURL:    getenv("SECTORS_BASE_URL", "https://api.sectors.app/v2/"),
		DBPath:            getenv("DB_PATH", "data/flowsight.db"),
		RedisURL:          os.Getenv("REDIS_URL"),
		DemoUserKey:       getenv("DEMO_USER_KEY", "demo"),
		LLMBaseURL:        os.Getenv("LLM_BASE_URL"),
		LLMAPIKey:         os.Getenv("LLM_API_KEY"),
		LLMTriage:         getenv("LLM_MODEL_TRIAGE", "gpt-4o-mini"),
		LLMSynth:          getenv("LLM_MODEL_SYNTH", "gpt-4o"),
		TelegramBotToken:  os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramChatID:    os.Getenv("TELEGRAM_CHAT_ID"),
		DiscordWebhookURL: os.Getenv("DISCORD_WEBHOOK_URL"),
		CreditCapPerCycle: getenvInt("CREDIT_CAP_PER_CYCLE", 120),
		Watchlist:         tickers,
		StaticDir:         os.Getenv("WEB_DIST_DIR"),
		GoogleClientID:    os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleRedirectURL:  os.Getenv("GOOGLE_REDIRECT_URL"),
	}
}
