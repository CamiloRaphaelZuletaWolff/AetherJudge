"use client";

// useContestEvents wires a contest room to the backend's live event stream.
//
// Semantics (mirroring ADR-0008): the WebSocket is delta-only; REST is the
// source of truth. Events write into the TanStack Query cache; on every
// (re)connect the room's queries are invalidated so a missed event can cost
// at most freshness, never correctness.
import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";

import { ensureFreshToken, wsURL } from "@/lib/api";
import { queryKeys } from "@/lib/query-keys";
import {
  eventEnvelopeSchema,
  leaderboardUpdateSchema,
  submissionUpdateSchema,
  type Leaderboard,
} from "@/lib/schemas";

const maxBackoffMs = 15_000;

export function useContestEvents(contestId: string): { connected: boolean } {
  const queryClient = useQueryClient();
  const [connected, setConnected] = useState(false);
  // The latest submission update is surfaced through the cache; components
  // subscribe to query keys, not to this hook, so re-renders stay local.
  const attemptRef = useRef(0);

  useEffect(() => {
    let stopped = false;
    let socket: WebSocket | null = null;
    let retryTimer: ReturnType<typeof setTimeout> | null = null;

    const handleMessage = (data: string) => {
      let envelope;
      try {
        envelope = eventEnvelopeSchema.parse(JSON.parse(data));
      } catch {
        // A malformed event must never crash the room.
        console.warn("dropping malformed contest event");
        return;
      }

      switch (envelope.type) {
        case "leaderboard.update": {
          const parsed = leaderboardUpdateSchema.safeParse(envelope.payload);
          if (parsed.success) {
            queryClient.setQueryData<Leaderboard>(queryKeys.leaderboard(contestId), {
              entries: parsed.data.entries,
            });
          }
          break;
        }
        case "submission.update": {
          const parsed = submissionUpdateSchema.safeParse(envelope.payload);
          if (parsed.success) {
            // Cheap and correct: refetch the user's submission list; the
            // event volume in a room is low.
            void queryClient.invalidateQueries({
              queryKey: queryKeys.mySubmissions(contestId),
            });
          }
          break;
        }
        default:
          // contest.event and future types: nothing to update yet.
          break;
      }
    };

    const connect = async () => {
      if (stopped) return;

      // Access tokens expire in 15 minutes; every (re)connect fetches a
      // fresh one so long sessions survive.
      const token = await ensureFreshToken();
      if (stopped || !token) return;

      socket = new WebSocket(
        wsURL(`/api/v1/ws/contests/${contestId}?access_token=${encodeURIComponent(token)}`),
      );

      socket.onopen = () => {
        attemptRef.current = 0;
        setConnected(true);
        // Re-snapshot: anything missed while disconnected is fetched, not
        // reconstructed from events.
        void queryClient.invalidateQueries({ queryKey: queryKeys.leaderboard(contestId) });
        void queryClient.invalidateQueries({ queryKey: queryKeys.mySubmissions(contestId) });
      };

      socket.onmessage = (ev) => handleMessage(String(ev.data));

      socket.onclose = () => {
        setConnected(false);
        if (stopped) return;
        const backoff = Math.min(1000 * 2 ** attemptRef.current, maxBackoffMs);
        attemptRef.current += 1;
        retryTimer = setTimeout(() => void connect(), backoff);
      };
    };

    void connect();

    return () => {
      stopped = true;
      if (retryTimer) clearTimeout(retryTimer);
      socket?.close();
    };
  }, [contestId, queryClient]);

  return { connected };
}
