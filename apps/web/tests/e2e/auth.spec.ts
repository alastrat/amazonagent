import { test, expect } from "@playwright/test";

test.describe("Authentication flow", () => {
  test("login page renders correctly", async ({ page }) => {
    await page.goto("/login");

    // Page heading
    await expect(page.getByRole("heading", { name: "FBA Orchestrator" })).toBeVisible();

    // Form fields
    await expect(page.getByLabel("Email")).toBeVisible();
    await expect(page.getByLabel("Password")).toBeVisible();

    // Sign In button
    await expect(page.getByRole("button", { name: "Sign In" })).toBeVisible();

    // Sign Up toggle link
    await expect(page.getByRole("button", { name: "Sign Up" })).toBeVisible();
  });

  test("can toggle between sign in and sign up modes", async ({ page }) => {
    await page.goto("/login");

    // Initially in sign-in mode
    await expect(page.getByText("Sign in to your account")).toBeVisible();

    // Click "Sign Up" toggle
    await page.getByRole("button", { name: "Sign Up" }).click();

    // Now in sign-up mode
    await expect(page.getByText("Create your account")).toBeVisible();
    await expect(page.getByRole("button", { name: "Sign Up" }).first()).toBeVisible();
  });

  test("dev mode auto-redirects from login to dashboard", async ({ page }) => {
    // When Supabase is NOT configured, /login redirects to /dashboard automatically
    await page.goto("/login");

    // Should redirect to dashboard (dev mode with no Supabase env vars)
    await page.waitForURL("**/dashboard", { timeout: 10_000 });
    await expect(page.getByRole("heading", { name: "Dashboard" })).toBeVisible();
  });

  test("dev mode allows direct access to dashboard", async ({ page }) => {
    // In dev mode (no Supabase configured), accessing /dashboard should work directly
    await page.goto("/dashboard");
    await expect(page.getByRole("heading", { name: "Dashboard" })).toBeVisible();
  });
});
