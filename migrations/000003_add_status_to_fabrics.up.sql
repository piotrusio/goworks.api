-- Add the new status column, defaulting to 'ACTIVE' for all existing records.
ALTER TABLE fabrics ADD COLUMN status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE';

-- Remove the old unique constraint that applied to all rows.
ALTER TABLE fabrics DROP CONSTRAINT code;

-- Create a new unique index that ONLY applies to rows where the status is 'ACTIVE'.
CREATE UNIQUE INDEX fabrics_active_code_idx ON fabrics (code) WHERE (status = 'ACTIVE');