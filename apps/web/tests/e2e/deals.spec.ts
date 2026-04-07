import { test, expect } from "@playwright/test";

test.describe("Deals page", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto("/deals");
    await expect(page.getByRole("heading", { name: "Deal Explorer" })).toBeVisible();
  });

  test("shows deals count in description", async ({ page }) => {
    // The description shows "X deals found"
    await expect(page.getByText(/\d+ deals found/)).toBeVisible();
  });

  test("shows search input and status filter", async ({ page }) => {
    await expect(
      page.getByPlaceholder("Search by title, brand, or ASIN...")
    ).toBeVisible();

    // Status filter select
    const statusSelect = page.locator("select");
    await expect(statusSelect).toBeVisible();
    await expect(statusSelect).toHaveValue("");
  });

  test("status filter has correct options", async ({ page }) => {
    const statusSelect = page.locator("select");
    await expect(statusSelect.locator("option")).toHaveCount(4);
    await expect(statusSelect.locator("option", { hasText: "All statuses" })).toBeAttached();
    await expect(statusSelect.locator("option", { hasText: "Needs Review" })).toBeAttached();
    await expect(statusSelect.locator("option", { hasText: "Approved" })).toBeAttached();
    await expect(statusSelect.locator("option", { hasText: "Rejected" })).toBeAttached();
  });

  test("shows deals table or empty state", async ({ page }) => {
    // Wait for loading to complete
    await page.waitForSelector("text=Loading...", { state: "hidden", timeout: 10_000 }).catch(() => {});

    const hasTable = await page.getByRole("columnheader", { name: "ASIN" }).isVisible().catch(() => false);
    const hasEmpty = await page.getByText("No deals found").isVisible().catch(() => false);

    expect(hasTable || hasEmpty).toBeTruthy();
  });

  test("deals table has correct columns when deals exist", async ({ page }) => {
    await page.waitForSelector("text=Loading...", { state: "hidden", timeout: 10_000 }).catch(() => {});

    const hasTable = await page.getByRole("columnheader", { name: "ASIN" }).isVisible().catch(() => false);

    if (hasTable) {
      await expect(page.getByRole("columnheader", { name: "ASIN" })).toBeVisible();
      await expect(page.getByRole("columnheader", { name: "Title" })).toBeVisible();
      await expect(page.getByRole("columnheader", { name: "Brand" })).toBeVisible();
      await expect(page.getByRole("columnheader", { name: "Score" })).toBeVisible();
      await expect(page.getByRole("columnheader", { name: "Margin" })).toBeVisible();
      await expect(page.getByRole("columnheader", { name: "Status" })).toBeVisible();
    }
  });
});
