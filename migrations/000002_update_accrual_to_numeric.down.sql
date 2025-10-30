-- Revert accrual column back to INTEGER
ALTER TABLE orders ALTER COLUMN accrual TYPE INTEGER USING ROUND(accrual)::INTEGER;