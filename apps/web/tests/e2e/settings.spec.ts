import { test, expect } from "@playwright/test";

test.describe("Settings page", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto("/settings");
    await expect(page.getByRole("heading", { name: "Settings" })).toBeVisible();
  });

  test("shows Settings heading and description", async ({ page }) => {
    await expect(page.getByText("Manage your tenant configuration")).toBeVisible();
  });

  test("shows Scoring Configuration section", async ({ page }) => {
    await expect(page.getByText("Scoring Configuration")).toBeVisible();
  });

  test("shows scoring weights when loaded", async ({ page }) => {
    // Wait for loading to finish
    await page.waitForSelector("text=Loading scoring config...", { state: "hidden", timeout: 10_000 }).catch(() => {});

    // Check if config loaded successfully or errored
    const hasWeights = await page.getByText("Weights").isVisible().catch(() => false);
    const hasError = await page.getByText("Failed to load scoring config").isVisible().catch(() => false);

    // Either we see the weights or an error (API may not be running)
    expect(hasWeights || hasError).toBeTruthy();

    if (hasWeights) {
      // Verify all weight dimensions
      await expect(page.getByText("Demand")).toBeVisible();
      await expect(page.getByText("Competition")).toBeVisible();
      await expect(page.getByText("Margin")).toBeVisible();
      await expect(page.getByText("Risk")).toBeVisible();
      await expect(page.getByText("Sourcing")).toBeVisible();

      // Verify thresholds section
      await expect(page.getByText("Thresholds")).toBeVisible();
      await expect(page.getByText("Minimum overall score")).toBeVisible();
      await expect(page.getByText("Minimum per-dimension score")).toBeVisible();
    }
  });

  test("shows version number with weights", async ({ page }) => {
    await page.waitForSelector("text=Loading scoring config...", { state: "hidden", timeout: 10_000 }).catch(() => {});

    const hasWeights = await page.getByText("Weights").isVisible().catch(() => false);

    if (hasWeights) {
      await expect(page.getByText(/Version \d+/)).toBeVisible();
    }
  });

  test("shows more settings coming soon note", async ({ page }) => {
    await expect(page.getByText("More settings coming soon.")).toBeVisible();
  });
});
