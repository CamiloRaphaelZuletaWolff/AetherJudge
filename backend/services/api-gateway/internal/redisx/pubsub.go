package redisx

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// ContestChannel names the pub/sub channel carrying one contest's events.
// Every gateway replica subscribes per room; publishing here reaches every
// connected client across all replicas.
func ContestChannel(contestID uuid.UUID) string {
	return "contest:" + contestID.String() + ":events"
}

// Publish sends one event payload to a channel.
func (c *Client) Publish(ctx context.Context, channel string, payload []byte) error {
	if err := c.rdb.Publish(ctx, channel, payload).Err(); err != nil {
		return fmt.Errorf("redisx: publish to %s: %w", channel, err)
	}
	return nil
}

// Subscribe starts consuming a channel. Messages arrive on the returned
// channel until ctx is cancelled; it is closed on exit. Subscription errors
// after the initial handshake are logged and end the stream rather than
// panicking the hub.
func (c *Client) Subscribe(ctx context.Context, channel string) (<-chan []byte, error) {
	pubsub := c.rdb.Subscribe(ctx, channel)

	// Force the subscription handshake so failures surface here, not as a
	// silently empty stream.
	if _, err := pubsub.Receive(ctx); err != nil {
		closeErr := pubsub.Close()
		if closeErr != nil {
			c.log.Warn("close pubsub after failed subscribe", "error", closeErr)
		}
		return nil, fmt.Errorf("redisx: subscribe %s: %w", channel, err)
	}

	out := make(chan []byte, 64)
	go func() {
		defer close(out)
		defer func() {
			if err := pubsub.Close(); err != nil {
				c.log.Warn("close pubsub", "channel", channel, "error", err)
			}
		}()

		msgs := pubsub.Channel()
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-msgs:
				if !ok {
					return
				}
				select {
				case out <- []byte(msg.Payload):
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return out, nil
}
