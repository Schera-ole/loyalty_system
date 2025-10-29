-- Update accrual column from INTEGER to NUMERIC(10,2) to support decimal values
ALTER TABLE orders ALTER COLUMN accrual TYPE NUMERIC(10,2) USING accrual::NUMERIC(10,2);