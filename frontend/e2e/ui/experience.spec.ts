import { expect, test, type Page, type WebSocketRoute } from "@playwright/test";

const contestId = "11111111-1111-4111-8111-111111111111";
const user = {
  id: "22222222-2222-4222-8222-222222222222",
  username: "carol",
  email: "carol@example.com",
  role: "admin",
  created_at: "2026-01-01T00:00:00Z",
};
const entries = [
  { rank: 1, username: "alice", solved: 2, penalty_s: 1000 },
  { rank: 2, username: "bob", solved: 1, penalty_s: 1200 },
  { rank: 3, username: "carol", solved: 1, penalty_s: 1500 },
];
const promoted = [
  { rank: 1, username: "carol", solved: 3, penalty_s: 1800 },
  { ...entries[0]!, rank: 2 },
  { ...entries[1]!, rank: 3 },
];

async function mockArena(page: Page, phase: "active" | "past" = "active") {
  let socket: WebSocketRoute | undefined;
  let liveEntries = entries;
  const contest = {
    id: contestId,
    slug: "arena-open",
    title: "Arena Open · September",
    description: "A fresh set of algorithmic challenges. Find your next breakthrough.",
    starts_at: new Date(Date.now() - 3600000).toISOString(),
    ends_at: new Date(Date.now() + (phase === "past" ? -1000 : 7200000)).toISOString(),
  };
  await page.routeWebSocket("**/api/v1/ws/**", (ws) => {
    socket = ws;
  });
  await page.route("**/api/v1/**", async (route) => {
    const url = new URL(route.request().url());
    const path = url.pathname;
    let body: unknown = {};
    if (path.endsWith("/auth/refresh")) body = { access_token: "test-token" };
    else if (path.endsWith("/me")) body = user;
    else if (path.endsWith("/admin/users"))
      body = {
        users: [
          user,
          {
            ...user,
            id: "33333333-3333-4333-8333-333333333333",
            username: "alice",
            email: "alice@example.com",
            role: "user",
          },
        ],
      };
    else if (path.endsWith("/contests"))
      body = {
        contests: ["active", "all", null].includes(url.searchParams.get("filter")) ? [contest] : [],
      };
    else if (path.endsWith("/leaderboard")) body = { entries: liveEntries };
    else if (path.endsWith("/submissions")) body = { submissions: [] };
    else if (/\/problems\/\d+$/.test(path))
      body = {
        ord: 1,
        title: "Sum of Two Numbers",
        statement_md:
          "## The challenge\nRead two integers and print their sum.\n\n### Input\nTwo space-separated integers.\n\n### Example\n\n```text\n1 2\n```\n\n### Output\n\n```text\n3\n```",
        time_limit_ms: 1000,
        memory_limit_mb: 256,
      };
    else if (path.endsWith(contestId))
      body = {
        contest,
        problems: [
          { ord: 1, title: "Sum of Two Numbers" },
          { ord: 2, title: "Shortest Path" },
          { ord: 3, title: "Range Queries" },
          { ord: 4, title: "A Perfect Match" },
        ],
      };
    await route.fulfill({ json: body });
  });
  return {
    promote: () => {
      liveEntries = promoted;
      socket?.send(JSON.stringify({ type: "leaderboard.update", payload: { entries: promoted } }));
    },
  };
}

test("landing demonstrates a real row climb and honors reduced motion", async ({ page }) => {
  await page.route("**/api/v1/auth/refresh", (route) => route.fulfill({ status: 401, json: {} }));
  await page.goto("/");
  const rows = page.locator("[data-competitor]");
  await expect(rows.first()).toHaveAttribute("data-competitor", "byte_force");
  // Sample actual browser geometry: data attributes alone cannot prove a visible climb.
  const movement = page.evaluate(async () => {
    const element = document.querySelector('[data-competitor="hello_world"]')!;
    const samples: number[] = [];
    await new Promise<void>((resolve) => {
      document.querySelector('[data-testid="scoreboard-demo-play"]')!.addEventListener(
        "click",
        () => {
          const until = performance.now() + 2000;
          const sample = () => {
            samples.push(element.getBoundingClientRect().top);
            if (performance.now() < until) requestAnimationFrame(sample);
            else resolve();
          };
          sample();
        },
        { once: true },
      );
    });
    return samples;
  });
  await page.getByRole("button", { name: "See an ICPC-style climb" }).click();
  await expect(page.locator('[data-competitor="hello_world"]')).toHaveAttribute(
    "data-rising",
    "true",
  );
  await expect(rows.first()).toHaveAttribute("data-competitor", "hello_world");
  await expect(page.getByRole("status")).toContainText("climbs 2 places");
  const positions = await movement;
  expect(new Set(positions.map(Math.round)).size).toBeGreaterThan(4);
  expect(positions[0]! - positions.at(-1)!).toBeGreaterThan(100);
  await expect(page.getByRole("button", { name: "See an ICPC-style climb" })).toBeEnabled();
  await page.screenshot({ path: "test-results/ui/home-desktop.png", fullPage: true });
  await page.emulateMedia({ reducedMotion: "reduce", colorScheme: "dark" });
  await page.reload();
  await page.getByRole("button", { name: "See an ICPC-style climb" }).click();
  await expect(rows.first()).toHaveAttribute("data-competitor", "hello_world");
  expect(await rows.first().evaluate((element) => getComputedStyle(element).transform)).toBe(
    "none",
  );
  await page.screenshot({ path: "test-results/ui/home-dark.png", fullPage: true });
});

test("live room pauses, resumes and expands standings without discarding the editor", async ({
  page,
}) => {
  const api = await mockArena(page);
  await page.goto(`/room/${contestId}`);
  await expect(page.getByTestId("ws-status")).toHaveText("Live");
  await expect(page.getByTestId("submit-button")).toBeEnabled();
  await page.getByRole("button", { name: "Use basic editor", exact: true }).click();
  await page
    .getByRole("textbox", { name: "Solution code" })
    .fill("print(sum(map(int, input().split())))");
  await page.getByTestId("stdin-input").fill("1 2");
  await page.getByRole("button", { name: "Pause", exact: true }).click();
  api.promote();
  await expect(page.getByText("Standings paused · new updates waiting")).toBeVisible();
  await expect(page.locator("[data-competitor]").first()).toHaveAttribute(
    "data-competitor",
    "alice",
  );
  await page.getByRole("button", { name: "Resume", exact: true }).click();
  await expect(page.locator('[data-competitor="carol"]')).toHaveAttribute("data-rising", "true");
  await expect(page.locator("[data-competitor]").first()).toHaveAttribute(
    "data-competitor",
    "carol",
  );
  await page.getByRole("tab", { name: "Full scoreboard" }).click();
  await expect(page.getByText("Every position matters.")).toBeVisible();
  await page.getByRole("button", { name: "Back to solving" }).click();
  await expect(page.getByTestId("stdin-input")).toHaveValue("1 2");
  await expect(page.getByRole("textbox", { name: "Solution code" })).toHaveValue(
    "print(sum(map(int, input().split())))",
  );
  await page.getByRole("textbox", { name: "Find a competitor" }).fill("no-such-name");
  await expect(page.getByText("No matching competitors")).toBeVisible();
  await page.getByRole("textbox", { name: "Find a competitor" }).fill("");
  await page.evaluate(() => window.scrollTo(0, 0));
  await page.screenshot({ path: "test-results/ui/room-desktop.png", fullPage: true });
  await page.setViewportSize({ width: 390, height: 844 });
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.getByRole("tab", { name: "Full scoreboard" }).click();
  await expect(page.getByTestId("leaderboard")).toBeVisible();
  await page.screenshot({ path: "test-results/ui/room-mobile.png", fullPage: true });
});

test("dashboard search, keyboard tabs, account menu, and profile work on mobile", async ({
  page,
}) => {
  await mockArena(page);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/dashboard");
  await expect(page.getByTestId("contest-card")).toHaveCount(1);
  await page.getByRole("textbox", { name: "Search contests" }).fill("missing");
  await expect(page.getByTestId("contest-card")).toHaveCount(0);
  await page.getByRole("button", { name: "Clear search" }).click();
  const tab = page.getByRole("tab", { name: "All contests" });
  await tab.focus();
  await page.keyboard.press("ArrowRight");
  await expect(page.getByRole("tab", { name: "Live", exact: true })).toBeFocused();
  await expect(page.getByTestId("contest-card")).toHaveCount(1);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  await page.screenshot({ path: "test-results/ui/dashboard-mobile.png", fullPage: true });
  await page.getByRole("button", { name: "Account menu for carol" }).focus();
  await page.keyboard.press("ArrowDown");
  await expect(page.getByRole("menuitem", { name: "Dashboard" })).toBeFocused();
  await page.keyboard.press("ArrowDown");
  await page.keyboard.press("Enter");
  await expect(page).toHaveURL(/\/profile$/);
  await page.getByRole("tab", { name: "Settings" }).click();
  await expect(page.getByLabel("Display name")).toBeVisible();
  await page.getByLabel("Display name").fill("Carol Codes");
  await page.getByRole("button", { name: "Save changes" }).click();
  await expect(page.getByRole("heading", { name: "Carol Codes" })).toBeVisible();
});

test("finished contests explain disabled submissions and admin search stays usable", async ({
  page,
}) => {
  await mockArena(page, "past");
  await page.goto(`/room/${contestId}`);
  await expect(page.getByTestId("submit-button")).toBeDisabled();
  await expect(page.getByTestId("run-button")).toBeEnabled();
  await expect(
    page.getByText("Submissions closed. You can still edit and run code."),
  ).toBeVisible();
  await page.goto("/admin");
  await page.getByRole("textbox", { name: "Search members" }).fill("alice");
  await expect(page.getByTestId("user-row")).toHaveCount(1);
  await expect(page.getByTestId("user-row")).toContainText("alice");
});

test("auth validates fields and can show the password", async ({ page }) => {
  await page.route("**/api/v1/auth/refresh", (route) => route.fulfill({ status: 401, json: {} }));
  await page.goto("/auth");
  await page.getByRole("tab", { name: "Sign up" }).click();
  await page.getByTestId("signup-submit").click();
  await expect(page.getByLabel("Username", { exact: true })).toHaveAttribute(
    "aria-invalid",
    "true",
  );
  await page.getByLabel("Password", { exact: true }).fill("example-password");
  await page.getByRole("button", { name: "Show password" }).click();
  await expect(page.getByLabel("Password", { exact: true })).toHaveAttribute("type", "text");
  await page.screenshot({ path: "test-results/ui/auth-desktop.png", fullPage: true });
});
