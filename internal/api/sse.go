package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

const (
	maxSubscribers = 32               // tabs open at the same time
	writeTimeout   = 10 * time.Second // a page that stops reading is disconnected
)

// Broadcaster sends the SSE "update" event to every open page.
type Broadcaster struct {
	mu          sync.Mutex
	subscribers map[chan UpdateEvent]struct{}
	closed      bool
}

func NewBroadcaster() *Broadcaster {
	return &Broadcaster{subscribers: map[chan UpdateEvent]struct{}{}}
}

// Announce tells subscribers that these tables changed. A subscriber that is too slow
// drops the event instead of blocking the import.
func (b *Broadcaster) Announce(tables ...string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	event := UpdateEvent{Tables: tables}
	for subscriber := range b.subscribers {
		select {
		case subscriber <- event:
		default:
		}
	}
}

// Close ends every connection (server shutdown).
func (b *Broadcaster) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true
	for subscriber := range b.subscribers {
		close(subscriber)
		delete(b.subscribers, subscriber)
	}
}

func (b *Broadcaster) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	_, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming is not supported")
		return
	}
	events := make(chan UpdateEvent, 4)
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		writeError(w, http.StatusServiceUnavailable, "shutting down")
		return
	}
	if len(b.subscribers) >= maxSubscribers {
		b.mu.Unlock()
		writeError(w, http.StatusTooManyRequests, "too many pages subscribed to updates")
		return
	}
	b.subscribers[events] = struct{}{}
	b.mu.Unlock()
	defer func() {
		b.mu.Lock()
		if _, ok := b.subscribers[events]; ok {
			delete(b.subscribers, events)
			close(events)
		}
		b.mu.Unlock()
	}()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	rc := http.NewResponseController(w)
	send := func(text string) bool {
		rc.SetWriteDeadline(time.Now().Add(writeTimeout))
		if _, err := fmt.Fprint(w, text); err != nil {
			return false
		}
		return rc.Flush() == nil
	}
	if !send("retry: 3000\n\n") {
		return
	}

	heartbeat := time.NewTicker(25 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			if !send(": heartbeat\n\n") {
				return
			}
		case event, ok := <-events:
			if !ok {
				return
			}
			body, _ := json.Marshal(event)
			if !send(fmt.Sprintf("event: update\ndata: %s\n\n", body)) {
				return
			}
		}
	}
}
