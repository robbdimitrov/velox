-- projection.order_summaries never had event_id, but apigateway's
-- GetOrganizerMetrics (organizer dashboard) already queries it by event_id,
-- and viewservice's projector already writes it — both predate this
-- migration; the column was simply missing.
ALTER TABLE projection.order_summaries
    ADD COLUMN IF NOT EXISTS event_id text;

CREATE INDEX IF NOT EXISTS idx_projection_order_summaries_event
    ON projection.order_summaries (event_id);
