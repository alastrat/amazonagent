import { test, expect } from "@playwright/test";

const navItems = [
  { label: "Dashboard", path: "/dashboard", heading: "Dashboard" },
  { label: "Campaigns", path: "/campaigns", heading: "Campaigns" },
  { label: "Deals", path: "/deals", heading: "Deal Explorer" },
  { label: "Discovery", path: "/discovery", heading: "Discovery" },
  { label: "Audit", path: "/audit", heading: "Audit Trail" },
  { label: "Settings", path: "/settings", heading: "Settings" },
];

test.describe("Sidebar navigation", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto("/dashboard");
    await expect(page.getByRole("heading", { name: "Dashboard" })).toBeVisible();
  });

  for (const item of navItems) {
    test(`navigates to ${item.label} page`, async ({ page }) => {
      // Click the nav link in the sidebar
      await page.getByRole("link", { name: item.label, exact: true }).click();

      // Verify URL
      await page.waitForURL(`**${item.path}`);

      // Verify page heading
      await expect(
        page.getByRole("heading", { name: item.heading })
      ).toBeVisible();
    });
  }

  test("sidebar shows FBA Orchestrator branding", async ({ page }) => {
    await expect(
      page.getByRole("heading", { name: "FBA Orchestrator" })
    ).toBeVisible();
  });

  test("active nav item has primary styling", async ({ page }) => {
    // On /dashboard, the Dashboard nav link should have the active class
    const dashboardLink = page.getByRole("link", { name: "Dashboard", exact: true });
    await expect(dashboardLink).toHaveClass(/bg-primary/);

    // Other nav links should NOT have the active class
    const campaignsLink = page.getByRole("link", { name: "Campaigns", exact: true });
    await expect(campaignsLink).not.toHaveClass(/bg-primary/);
  });
});
