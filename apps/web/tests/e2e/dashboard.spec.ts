import { test, expect } from "@playwright/test";

test.describe("Dashboard page", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto("/dashboard");
    await expect(page.getByRole("heading", { name: "Dashboard" })).toBeVisible();
  });

  test("shows Dashboard heading and description", async ({ page }) => {
    await expect(page.getByText("Overview of your sourcing pipeline")).toBeVisible();
  });

  test("shows three metric cards", async ({ page }) => {
    await expect(page.getByText("Pending Review")).toBeVisible();
    await expect(page.getByText("Approved Deals")).toBeVisible();
    await expect(page.getByText("Active Campaigns")).toBeVisible();
  });

  test("metric card descriptions are visible", async ({ page }) => {
    await expect(page.getByText("Deals awaiting your decision")).toBeVisible();
    await expect(page.getByText("Ready for sourcing")).toBeVisible();
    await expect(page.getByText("Currently running")).toBeVisible();
  });

  test("shows Recent Deals section", async ({ page }) => {
    await expect(
      page.getByRole("heading", { name: "Recent Deals" })
    ).toBeVisible();
  });

  test("recent deals shows either table or empty message", async ({ page }) => {
    // Either the table headers are visible (deals exist) or the empty state message
    const hasTable = await page.getByRole("columnheader", { name: "ASIN" }).isVisible().catch(() => false);
    const hasEmpty = await page.getByText("No deals yet").isVisible().catch(() => false);

    expect(hasTable || hasEmpty).toBeTruthy();
  });
});
