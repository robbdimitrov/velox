CREATE TABLE IF NOT EXISTS catalog.event_announcements (
    id UUID PRIMARY KEY,
    event_id TEXT NOT NULL REFERENCES catalog.events(id),
    organizer_id TEXT NOT NULL,
    title TEXT NOT NULL,
    body TEXT NOT NULL,
    severity TEXT NOT NULL DEFAULT 'INFO' CHECK (severity IN ('INFO', 'SCHEDULE_CHANGE', 'CANCELLATION')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_event_announcements_event_created
    ON catalog.event_announcements (event_id, created_at DESC);
