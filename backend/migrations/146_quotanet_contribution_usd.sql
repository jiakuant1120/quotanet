-- Track QuotaNet contribution in Sub2API's USD billing unit first.
-- Chain token conversion/payout rules are intentionally kept as a later layer.

ALTER TABLE quotanet_contribution_ledger
    ADD COLUMN IF NOT EXISTS standard_cost_usd DECIMAL(20,10) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS actual_cost_usd DECIMAL(20,10) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS contribution_usd DECIMAL(20,10) NOT NULL DEFAULT 0;

ALTER TABLE quotanet_payout_batches
    ADD COLUMN IF NOT EXISTS total_contribution_usd DECIMAL(20,10) NOT NULL DEFAULT 0;

ALTER TABLE quotanet_payout_items
    ADD COLUMN IF NOT EXISTS contribution_usd DECIMAL(20,10) NOT NULL DEFAULT 0;

UPDATE quotanet_contribution_ledger
SET contribution_usd = COALESCE(NULLIF(contribution_usd, 0), amount_cxs),
    actual_cost_usd = COALESCE(NULLIF(actual_cost_usd, 0), amount_cxs),
    standard_cost_usd = COALESCE(NULLIF(standard_cost_usd, 0), amount_cxs)
WHERE amount_cxs <> 0
  AND contribution_usd = 0
  AND actual_cost_usd = 0
  AND standard_cost_usd = 0;

UPDATE quotanet_payout_batches
SET total_contribution_usd = COALESCE(NULLIF(total_contribution_usd, 0), total_amount_cxs)
WHERE total_amount_cxs <> 0
  AND total_contribution_usd = 0;

UPDATE quotanet_payout_items
SET contribution_usd = COALESCE(NULLIF(contribution_usd, 0), amount_cxs)
WHERE amount_cxs <> 0
  AND contribution_usd = 0;
