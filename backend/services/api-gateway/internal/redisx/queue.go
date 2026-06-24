package redisx

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// The durable judge queue (ADR-0011). A submission is a PostgreSQL row
// (status='queued') — the source of truth — and this Stream is only the
// dispatch mechanism: losing it loses no data (the judge reconciles pending
// rows from PG on startup). Consumer groups give at-least-once delivery,
// per-message acks, and crash recovery via claim of a dead consumer's
// un-acked messages.
const (
	// JudgeStream is the Redis Stream key carrying submission work items.
	JudgeStream = "arena:judgeq"
	// JudgeGroup is the consumer group every judge worker joins.
	JudgeGroup = "judges"

	// judgeMaxLen is a generous safety cap (approximate trim) so a runaway
	// producer cannot grow the stream without bound. It is NOT backpressure:
	// real backpressure is a depth check before XADD (trimming would drop
	// un-acked work, i.e. lose submissions). At steady state the stream is
	// near-empty because workers ack promptly.
	judgeMaxLen = 100_000

	fieldSubmissionID = "submission_id"
	fieldTraceParent  = "traceparent"
)

// QueueItem is one work item read from the judge stream.
type QueueItem struct {
	// MessageID is the stream entry ID, needed to Ack the item.
	MessageID string
	// SubmissionID is the submission to judge.
	SubmissionID uuid.UUID
	// TraceParent is the W3C traceparent of the enqueuing request, used to
	// link the judge span back across the async hop (empty for reconciled
	// or untraced items).
	TraceParent string
}

// EnsureJudgeGroup creates the consumer group (and the stream) if absent.
// Idempotent: an existing group is not an error. The group starts at "0" so
// a worker that comes up after submissions were already enqueued still sees
// them.
func (c *Client) EnsureJudgeGroup(ctx context.Context) error {
	err := c.rdb.XGroupCreateMkStream(ctx, JudgeStream, JudgeGroup, "0").Err()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		return fmt.Errorf("redisx: create judge group: %w", err)
	}
	return nil
}

// EnqueueJudge appends a submission to the stream. The stream is created on
// first add; the approximate MAXLEN is a safety cap only.
func (c *Client) EnqueueJudge(ctx context.Context, submissionID uuid.UUID, traceParent string) error {
	if err := c.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: JudgeStream,
		MaxLen: judgeMaxLen,
		Approx: true,
		Values: map[string]any{
			fieldSubmissionID: submissionID.String(),
			fieldTraceParent:  traceParent,
		},
	}).Err(); err != nil {
		return fmt.Errorf("redisx: enqueue judge %s: %w", submissionID, err)
	}
	return nil
}

// JudgeQueueDepth returns the number of entries in the stream — ready plus
// in-flight (un-acked). Used for the queue-depth metric and for admission
// backpressure.
func (c *Client) JudgeQueueDepth(ctx context.Context) (int64, error) {
	n, err := c.rdb.XLen(ctx, JudgeStream).Result()
	if err != nil {
		return 0, fmt.Errorf("redisx: judge queue depth: %w", err)
	}
	return n, nil
}

// ReadJudge blocks up to block for new (never-delivered) messages for this
// consumer. A nil slice with nil error means the block elapsed with nothing
// new. Cancelling ctx unblocks it.
func (c *Client) ReadJudge(ctx context.Context, consumer string, count int64, block time.Duration) ([]QueueItem, error) {
	streams, err := c.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    JudgeGroup,
		Consumer: consumer,
		Streams:  []string{JudgeStream, ">"},
		Count:    count,
		Block:    block,
	}).Result()
	if errors.Is(err, redis.Nil) || errors.Is(err, context.DeadlineExceeded) {
		return nil, nil // block elapsed with nothing new
	}
	if err != nil {
		if ctx.Err() != nil {
			return nil, nil // shutting down
		}
		return nil, fmt.Errorf("redisx: read judge group: %w", err)
	}
	return itemsFromStreams(streams), nil
}

// ClaimStaleJudge recovers messages whose consumer has not acked them within
// minIdle (typically because it crashed). Messages whose delivery count
// exceeds maxDeliveries are returned separately as poison: the caller must
// dead-letter them (mark the submission failed and Ack) rather than retry
// forever. Reclaimed (non-poison) messages are transferred to consumer and
// returned for processing.
func (c *Client) ClaimStaleJudge(ctx context.Context, consumer string, minIdle time.Duration, count, maxDeliveries int64) (claimed, poison []QueueItem, err error) {
	pending, err := c.rdb.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: JudgeStream,
		Group:  JudgeGroup,
		Idle:   minIdle,
		Start:  "-",
		End:    "+",
		Count:  count,
	}).Result()
	if err != nil {
		return nil, nil, fmt.Errorf("redisx: scan pending judge: %w", err)
	}
	if len(pending) == 0 {
		return nil, nil, nil
	}

	var claimIDs, poisonIDs []string
	for _, p := range pending {
		if p.RetryCount > maxDeliveries {
			poisonIDs = append(poisonIDs, p.ID)
		} else {
			claimIDs = append(claimIDs, p.ID)
		}
	}

	if len(poisonIDs) > 0 {
		msgs, claimErr := c.rdb.XClaim(ctx, &redis.XClaimArgs{
			Stream: JudgeStream, Group: JudgeGroup, Consumer: consumer,
			MinIdle: minIdle, Messages: poisonIDs,
		}).Result()
		if claimErr != nil {
			return nil, nil, fmt.Errorf("redisx: claim poison judge: %w", claimErr)
		}
		poison = itemsFromMessages(msgs)
	}
	if len(claimIDs) > 0 {
		msgs, claimErr := c.rdb.XClaim(ctx, &redis.XClaimArgs{
			Stream: JudgeStream, Group: JudgeGroup, Consumer: consumer,
			MinIdle: minIdle, Messages: claimIDs,
		}).Result()
		if claimErr != nil {
			return nil, nil, fmt.Errorf("redisx: claim stale judge: %w", claimErr)
		}
		claimed = itemsFromMessages(msgs)
	}
	return claimed, poison, nil
}

// AckJudge acknowledges a processed message and deletes it from the stream
// (the submission's durable record lives in PostgreSQL, so the stream entry
// has no further use once acked).
func (c *Client) AckJudge(ctx context.Context, messageID string) error {
	if err := c.rdb.XAck(ctx, JudgeStream, JudgeGroup, messageID).Err(); err != nil {
		return fmt.Errorf("redisx: ack judge %s: %w", messageID, err)
	}
	if err := c.rdb.XDel(ctx, JudgeStream, messageID).Err(); err != nil {
		return fmt.Errorf("redisx: del judge %s: %w", messageID, err)
	}
	return nil
}

func itemsFromStreams(streams []redis.XStream) []QueueItem {
	var out []QueueItem
	for _, s := range streams {
		out = append(out, itemsFromMessages(s.Messages)...)
	}
	return out
}

func itemsFromMessages(msgs []redis.XMessage) []QueueItem {
	out := make([]QueueItem, 0, len(msgs))
	for _, m := range msgs {
		idStr, _ := m.Values[fieldSubmissionID].(string)
		id, err := uuid.Parse(idStr)
		if err != nil {
			// Unparseable entry: surface it as a zero-ID item so the caller
			// can ack-and-drop it rather than block the group forever.
			out = append(out, QueueItem{MessageID: m.ID})
			continue
		}
		tp, _ := m.Values[fieldTraceParent].(string)
		out = append(out, QueueItem{MessageID: m.ID, SubmissionID: id, TraceParent: tp})
	}
	return out
}
