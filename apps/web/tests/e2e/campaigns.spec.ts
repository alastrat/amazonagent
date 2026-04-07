import { test, expect } from "@playwright/test";

test.describe("Campaigns page", () => {
  test("shows Campaigns heading", async ({ page }) => {
    await page.goto("/campaigns");
    await expect(page.getByRole("heading", { name: "Campaigns" })).toBeVisible();
    await expect(page.getByText("Research campaigns and discovery runs")).toBeVisible();
  });

  test("shows New Campaign button", async ({ page }) => {
    await page.goto("/campaigns");
    await expect(page.getByRole("link", { name: "New Campaign" })).toBeVisible();
  });

  test("shows campaign list or empty state", async ({ page }) => {
    await page.goto("/campaigns");

    // Wait for loading to finish
    await page.waitForSelector("text=Loading...", { state: "hidden", timeout: 10_000 }).catch(() => {});

    const hasTable = await page.getByRole("columnheader", { name: "Type" }).isVisible().catch(() => false);
    const hasEmpty = await page.getByText("No campaigns yet").isVisible().catch(() => false);

    expect(hasTable || hasEmpty).toBeTruthy();
  });
});

test.describe.serial("Campaign creation flow", () => {
  test("navigate to new campaign page", async ({ page }) => {
    await page.goto("/campaigns");
    await page.getByRole("link", { name: "New Campaign" }).click();
    await page.waitForURL("**/campaigns/new");
    await expect(page.getByRole("heading", { name: "New Campaign" })).toBeVisible();
  });

  test("new campaign form has all fields", async ({ page }) => {
    await page.goto("/campaigns/new");

    await expect(page.getByText("Keywords (comma-separated)")).toBeVisible();
    await expect(page.getByText("Marketplace")).toBeVisible();
    await expect(page.getByText("Minimum Margin %")).toBeVisible();
    await expect(page.getByRole("button", { name: "Create Campaign" })).toBeVisible();
  });

  test("fill in and submit campaign form", async ({ page }) => {
    await page.goto("/campaigns/new");

    // Fill in keywords
    await page.getByPlaceholder("kitchen gadgets, home fitness").fill("test product, e2e test");

    // Marketplace defaults to US - verify
    const marketplaceSelect = page.locator("select").first();
    await expect(marketplaceSelect).toHaveValue("US");

    // Fill in minimum margin
    await page.getByPlaceholder("30").fill("25");

    // Submit the form
    await page.getByRole("button", { name: "Create Campaign" }).click();

    // Should redirect back to campaigns list
    await page.waitForURL("**/campaigns", { timeout: 15_000 });
    await expect(page.getByRole("heading", { name: "Campaigns" })).toBeVisible();
  });
});
