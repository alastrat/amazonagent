import { test, expect } from "@playwright/test";

const API_BASE = "http://localhost:8081";

/**
 * Mock all chat-related API endpoints so tests run without a backend.
 */
async function mockChatAPI(page: import("@playwright/test").Page) {
  // POST /chat/send — accept the message
  await page.route(`${API_BASE}/chat/send`, (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ status: "accepted" }),
    })
  );

  // GET /chat/history — empty conversation
  await page.route(`${API_BASE}/chat/history`, (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ messages: [] }),
    })
  );

  // GET /chat/events — SSE stream (return empty and close immediately)
  await page.route(`${API_BASE}/chat/events*`, (route) =>
    route.fulfill({
      status: 200,
      contentType: "text/event-stream",
      body: "",
    })
  );
}

test.describe("Concierge chat panel", () => {
  test.beforeEach(async ({ page }) => {
    // Clear localStorage so each test starts with a clean slate
    await page.addInitScript(() => localStorage.clear());

    await mockChatAPI(page);
    await page.goto("/dashboard");
    await expect(page.getByRole("heading", { name: "Dashboard" })).toBeVisible();
  });

  // -----------------------------------------------------------------------
  // 1. Open panel
  // -----------------------------------------------------------------------
  test("opens concierge panel when Ask Concierge clicked", async ({ page }) => {
    await page.getByRole("button", { name: "Ask Concierge" }).click();

    // Panel header is visible
    await expect(page.getByText("FBA Concierge")).toBeVisible();

    // Textarea with placeholder is visible
    await expect(
      page.getByPlaceholder("Ask your concierge...")
    ).toBeVisible();
  });

  // -----------------------------------------------------------------------
  // 2. Close panel
  // -----------------------------------------------------------------------
  test("closes concierge panel when X clicked", async ({ page }) => {
    // Open
    await page.getByRole("button", { name: "Ask Concierge" }).click();
    await expect(page.getByText("FBA Concierge")).toBeVisible();

    // Close via the x button in the panel header
    await page.getByRole("button", { name: "\u00d7" }).click();

    // Panel should be gone
    await expect(page.getByText("FBA Concierge")).not.toBeVisible();
  });

  // -----------------------------------------------------------------------
  // 3. Empty state
  // -----------------------------------------------------------------------
  test("shows empty state on first open", async ({ page }) => {
    await page.getByRole("button", { name: "Ask Concierge" }).click();

    await expect(
      page.getByText("Hi! I\u2019m your FBA Concierge.")
    ).toBeVisible();
    await expect(
      page.getByText("Ask me about your products, eligibility, categories, or strategy.")
    ).toBeVisible();
  });

  // -----------------------------------------------------------------------
  // 4. Send message and see user bubble
  // -----------------------------------------------------------------------
  test("sends message and shows user bubble", async ({ page }) => {
    await page.getByRole("button", { name: "Ask Concierge" }).click();

    const textarea = page.getByPlaceholder("Ask your concierge...");
    await textarea.fill("What categories can I sell in?");
    await textarea.press("Enter");

    // User message bubble appears
    await expect(
      page.getByText("What categories can I sell in?")
    ).toBeVisible();

    // Input was cleared
    await expect(textarea).toHaveValue("");
  });

  // -----------------------------------------------------------------------
  // 5. Send button disabled when empty
  // -----------------------------------------------------------------------
  test("Send button disabled when input empty", async ({ page }) => {
    await page.getByRole("button", { name: "Ask Concierge" }).click();

    const sendBtn = page.getByRole("button", { name: "Send" });
    const textarea = page.getByPlaceholder("Ask your concierge...");

    // Initially disabled
    await expect(sendBtn).toBeDisabled();

    // Type something — enabled
    await textarea.fill("hello");
    await expect(sendBtn).toBeEnabled();

    // Clear — disabled again
    await textarea.fill("");
    await expect(sendBtn).toBeDisabled();
  });

  // -----------------------------------------------------------------------
  // 6. Shift+Enter adds newline, Enter sends
  // -----------------------------------------------------------------------
  test("Shift+Enter adds newline, Enter sends", async ({ page }) => {
    await page.getByRole("button", { name: "Ask Concierge" }).click();

    const textarea = page.getByPlaceholder("Ask your concierge...");

    // Type first line
    await textarea.fill("line 1");

    // Shift+Enter to add newline, then type second line
    await textarea.press("Shift+Enter");
    await textarea.pressSequentially("line 2");

    // Textarea should contain both lines
    await expect(textarea).toHaveValue("line 1\nline 2");

    // Enter sends the message
    await textarea.press("Enter");

    // Message bubble appears with both lines
    await expect(page.getByText("line 1")).toBeVisible();
    await expect(page.getByText("line 2")).toBeVisible();

    // Input was cleared
    await expect(textarea).toHaveValue("");
  });

  // -----------------------------------------------------------------------
  // 7. Persist panel state in localStorage
  // -----------------------------------------------------------------------
  test("persists panel state in localStorage", async ({ page }) => {
    // Open panel
    await page.getByRole("button", { name: "Ask Concierge" }).click();
    await expect(page.getByText("FBA Concierge")).toBeVisible();

    // Reload
    await page.reload();
    await expect(page.getByRole("heading", { name: "Dashboard" })).toBeVisible();

    // Panel should still be open
    await expect(page.getByText("FBA Concierge")).toBeVisible();
  });

  // -----------------------------------------------------------------------
  // 8. Typing indicator while waiting
  // -----------------------------------------------------------------------
  test("typing indicator shows while waiting", async ({ page }) => {
    // Override send endpoint with a delayed response to keep isLoading true
    await page.route(`${API_BASE}/chat/send`, async (route) => {
      await new Promise((resolve) => setTimeout(resolve, 2000));
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ status: "accepted" }),
      });
    });

    await page.getByRole("button", { name: "Ask Concierge" }).click();

    const textarea = page.getByPlaceholder("Ask your concierge...");
    await textarea.fill("Tell me about ungated categories");
    await textarea.press("Enter");

    // Typing indicator should appear (isLoading is set to true on ADD_USER_MESSAGE)
    await expect(page.getByText("Thinking...")).toBeVisible();
  });
});
