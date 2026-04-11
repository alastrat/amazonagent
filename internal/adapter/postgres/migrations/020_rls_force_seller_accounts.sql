-- Ensure RLS cannot be bypassed by table owner
ALTER TABLE amazon_seller_accounts FORCE ROW LEVEL SECURITY;
