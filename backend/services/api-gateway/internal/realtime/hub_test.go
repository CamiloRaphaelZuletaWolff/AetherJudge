package realtime

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// fakeSubscriber hands out controllable channels per Subscribe call.
type fakeSubscriber struct {
	mu      sync.Mutex
	streams map[string]chan []byte
	cancels map[string]bool
}

func newFakeSubscriber() *fakeSubscriber {
	return &fakeSubscriber{streams: map[string]chan []byte{}, cancels: map[string]bool{}}
}

func (f *fakeSubscriber) Subscribe(ctx context.Context, channel string) (<-chan []byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	ch := make(chan []byte, 64)
	f.streams[channel] = ch

	go func() {
		<-ctx.Done()
		f.mu.Lock()
		defer f.mu.Unlock()
		f.cancels[channel] = true
		close(ch)
	}()
	return ch, nil
}

func (f *fakeSubscriber) push(t *testing.T, channel string, msg []byte) {
	t.Helper()
	f.mu.Lock()
	ch, ok := f.streams[channel]
	f.mu.Unlock()
	if !ok {
		t.Fatalf("no subscription for %s", channel)
	}
	ch <- msg
}

func (f *fakeSubscriber) cancelled(channel string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.cancels[channel]
}

func recv(t *testing.T, c *Client) []byte {
	t.Helper()
	select {
	case msg, ok := <-c.Receive():
		if !ok {
			t.Fatal("client channel closed unexpectedly")
		}
		return msg
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for message")
		return nil
	}
}

func TestRoomBroadcastReachesAllClients(t *testing.T) {
	t.Parallel()

	sub := newFakeSubscriber()
	hub := NewHub(context.Background(), sub, discardLogger())
	contestID := uuid.New()
	channel := "contest:" + contestID.String() + ":events"

	c1, err := hub.Join(contestID)
	if err != nil {
		t.Fatalf("join c1: %v", err)
	}
	c2, err := hub.Join(contestID)
	if err != nil {
		t.Fatalf("join c2: %v", err)
	}

	sub.push(t, channel, []byte(`{"type":"x"}`))

	if got := string(recv(t, c1)); got != `{"type":"x"}` {
		t.Errorf("c1 got %q", got)
	}
	if got := string(recv(t, c2)); got != `{"type":"x"}` {
		t.Errorf("c2 got %q", got)
	}
}

func TestLastLeaveCancelsSubscription(t *testing.T) {
	t.Parallel()

	sub := newFakeSubscriber()
	hub := NewHub(context.Background(), sub, discardLogger())
	contestID := uuid.New()
	channel := "contest:" + contestID.String() + ":events"

	c1, err := hub.Join(contestID)
	if err != nil {
		t.Fatalf("join: %v", err)
	}
	c2, err := hub.Join(contestID)
	if err != nil {
		t.Fatalf("join: %v", err)
	}

	hub.Leave(contestID, c1)
	if sub.cancelled(channel) {
		t.Fatal("subscription cancelled while a client remained")
	}

	hub.Leave(contestID, c2)
	deadline := time.Now().Add(2 * time.Second)
	for !sub.cancelled(channel) {
		if time.Now().After(deadline) {
			t.Fatal("subscription not cancelled after last client left")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestSlowClientIsDropped(t *testing.T) {
	t.Parallel()

	sub := newFakeSubscriber()
	hub := NewHub(context.Background(), sub, discardLogger())
	contestID := uuid.New()
	channel := "contest:" + contestID.String() + ":events"

	slow, err := hub.Join(contestID)
	if err != nil {
		t.Fatalf("join slow: %v", err)
	}
	// A second, promptly-read client paces the test against the pump: once
	// a message arrives here, the pump has processed it for every client.
	fast, err := hub.Join(contestID)
	if err != nil {
		t.Fatalf("join fast: %v", err)
	}

	// Apply net-positive pressure on the slow client: push two messages and
	// consume at most one from it per round, so its buffer must overflow no
	// matter how the goroutines interleave (the original push-then-wait
	// version raced the pump against this loop's own draining reads).
	deadline := time.Now().Add(5 * time.Second)
	for i := 0; ; i++ {
		if time.Now().After(deadline) {
			t.Fatal("slow client was not dropped")
		}

		sub.push(t, channel, fmt.Appendf(nil, `{"n":%d}`, 2*i))
		sub.push(t, channel, fmt.Appendf(nil, `{"n":%d}`, 2*i+1))
		recv(t, fast)
		recv(t, fast)

		select {
		case _, ok := <-slow.Receive():
			if !ok {
				return // dropped, as required
			}
		default:
		}
	}
}

func TestSecondRoomIsIndependent(t *testing.T) {
	t.Parallel()

	sub := newFakeSubscriber()
	hub := NewHub(context.Background(), sub, discardLogger())
	contestA, contestB := uuid.New(), uuid.New()

	ca, err := hub.Join(contestA)
	if err != nil {
		t.Fatalf("join A: %v", err)
	}
	cb, err := hub.Join(contestB)
	if err != nil {
		t.Fatalf("join B: %v", err)
	}

	sub.push(t, "contest:"+contestA.String()+":events", []byte("a"))

	if got := string(recv(t, ca)); got != "a" {
		t.Errorf("room A client got %q", got)
	}
	select {
	case msg := <-cb.Receive():
		t.Errorf("room B client received room A's message: %q", msg)
	case <-time.After(100 * time.Millisecond):
	}
}
