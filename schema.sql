-- Create appointments table to store all scheduled coaching appointments
CREATE TABLE IF NOT EXISTS appointments (
    id          SERIAL PRIMARY KEY,
    trainer_id  INTEGER NOT NULL,
    user_id     INTEGER NOT NULL,
    starts_at   TIMESTAMPTZ NOT NULL,
    ends_at     TIMESTAMPTZ NOT NULL,
    
    -- Prevent duplicate slots for the same trainer
    CONSTRAINT unique_trainer_slot UNIQUE (trainer_id, starts_at)
);

-- Index for fast lookups by trainer and time
CREATE INDEX IF NOT EXISTS idx_appointments_trainer_starts 
    ON appointments (trainer_id, starts_at);

-- Index for time range queries
CREATE INDEX IF NOT EXISTS idx_appointments_starts_ends 
    ON appointments (starts_at, ends_at);

-- Add helpful comments
COMMENT ON TABLE appointments IS 'Stores all scheduled coaching appointments with business rules enforced at application level';
COMMENT ON COLUMN appointments.starts_at IS 'Start time in Pacific Time (America/Los_Angeles)';
COMMENT ON COLUMN appointments.ends_at IS 'End time in Pacific Time (America/Los_Angeles)';

-- Add a check constraint to ensure ends_at > starts_at
ALTER TABLE appointments 
ADD CONSTRAINT check_ends_after_starts 
CHECK (ends_at > starts_at);