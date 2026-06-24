// The Track-1 acceptance test: the full contest flow through the real stack
// (frontend → gateway → sandboxed executor), including a second browser
// session observing live updates.
import { expect, test, type Page } from "@playwright/test";

const PASSWORD = "password123";

function uniqueUser(prefix: string): string {
  return `${prefix}${Date.now() % 1_000_000_000}`;
}

async function signup(page: Page, username: string): Promise<void> {
  await page.goto("/auth");
  await page.getByRole("tab", { name: "Sign up" }).click();
  await page.getByLabel("Username").fill(username);
  await page.getByLabel("Email").fill(`${username}@example.com`);
  await page.getByLabel("Password").fill(PASSWORD);
  await page.getByTestId("signup-submit").click();
  await expect(page.getByTestId("current-user")).toHaveText(username);
}

async function enterDemoRoom(page: Page): Promise<void> {
  await expect(page.getByText("Arena Demo Contest")).toBeVisible();
  await page.getByTestId("enter-room").first().click();
  await expect(page.getByTestId("contest-title")).toHaveText("Arena Demo Contest");
  await expect(page.getByTestId("problem-statement")).toBeVisible();
  await expect(page.getByTestId("leaderboard")).toBeVisible();
}

async function typePythonSolution(page: Page, code: string): Promise<void> {
  await page.getByTestId("language-select").selectOption("python");
  const editor = page.locator(".monaco-editor").first();
  await editor.waitFor();
  await editor.click();
  await page.keyboard.press("Control+a");
  await page.keyboard.press("Delete");
  await page.keyboard.insertText(code);
}

function latestVerdict(page: Page) {
  return page.getByTestId("my-submissions").getByTestId("verdict-badge").first();
}

test("full contest flow with live updates for a second session", async ({ page, browser }) => {
  const solver = uniqueUser("e2e");
  await signup(page, solver);
  await enterDemoRoom(page);

  // A second, independent session watches the same room.
  const observerContext = await browser.newContext();
  const observer = await observerContext.newPage();
  await signup(observer, uniqueUser("obs"));
  await enterDemoRoom(observer);

  // Problem 1: Sum of Two Numbers.
  await typePythonSolution(page, "a, b = map(int, input().split())\nprint(a + b)");

  // Run against custom stdin first — no judging, just output.
  await page.getByTestId("stdin-input").fill("1 2");
  await page.getByTestId("run-button").click();
  await expect(page.getByTestId("run-stdout")).toContainText("3", { timeout: 90_000 });

  // Submit for real judging across the hidden test cases.
  await page.getByTestId("submit-button").click();
  await expect(page.getByTestId("submit-confirmation")).toBeVisible();
  await expect(latestVerdict(page)).toHaveAttribute("data-verdict", "accepted", {
    timeout: 120_000,
  });

  // The solver's leaderboard shows the solve...
  await expect(page.getByTestId("leaderboard")).toContainText(solver);

  // ...and the observer sees it arrive live, without any reload.
  await expect(observer.getByTestId("leaderboard")).toContainText(solver, { timeout: 30_000 });

  await observerContext.close();
});

test("wrong submission is judged wrong_answer and never scores", async ({ page }) => {
  const user = uniqueUser("wa");
  await signup(page, user);
  await enterDemoRoom(page);

  await typePythonSolution(page, "print(999)");
  await page.getByTestId("submit-button").click();

  await expect(latestVerdict(page)).toHaveAttribute("data-verdict", "wrong_answer", {
    timeout: 120_000,
  });
  await expect(page.getByTestId("leaderboard")).not.toContainText(user);
});
