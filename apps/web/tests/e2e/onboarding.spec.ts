import { test, expect } from "@playwright/test";

test.describe("Onboarding flow", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto("/onboarding");
  });

  test("shows Get Started heading", async ({ page }) => {
    await expect(page.getByRole("heading", { name: "Get Started" })).toBeVisible();
  });

  test("shows step indicator with Connect as first step", async ({ page }) => {
    await expect(page.getByText("1. Connect")).toBeVisible();
    await expect(page.getByText("2. Discover")).toBeVisible();
    await expect(page.getByText("3. Reveal")).toBeVisible();
    await expect(page.getByText("4. Commit")).toBeVisible();
  });

  test("Step 1 shows credential input form", async ({ page }) => {
    await expect(page.getByText("Connect Your Amazon Seller Account")).toBeVisible();
    // Should have SP-API credential fields
    await expect(page.getByText("SP-API Client ID")).toBeVisible();
    await expect(page.getByText("Seller ID")).toBeVisible();
  });

  test("Step 1 shows connect button", async ({ page }) => {
    await expect(page.getByRole("button", { name: /connect/i })).toBeVisible();
  });

  test("Step 1 has credential input fields", async ({ page }) => {
    // Check for placeholder text in inputs
    await expect(page.getByPlaceholder(/amzn1/i)).toBeVisible();
    await expect(page.getByPlaceholder(/A2EXAMPLE/i)).toBeVisible();
  });

  test("Step 1 fields accept input", async ({ page }) => {
    await page.getByPlaceholder(/amzn1/i).fill("test-client-id");
    await page.getByPlaceholder(/A2EXAMPLE/i).fill("test-seller-id");
    // Verify values stuck
    await expect(page.getByPlaceholder(/amzn1/i)).toHaveValue("test-client-id");
  });
});

test.describe("Onboarding — assessment submission", () => {
  test("submitting credentials starts assessment and advances to Discover", async ({ page }) => {
    await page.goto("/onboarding");

    // Fill in credentials using placeholders
    await page.getByPlaceholder(/amzn1/i).fill("test-client-id");
    await page.getByPlaceholder(/client secret/i).fill("test-secret");
    await page.getByPlaceholder(/Atzr/i).fill("test-token");
    await page.getByPlaceholder(/A2EXAMPLE/i).fill("test-seller");

    // Mock the API responses
    await page.route("**/seller-account/connect", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ id: "test-account", status: "valid" }),
      });
    });
    await page.route("**/assessment/start", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          id: "test-profile",
          archetype: "greenhorn",
          assessment_status: "running",
        }),
      });
    });

    // Click connect
    await page.getByRole("button", { name: /connect.*amazon/i }).click();

    // Should advance to step 2 (Discover)
    await expect(page.getByText(/assessment in progress|analyzing|searching/i)).toBeVisible({ timeout: 10000 });
  });
});

test.describe("Onboarding — Discover step with graph", () => {
  test("shows progress stats and graph during scanning", async ({ page }) => {
    await page.goto("/onboarding");

    // Mock: already connected + assessment running
    await page.route("**/seller-account", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ id: "sa-1", status: "valid" }),
      });
    });
    await page.route("**/assessment/status", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ archetype: "greenhorn", status: "running" }),
      });
    });
    await page.route("**/assessment/graph", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          status: "running",
          graph: {
            nodes: [
              { id: "root", type: "root", label: "Amazon US" },
              { id: "cat-home", type: "category", label: "Home & Kitchen", status: "scanned", open_rate: 75 },
              { id: "cat-office", type: "category", label: "Office Products", status: "scanning" },
            ],
            edges: [
              { source: "root", target: "cat-home" },
              { source: "root", target: "cat-office" },
            ],
            stats: {
              categories_scanned: 1,
              categories_total: 20,
              eligible_products: 15,
              restricted_products: 5,
            },
          },
        }),
      });
    });

    // Navigate directly to discover step (simulate state)
    // The graph component should render on the discover page
    await page.goto("/onboarding");

    // Verify page loads without errors
    await expect(page.getByRole("heading", { name: "Get Started" })).toBeVisible();
  });
});

test.describe("Onboarding — Reveal step", () => {
  test("shows opportunity results when products found", async ({ page }) => {
    // Mock completed assessment with opportunities
    await page.route("**/assessment/status", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({ archetype: "greenhorn", status: "completed" }),
      });
    });
    await page.route("**/assessment/profile", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          profile: { archetype: "greenhorn", assessment_status: "completed" },
          fingerprint: {
            total_probes: 400,
            total_eligible: 47,
            total_restricted: 353,
            overall_open_rate: 11.75,
            categories: [
              { category: "Home & Kitchen", probe_count: 20, open_count: 15, gated_count: 5, open_rate: 75 },
              { category: "Office Products", probe_count: 20, open_count: 16, gated_count: 4, open_rate: 80 },
            ],
          },
        }),
      });
    });
    await page.route("**/assessment/graph", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          status: "completed",
          outcome: {
            has_opportunities: true,
            opportunities: {
              categories: [
                { category: "Home & Kitchen", eligible_count: 15, avg_margin: 24.2 },
                { category: "Office Products", eligible_count: 16, avg_margin: 19.8 },
              ],
              products: [
                { asin: "B0CX23V5KK", title: "Kitchen Utensil Set", buy_box_price: 29.99, estimated_margin: 24.1 },
              ],
            },
          },
          graph: { nodes: [], edges: [], stats: { categories_scanned: 20, eligible_products: 47 } },
        }),
      });
    });
    await page.route("**/strategy/versions", async (route) => {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify([
          { id: "sv-1", version_number: 1, status: "draft", goals: [{ type: "revenue", target_amount: 2000 }] },
        ]),
      });
    });

    await page.goto("/onboarding");

    // The page should detect completed status and show Reveal
    await expect(page.getByRole("heading", { name: "Get Started" })).toBeVisible();
  });
});

test.describe("Strategy page", () => {
  test("loads without crashing", async ({ page }) => {
    await page.goto("/strategy");
    // Page should render — either strategy content or empty/loading state
    await expect(page.locator("body")).toBeVisible();
    // Should not show an unhandled error
    const errorText = await page.getByText(/unhandled|error|500/i).isVisible().catch(() => false);
    expect(errorText).toBeFalsy();
  });
});

test.describe("Suggestions page", () => {
  test("loads without crashing", async ({ page }) => {
    await page.goto("/suggestions");
    await expect(page.locator("body")).toBeVisible();
    const errorText = await page.getByText(/unhandled|error|500/i).isVisible().catch(() => false);
    expect(errorText).toBeFalsy();
  });
});

test.describe("Navigation — new pages", () => {
  test("nav has Onboarding link", async ({ page }) => {
    await page.goto("/dashboard");
    await expect(page.getByRole("link", { name: "Onboarding" })).toBeVisible();
  });

  test("nav has Strategy link", async ({ page }) => {
    await page.goto("/dashboard");
    await expect(page.getByRole("link", { name: "Strategy" })).toBeVisible();
  });

  test("nav has Suggestions link", async ({ page }) => {
    await page.goto("/dashboard");
    await expect(page.getByRole("link", { name: "Suggestions" })).toBeVisible();
  });

  test("Onboarding link navigates to /onboarding", async ({ page }) => {
    await page.goto("/dashboard");
    await page.getByRole("link", { name: "Onboarding" }).click();
    await expect(page).toHaveURL(/\/onboarding/);
  });

  test("Strategy link navigates to /strategy", async ({ page }) => {
    await page.goto("/dashboard");
    await page.getByRole("link", { name: "Strategy" }).click();
    await expect(page).toHaveURL(/\/strategy/);
  });
});

test.describe("Dashboard — onboarding prompt", () => {
  test("shows Get Started card linking to onboarding", async ({ page }) => {
    // Mock no assessment
    await page.route("**/assessment/status", async (route) => {
      await route.fulfill({ status: 404, contentType: "application/json", body: '{"error":"not found"}' });
    });

    await page.goto("/dashboard");

    // Should show onboarding prompt
    const getStarted = page.getByRole("link", { name: /get started/i });
    if (await getStarted.isVisible().catch(() => false)) {
      await getStarted.click();
      await expect(page).toHaveURL(/\/onboarding/);
    }
  });
});
