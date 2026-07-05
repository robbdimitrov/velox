ALTER TABLE orders.outbox_events
    ADD COLUMN IF NOT EXISTS last_attempt_at timestamptz;
