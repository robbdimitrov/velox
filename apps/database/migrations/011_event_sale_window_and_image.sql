BEGIN;

ALTER TABLE catalog.events
    ADD COLUMN IF NOT EXISTS image_key TEXT NOT NULL DEFAULT 'event-midnight-array',
    ADD COLUMN IF NOT EXISTS sale_starts_at TIMESTAMPTZ NOT NULL DEFAULT now();

ALTER TABLE catalog.events
    ADD CONSTRAINT events_image_key_check
    CHECK (image_key IN ('event-midnight-array', 'event-final-whistle', 'event-zero-hour'));

COMMIT;
