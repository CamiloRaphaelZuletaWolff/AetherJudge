// Command seed populates the local database with a demo contest, three
// problems with hidden test cases, and two demo accounts — everything needed
// to exercise the full contest flow by hand. Idempotent: safe to re-run.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/caezu/arena/backend/pkg/logging"

	"github.com/caezu/arena/backend/services/api-gateway/internal/auth"
	"github.com/caezu/arena/backend/services/api-gateway/internal/config"
	"github.com/caezu/arena/backend/services/api-gateway/internal/db"
)

// demoPassword is for the two local demo accounts only; the seeder refuses
// to run in production mode.
const demoPassword = "password123"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "seed:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if cfg.IsProduction() {
		return errors.New("refusing to seed demo data in production mode")
	}

	log, err := logging.New(os.Stdout, cfg.LogLevel, "text")
	if err != nil {
		return fmt.Errorf("init logger: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := db.Migrate(ctx, cfg.DatabaseURL, log); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	store := db.New(pool)

	for _, username := range []string{"alice", "bob"} {
		hash, err := auth.HashPassword(demoPassword)
		if err != nil {
			return fmt.Errorf("hash demo password: %w", err)
		}
		_, err = store.CreateUser(ctx, username, username+"@example.com", hash)
		switch {
		case errors.Is(err, db.ErrUsernameTaken), errors.Is(err, db.ErrEmailTaken):
			log.Info("demo user already exists", "username", username)
		case err != nil:
			return fmt.Errorf("create demo user %s: %w", username, err)
		default:
			log.Info("created demo user", "username", username, "password", demoPassword)
		}
	}

	now := time.Now()
	contest, err := store.CreateContest(ctx,
		"demo-contest",
		"Arena Demo Contest",
		"Three classic warm-up problems. Open for 7 days — plenty of time to try every language.",
		now.Add(-30*time.Minute),
		now.Add(7*24*time.Hour),
	)
	if err != nil {
		return fmt.Errorf("create demo contest: %w", err)
	}
	log.Info("seeded contest", "slug", contest.Slug, "id", contest.ID.String())

	type seedCase struct{ in, out string }
	problems := []struct {
		ord       int
		title     string
		statement string
		cases     []seedCase
	}{
		{
			ord:   1,
			title: "Sum of Two Numbers",
			statement: "Read two integers `a` and `b` (separated by a space) from standard input " +
				"and print their sum.\n\n" +
				"**Input**\n\n`a b` where -10^9 <= a, b <= 10^9\n\n" +
				"**Output**\n\nA single integer: `a + b`.\n\n" +
				"**Example**\n\nInput: `1 2` → Output: `3`",
			cases: []seedCase{
				{"1 2", "3"},
				{"10 -4", "6"},
				{"1000000000 1000000000", "2000000000"},
			},
		},
		{
			ord:   2,
			title: "Double It",
			statement: "Read one integer `n` from standard input and print `2 * n`.\n\n" +
				"**Input**\n\n`n` where -10^9 <= n <= 10^9\n\n" +
				"**Output**\n\nA single integer: `2 * n`.\n\n" +
				"**Example**\n\nInput: `21` → Output: `42`",
			cases: []seedCase{
				{"21", "42"},
				{"0", "0"},
				{"-7", "-14"},
			},
		},
		{
			ord:   3,
			title: "Count the Vowels",
			statement: "Read one line of text and print how many vowels (`a e i o u`, " +
				"case-insensitive) it contains.\n\n" +
				"**Input**\n\nA single line of at most 1000 characters.\n\n" +
				"**Output**\n\nA single integer: the vowel count.\n\n" +
				"**Example**\n\nInput: `hello` → Output: `2`",
			cases: []seedCase{
				{"hello", "2"},
				{"xyz", "0"},
				{"AEIOU aeiou", "10"},
			},
		},
	}

	for _, p := range problems {
		problem, err := store.CreateProblem(ctx, db.Problem{
			ContestID:     contest.ID,
			Ord:           p.ord,
			Title:         p.title,
			StatementMD:   p.statement,
			TimeLimitMs:   2000,
			MemoryLimitMB: 128,
		})
		if err != nil {
			return fmt.Errorf("create problem %d: %w", p.ord, err)
		}
		for i, c := range p.cases {
			if err := store.CreateTestCase(ctx, db.TestCase{
				ProblemID:      problem.ID,
				Ord:            i + 1,
				Stdin:          c.in,
				ExpectedOutput: c.out,
			}); err != nil {
				return fmt.Errorf("create test case %d for problem %d: %w", i+1, p.ord, err)
			}
		}
		log.Info("seeded problem", "ord", p.ord, "title", p.title, "cases", len(p.cases))
	}

	log.Info("seed complete",
		"contest_id", contest.ID.String(),
		"login", "alice / bob",
		"password", demoPassword,
	)
	return nil
}
