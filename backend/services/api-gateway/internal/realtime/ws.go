package realtime

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
)

// pingInterval keeps intermediaries from idling out quiet connections.
const pingInterval = 30 * time.Second

// writeTimeout bounds a single frame write to a client.
const writeTimeout = 5 * time.Second

// ServeWS upgrades the request and pumps room events to the client until it
// disconnects, the hub drops it, or the server shuts down. originPatterns
// follows websocket.AcceptOptions semantics (host patterns).
func ServeWS(w http.ResponseWriter, r *http.Request, hub *Hub, contestID uuid.UUID, originPatterns []string, log *slog.Logger) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: originPatterns})
	if err != nil {
		// Accept has already written the HTTP error response.
		log.Debug("websocket accept", "error", err)
		return
	}
	defer func() {
		// Best-effort close; the peer is usually already gone by now.
		if err := conn.CloseNow(); err != nil {
			log.Debug("websocket close", "error", err)
		}
	}()

	client, err := hub.Join(contestID)
	if err != nil {
		log.Error("join room", "contest_id", contestID, "error", err)
		if closeErr := conn.Close(websocket.StatusInternalError, "room unavailable"); closeErr != nil {
			log.Debug("close after join failure", "error", closeErr)
		}
		return
	}
	defer hub.Leave(contestID, client)

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Reader: the client never sends application data, but reading is how
	// close frames and pings are processed — and how we learn it left.
	go func() {
		defer cancel()
		for {
			if _, _, err := conn.Read(ctx); err != nil {
				return
			}
		}
	}()

	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-client.Receive():
			if !ok {
				// Hub dropped us (slow consumer or room teardown).
				if closeErr := conn.Close(websocket.StatusTryAgainLater, "dropped by hub"); closeErr != nil {
					log.Debug("close dropped client", "error", closeErr)
				}
				return
			}
			if err := writeFrame(ctx, conn, msg); err != nil {
				return
			}
		case <-ticker.C:
			pingCtx, pingCancel := context.WithTimeout(ctx, writeTimeout)
			err := conn.Ping(pingCtx)
			pingCancel()
			if err != nil {
				return
			}
		}
	}
}

func writeFrame(ctx context.Context, conn *websocket.Conn, msg []byte) error {
	writeCtx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	return conn.Write(writeCtx, websocket.MessageText, msg)
}
