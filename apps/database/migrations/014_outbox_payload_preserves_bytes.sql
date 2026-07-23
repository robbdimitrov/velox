-- jsonb (and the driver's cached json/jsonb parameter binding) reformats and
-- reorders keys on storage, breaking the byte-identical payload that
-- order.events.v1 signatures are computed over. text guarantees the exact
-- input bytes round-trip through publish, with no JSON-aware reformatting.
ALTER TABLE orders.outbox_events
    ALTER COLUMN payload TYPE text USING payload::text;
