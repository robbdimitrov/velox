-- jsonb reorders keys on storage, breaking order.events.v1 signatures; text
-- round-trips the exact signed bytes.
ALTER TABLE orders.outbox_events
    ALTER COLUMN payload TYPE text USING payload::text;
