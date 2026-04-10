-- Seed the test tenant's seller account from env-var credentials.
-- In dev mode (no ENCRYPTION_KEY), credentials are stored in plaintext.
-- In production, the Go app encrypts before insertion via the service layer.
--
-- This migration inserts a placeholder row that the app will populate on startup
-- if the test tenant exists and has no seller account yet.
-- The actual seeding happens in Go (apps/api/main.go) because encryption
-- must be applied at the app layer, not in SQL.

-- No-op SQL migration: seeding is done in Go at startup.
-- See apps/api/main.go for the SeedTestSellerAccount call.
SELECT 1;
