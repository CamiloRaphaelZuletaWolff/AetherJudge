package redisx

import "testing"

func TestRankScoreOrdersBySolvedThenPenalty(t *testing.T) {
	t.Parallel()

	// A higher score sorts first under ZREVRANGE. More solved must always win,
	// and for equal solves a smaller penalty must win.
	cases := []struct {
		name       string
		aSolved    int
		aPenalty   int64
		bSolved    int
		bPenalty   int64
		wantAOverB bool
	}{
		{"more solved beats fewer despite huge penalty", 3, 9_000_000, 2, 0, true},
		{"equal solved: lower penalty wins", 2, 100, 2, 101, true},
		{"equal solved: higher penalty loses", 2, 500, 2, 100, false},
		{"one solve beats zero", 1, 1_000_000, 0, 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := rankScore(tc.aSolved, tc.aPenalty)
			b := rankScore(tc.bSolved, tc.bPenalty)
			if got := a > b; got != tc.wantAOverB {
				t.Errorf("rankScore(%d,%d)=%v vs rankScore(%d,%d)=%v: a>b=%v, want %v",
					tc.aSolved, tc.aPenalty, a, tc.bSolved, tc.bPenalty, b, got, tc.wantAOverB)
			}
		})
	}
}
