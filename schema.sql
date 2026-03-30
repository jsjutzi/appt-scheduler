-- Create appointments table to store all scheduled coaching appointments
CREATE TABLE IF NOT EXISTS appointments (
    id          SERIAL PRIMARY KEY,
    trainer_id  INTEGER NOT NULL,
    user_id     INTEGER NOT NULL,
    started_at   TIMESTAMPTZ NOT NULL,
    ended_at     TIMESTAMPTZ NOT NULL,
    
    -- Prevent duplicate slots for the same trainer
    CONSTRAINT unique_trainer_slot UNIQUE (trainer_id, started_at)
);

-- Index for fast lookups by trainer and time
CREATE INDEX IF NOT EXISTS idx_appointments_trainer_starts 
    ON appointments (trainer_id, started_at);

-- Index for time range queries
CREATE INDEX IF NOT EXISTS idx_appointments_starts_ends 
    ON appointments (started_at, ended_at);

-- Add helpful comments
COMMENT ON TABLE appointments IS 'Stores all scheduled coaching appointments with business rules enforced at application level';
COMMENT ON COLUMN appointments.started_at IS 'Start time in Pacific Time (America/Los_Angeles)';
COMMENT ON COLUMN appointments.ended_at IS 'End time in Pacific Time (America/Los_Angeles)';

-- Add a check constraint to ensure ended_at > started_at
ALTER TABLE appointments 
ADD CONSTRAINT check_ends_after_starts 
CHECK (ended_at > started_at);