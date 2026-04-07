import { test, expect } from "@playwright/test";

test.describe("Discovery page", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto("/discovery");
    await expect(page.getByRole("heading", { name: "Discovery" })).toBeVisible();
  });

  test("shows page heading and description", async ({ page }) => {
    await expect(
      page.getByText("Configure continuous background product sourcing")
    ).toBeVisible();
  });

  test("shows Last Run and Next Run metric cards", async ({ page }) => {
    // Wait for loading to finish
    await page.waitForSelector("text=Loading configuration...", { state: "hidden", timeout: 10_000 }).catch(() => {});

    await expect(page.getByText("Last Run")).toBeVisible();
    await expect(page.getByText("Next Run")).toBeVisible();
    await expect(page.getByText("Most recent discovery cycle completion")).toBeVisible();
    await expect(page.getByText("Scheduled start of next discovery cycle")).toBeVisible();
  });

  test("shows Discovery Settings form", async ({ page }) => {
    await page.waitForSelector("text=Loading configuration...", { state: "hidden", timeout: 10_000 }).catch(() => {});

    await expect(page.getByText("Discovery Settings")).toBeVisible();
  });

  test("shows enabled toggle", async ({ page }) => {
    await page.waitForSelector("text=Loading configuration...", { state: "hidden", timeout: 10_000 }).catch(() => {});

    // Toggle shows either "Discovery enabled" or "Discovery disabled"
    const enabledText = await page.getByText("Discovery enabled").isVisible().catch(() => false);
    const disabledText = await page.getByText("Discovery disabled").isVisible().catch(() => false);

    expect(enabledText || disabledText).toBeTruthy();
  });

  test("shows categories input", async ({ page }) => {
    await page.waitForSelector("text=Loading configuration...", { state: "hidden", timeout: 10_000 }).catch(() => {});

    await expect(page.getByText("Categories (comma-separated)")).toBeVisible();
    await expect(
      page.getByPlaceholder("kitchen, home fitness, pet supplies")
    ).toBeVisible();
  });

  test("shows cadence selector", async ({ page }) => {
    await page.waitForSelector("text=Loading configuration...", { state: "hidden", timeout: 10_000 }).catch(() => {});

    await expect(page.getByText("Run Cadence")).toBeVisible();

    // Cadence select has Nightly, Twice Daily, Weekly options
    const cadenceSelect = page.locator("select").filter({ hasText: "Nightly" });
    await expect(cadenceSelect).toBeVisible();
  });

  test("shows marketplace and min margin fields", async ({ page }) => {
    await page.waitForSelector("text=Loading configuration...", { state: "hidden", timeout: 10_000 }).catch(() => {});

    await expect(page.getByText("Marketplace")).toBeVisible();
    await expect(page.getByText("Minimum Margin %")).toBeVisible();
  });

  test("shows Save Settings button", async ({ page }) => {
    await page.waitForSelector("text=Loading configuration...", { state: "hidden", timeout: 10_000 }).catch(() => {});

    await expect(
      page.getByRole("button", { name: "Save Settings" })
    ).toBeVisible();
  });
});
