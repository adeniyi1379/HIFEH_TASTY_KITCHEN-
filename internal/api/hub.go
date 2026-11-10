package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
)

// Hub broadcasts order events to connected terminals (SSE).
type Hub struct {
	mu      sync.Mutex
	clients map[chan []byte]struct{}
}

func NewHub() *Hub {
	return &Hub{clients: make(map[chan []byte]struct{})}
}

func (h *Hub) add() chan []byte {
	ch := make(chan []byte, 8)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *Hub) remove(ch chan []byte) {
	h.mu.Lock()
	delete(h.clients, ch)
	h.mu.Unlock()
}

func (h *Hub) Broadcast(event string, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		slog.Error("broadcast marshal", "err", err)
		return
	}
	msg := fmt.Appendf(nil, "event: %s\ndata: %s\n\n", event, body)
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.clients {
		select {
		case ch <- msg:
		default: // slow client: drop the frame rather than block sellers
		}
	}
}

// ServeSSE handles GET /api/events.
func (h *Hub) ServeSSE(w http.ResponseWriter, r *http.Request) {
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")

	ch := h.add()
	defer h.remove(ch)

	// Initial retry hint for EventSource.
	fmt.Fprint(w, "retry: 2000\n\n")
	fl.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case msg := <-ch:
			if _, err := w.Write(msg); err != nil {
				return
			}
			fl.Flush()
		}
	}
}
