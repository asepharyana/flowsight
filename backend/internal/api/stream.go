package api

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Hub fans SSE events out to connected browsers. Channels: agents
// (status+scores during runs), alerts (new events), activity (feed rows).
// Heartbeat 15s; reconnect resumes from last event ID (best-effort replay of
// the last 50 events).
type Hub struct {
	mu      sync.Mutex
	subs    map[chan SSEEvent]bool
	history []SSEEvent
	nextID  int64
}

// SSEEvent is one server-sent event.
type SSEEvent struct {
	ID      int64
	Channel string
	Data    string
}

// NewHub builds an empty hub.
func NewHub() *Hub { return &Hub{subs: map[chan SSEEvent]bool{}} }

// Publish broadcasts to all subscribers and appends to history.
func (h *Hub) Publish(channel, data string) {
	h.mu.Lock()
	h.nextID++
	ev := SSEEvent{ID: h.nextID, Channel: channel, Data: data}
	h.history = append(h.history, ev)
	if len(h.history) > 50 {
		h.history = h.history[len(h.history)-50:]
	}
	for ch := range h.subs {
		select {
		case ch <- ev:
		default:
		}
	}
	h.mu.Unlock()
}

func (h *Hub) subscribe() chan SSEEvent {
	ch := make(chan SSEEvent, 16)
	h.mu.Lock()
	h.subs[ch] = true
	h.mu.Unlock()
	return ch
}

// since returns history entries newer than id (all when id <= 0).
func (h *Hub) since(id int64) []SSEEvent {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []SSEEvent
	for _, ev := range h.history {
		if ev.ID > id {
			out = append(out, ev)
		}
	}
	return out
}

// lastEventID parses Last-Event-ID (header or query) for resume.
func lastEventID(r *http.Request) int64 {
	s := r.Header.Get("Last-Event-ID")
	if s == "" {
		s = r.URL.Query().Get("lastEventId")
	}
	var id int64
	fmt.Sscanf(s, "%d", &id)
	return id
}

func (h *Hub) unsubscribe(ch chan SSEEvent) {
	h.mu.Lock()
	delete(h.subs, ch)
	close(ch)
	h.mu.Unlock()
}

// Stream serves GET /api/stream as text/event-stream.
func (s *Server) Stream(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	ch := s.Hub.subscribe()
	defer s.Hub.unsubscribe(ch)
	// Best-effort replay: resume after Last-Event-ID so reconnects do not
	// lose the last 50 events (matches the history comment on Publish).
	for _, ev := range s.Hub.since(lastEventID(r)) {
		fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", ev.ID, ev.Channel, ev.Data)
	}
	fl.Flush()
	tick := time.NewTicker(15 * time.Second)
	defer tick.Stop()
	fmt.Fprintf(w, ": connected\n\n")
	fl.Flush()
	for {
		select {
		case <-r.Context().Done():
			return
		case ev := <-ch:
			fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", ev.ID, ev.Channel, ev.Data)
			fl.Flush()
		case <-tick.C:
			fmt.Fprintf(w, ": heartbeat\n\n")
			fl.Flush()
		}
	}
}
