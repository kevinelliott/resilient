package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

type Event struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

type EventBus struct {
	clients map[chan Event]bool
	mu      sync.RWMutex
}

func NewEventBus() *EventBus {
	return &EventBus{
		clients: make(map[chan Event]bool),
	}
}

func (eb *EventBus) Subscribe() chan Event {
	ch := make(chan Event, 64)
	eb.mu.Lock()
	eb.clients[ch] = true
	eb.mu.Unlock()
	return ch
}

func (eb *EventBus) Unsubscribe(ch chan Event) {
	eb.mu.Lock()
	delete(eb.clients, ch)
	close(ch)
	eb.mu.Unlock()
}

func (eb *EventBus) Publish(eventType string, payload interface{}) {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	ev := Event{
		Type:    eventType,
		Payload: payload,
	}

	for ch := range eb.clients {
		select {
		case ch <- ev:
		default:
			// If the channel is full and cannot receive, skip it.
			// This prevents slow clients from blocking the entire event loop.
		}
	}
}

// handleEvents is a Server-Sent Events (SSE) HTTP handler.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	setCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Make sure the writer supports flushing.
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := s.eventBus.Subscribe()
	defer s.eventBus.Unsubscribe(ch)

	// Send an initial connected event
	fmt.Fprintf(w, "event: connected\ndata: {}\n\n")
	flusher.Flush()

	// Listen to the client disconnecting
	ctx := r.Context()

	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-ch:
			data, err := json.Marshal(ev.Payload)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Type, string(data))
			flusher.Flush()
		}
	}
}
