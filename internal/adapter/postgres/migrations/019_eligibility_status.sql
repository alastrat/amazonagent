-- Add eligibility_status and approval_url to assessment_probe_results
ALTER TABLE assessment_probe_results
  ADD COLUMN IF NOT EXISTS eligibility_status TEXT NOT NULL DEFAULT 'restricted',
  ADD COLUMN IF NOT EXISTS approval_url TEXT NOT NULL DEFAULT '';

-- Backfill existing data: eligible=true → 'eligible', else 'restricted'
UPDATE assessment_probe_results SET eligibility_status = 'eligible' WHERE eligible = true;
