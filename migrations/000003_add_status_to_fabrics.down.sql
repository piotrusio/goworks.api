-- Drop the partial unique index first.
DROP INDEX IF EXISTS fabrics_active_code_idx;

-- Remove the status column.
ALTER TABLE fabrics DROP COLUMN status;

-- Restore the original simple unique constraint on the code.
ALTER TABLE fabrics ADD CONSTRAINT code UNIQUE (code);