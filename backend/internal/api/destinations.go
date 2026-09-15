package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"flowsight/internal/store"
)

// destOut is the masked API shape: secrets never leave the server.
type destOut struct {
	ID         int64  `json:"id"`
	Kind       string `json:"kind"`
	Label      string `json:"label"`
	Enabled    bool   `json:"enabled"`
	Configured bool   `json:"configured"`
}

func maskDestinations(rows []store.Destination) []destOut {
	out := make([]destOut, 0, len(rows))
	for _, d := range rows {
		cfg := false
		switch d.Kind {
		case store.DestTelegram:
			cfg = d.BotToken != "" && d.ChatID != ""
		case store.DestDiscord:
			cfg = d.WebhookURL != ""
		}
		out = append(out, destOut{ID: d.ID, Kind: d.Kind, Label: d.Label, Enabled: d.Enabled, Configured: cfg})
	}
	return out
}

// ListDestinations serves GET /api/destinations (secrets masked).
func (s *Server) ListDestinations(w http.ResponseWriter, r *http.Request) {
	rows, err := s.DB.ListDestinations(s.userKey(r))
	if err != nil {
		writeErr(w, http.StatusBadGateway, "db: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"destinations": maskDestinations(rows)})
}

// CreateDestination serves POST /api/destinations.
func (s *Server) CreateDestination(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Kind       string `json:"kind" validate:"required,oneof=telegram discord"`
		Label      string `json:"label"`
		BotToken   string `json:"bot_token"`
		ChatID     string `json:"chat_id"`
		WebhookURL string `json:"webhook_url"`
		Enabled    *bool  `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	req.Kind = strings.ToLower(strings.TrimSpace(req.Kind))
	if err := s.Validate.Struct(req); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, "kind must be telegram or discord")
		return
	}
	if msg := checkDestSecrets(req.Kind, req.BotToken, req.ChatID, req.WebhookURL); msg != "" {
		writeErr(w, http.StatusUnprocessableEntity, msg)
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	id, err := s.DB.CreateDestination(store.Destination{
		UserKey: s.userKey(r), Kind: req.Kind, Label: req.Label,
		BotToken: req.BotToken, ChatID: req.ChatID, WebhookURL: req.WebhookURL,
		Enabled: enabled,
	})
	if err != nil {
		writeErr(w, http.StatusBadGateway, "db: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "kind": req.Kind})
}

// UpdateDestination serves PATCH /api/destinations/:id. Kind is immutable;
// omitted secret fields keep their stored value.
func (s *Server) UpdateDestination(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req struct {
		Label      *string `json:"label"`
		Enabled    *bool   `json:"enabled"`
		BotToken   *string `json:"bot_token"`
		ChatID     *string `json:"chat_id"`
		WebhookURL *string `json:"webhook_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	ok, err := s.DB.UpdateDestination(id, s.userKey(r), req.Label, req.Enabled, req.BotToken, req.ChatID, req.WebhookURL)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "db: "+err.Error())
		return
	}
	if !ok {
		writeErr(w, http.StatusNotFound, "destination not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "ok": true})
}

// DeleteDestination serves DELETE /api/destinations/:id.
func (s *Server) DeleteDestination(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	ok, err := s.DB.DeleteDestination(id, s.userKey(r))
	if err != nil {
		writeErr(w, http.StatusBadGateway, "db: "+err.Error())
		return
	}
	if !ok {
		writeErr(w, http.StatusNotFound, "destination not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "ok": true})
}

// checkDestSecrets validates kind-appropriate secrets.
func checkDestSecrets(kind, botToken, chatID, webhookURL string) string {
	switch kind {
	case store.DestTelegram:
		if strings.TrimSpace(botToken) == "" || strings.TrimSpace(chatID) == "" {
			return "telegram needs bot_token and chat_id"
		}
	case store.DestDiscord:
		u := strings.TrimSpace(webhookURL)
		if u == "" {
			return "discord needs webhook_url"
		}
		if !strings.HasPrefix(u, "https://") {
			return "webhook_url must be https"
		}
	default:
		return "kind must be telegram or discord"
	}
	return ""
}
