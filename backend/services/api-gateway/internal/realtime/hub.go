// Package realtime fans contest events out to WebSocket clients. Rooms are
// keyed by contest; each room holds one Redis subscription shared by all its
// local clients, so the design scales horizontally: every gateway replica
// subscribes once per room and serves its own connections.
package realtime

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/google/uuid"

	"github.com/caezu/arena/backend/services/api-gateway/internal/redisx"
)

// Subscriber provides the per-room event stream; redisx.Client satisfies it.
type Subscriber interface {
	Subscribe(ctx context.Context, channel string) (<-chan []byte, error)
}

// sendBuffer bounds the per-client queue. A client that cannot drain this
// many messages is disconnected rather than allowed to back-pressure the
// room (it can reconnect and re-snapshot via REST).
const sendBuffer = 32

// Client is one WebSocket connection's view of a room.
type Client struct {
	send chan []byte
}

// Receive returns the channel of outbound messages. It is closed when the
// hub drops the client (slow consumer or room shutdown).
func (c *Client) Receive() <-chan []byte { return c.send }

type room struct {
	clients map[*Client]struct{}
	cancel  context.CancelFunc
}

// Hub tracks rooms and their Redis subscriptions.
type Hub struct {
	sub Subscriber
	log *slog.Logger

	mu      sync.Mutex
	baseCtx context.Context
	rooms   map[uuid.UUID]*room
}

// NewHub builds a hub. baseCtx bounds every room subscription's lifetime
// (process shutdown).
func NewHub(baseCtx context.Context, sub Subscriber, log *slog.Logger) *Hub {
	return &Hub{
		sub:     sub,
		log:     log,
		baseCtx: baseCtx,
		rooms:   make(map[uuid.UUID]*room),
	}
}

// Join adds a client to a contest room, creating the room (and its Redis
// subscription) on first join.
func (h *Hub) Join(contestID uuid.UUID) (*Client, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	r, ok := h.rooms[contestID]
	if !ok {
		subCtx, cancel := context.WithCancel(h.baseCtx)
		msgs, err := h.sub.Subscribe(subCtx, redisx.ContestChannel(contestID))
		if err != nil {
			cancel()
			return nil, fmt.Errorf("realtime: subscribe room %s: %w", contestID, err)
		}

		r = &room{clients: make(map[*Client]struct{}), cancel: cancel}
		h.rooms[contestID] = r
		go h.pump(contestID, r, msgs)
	}

	c := &Client{send: make(chan []byte, sendBuffer)}
	r.clients[c] = struct{}{}
	wsActiveConnections.Inc()
	return c, nil
}

// Leave removes a client; the last client out tears the room down.
func (h *Hub) Leave(contestID uuid.UUID, c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.dropLocked(contestID, c)
}

// pump relays the room's Redis stream to every local client until the
// subscription ends (hub shutdown, room teardown, or Redis failure).
func (h *Hub) pump(contestID uuid.UUID, r *room, msgs <-chan []byte) {
	for msg := range msgs {
		h.mu.Lock()
		for c := range r.clients {
			select {
			case c.send <- msg:
			default:
				// Slow consumer: drop it rather than stall the room.
				h.log.Warn("dropping slow websocket client", "contest_id", contestID)
				h.dropLocked(contestID, c)
			}
		}
		h.mu.Unlock()
	}

	// Stream ended. If the room still has clients (Redis failure rather
	// than normal teardown), disconnect them so they reconnect cleanly.
	h.mu.Lock()
	defer h.mu.Unlock()
	if r, ok := h.rooms[contestID]; ok {
		for c := range r.clients {
			h.dropLocked(contestID, c)
		}
	}
}

// dropLocked removes one client (closing its send channel) and tears the
// room down when it empties. Callers hold h.mu. Idempotent per client.
func (h *Hub) dropLocked(contestID uuid.UUID, c *Client) {
	r, ok := h.rooms[contestID]
	if !ok {
		return
	}
	if _, member := r.clients[c]; !member {
		return
	}
	delete(r.clients, c)
	close(c.send)
	wsActiveConnections.Dec()

	if len(r.clients) == 0 {
		r.cancel()
		delete(h.rooms, contestID)
	}
}
