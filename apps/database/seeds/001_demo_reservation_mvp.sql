BEGIN;

WITH generated_seats AS (
    SELECT
        'evt_neon_riot'::text AS event_id,
        section_id,
        row_label || '-' || lpad(seat_number::text, 2, '0') AS seat_id,
        8500 + seat_number * 150 AS price_amount_minor
    FROM unnest(ARRAY['A', 'B']) AS section_id
    CROSS JOIN unnest(ARRAY['A', 'B', 'C', 'D']) AS row_label
    CROSS JOIN generate_series(1, 10) AS seat_number
)
INSERT INTO inventory.event_streams (
    stream_key,
    event_id,
    section_id,
    seat_id,
    current_version
)
SELECT
    'seat:' || event_id || ':' || section_id || ':' || seat_id,
    event_id,
    section_id,
    seat_id,
    0
FROM generated_seats
ON CONFLICT (stream_key) DO NOTHING;

WITH generated_seats AS (
    SELECT
        'evt_neon_riot'::text AS event_id,
        section_id,
        row_label || '-' || lpad(seat_number::text, 2, '0') AS seat_id,
        8500 + seat_number * 150 AS price_amount_minor
    FROM unnest(ARRAY['A', 'B']) AS section_id
    CROSS JOIN unnest(ARRAY['A', 'B', 'C', 'D']) AS row_label
    CROSS JOIN generate_series(1, 10) AS seat_number
)
INSERT INTO projection.seat_snapshots (
    event_id,
    section_id,
    seat_id,
    status,
    aggregate_version,
    price_amount_minor
)
SELECT
    event_id,
    section_id,
    seat_id,
    'AVAILABLE',
    0,
    price_amount_minor
FROM generated_seats
ON CONFLICT (event_id, section_id, seat_id) DO UPDATE
SET price_amount_minor = EXCLUDED.price_amount_minor;

COMMIT;
