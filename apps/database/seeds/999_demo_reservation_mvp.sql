BEGIN;

UPDATE catalog.events
SET description = CASE id
        WHEN 'evt_neon_riot' THEN 'Arena-scale synth and alt-pop with synchronized fan drops.'
        WHEN 'evt_north_pier' THEN 'A waterfront symphony program with limited reserved seating.'
        WHEN 'evt_civic_bowl' THEN 'Championship football with live inventory across lower bowl sections.'
        WHEN 'evt_summer_fests' THEN 'Outdoor festival entry with reserved viewing sections.'
        ELSE description
    END,
    category = CASE id
        WHEN 'evt_civic_bowl' THEN 'Sports'
        WHEN 'evt_summer_fests' THEN 'Festivals'
        ELSE 'Concerts'
    END,
    image_key = CASE id
        WHEN 'evt_neon_riot' THEN 'event-final-whistle'
        WHEN 'evt_summer_fests' THEN 'event-final-whistle'
        ELSE 'event-zero-hour'
    END,
    sale_starts_at = CASE id
        WHEN 'evt_neon_riot' THEN '2026-07-19 18:00:00+00'::timestamptz
        WHEN 'evt_north_pier' THEN '2026-08-01 17:00:00+00'::timestamptz
        WHEN 'evt_civic_bowl' THEN '2026-08-20 16:00:00+00'::timestamptz
        WHEN 'evt_summer_fests' THEN '2026-07-19 09:00:00+00'::timestamptz
        ELSE sale_starts_at
    END,
    timezone = CASE id
        WHEN 'evt_north_pier' THEN 'America/Los_Angeles'
        WHEN 'evt_civic_bowl' THEN 'America/Denver'
        ELSE 'America/Chicago'
    END
WHERE id IN ('evt_neon_riot', 'evt_north_pier', 'evt_civic_bowl', 'evt_summer_fests');

INSERT INTO catalog.venue_sections (
    venue_id,
    section_id,
    name,
    display_order,
    width,
    height,
    default_price_amount_minor
)
SELECT
    venue_id,
    section_id,
    section_id || ' Section',
    display_order,
    464,
    204,
    default_price_amount_minor
FROM (
    VALUES
        ('ven_velox_arena', 'A', 1, 8650),
        ('ven_velox_arena', 'B', 2, 8650),
        ('ven_north_pier', 'A', 1, 7450),
        ('ven_north_pier', 'B', 2, 7450),
        ('ven_civic_bowl', 'A', 1, 9250),
        ('ven_civic_bowl', 'B', 2, 9250),
        ('ven_moonlight', 'A', 1, 6800),
        ('ven_moonlight', 'B', 2, 6800)
) AS sections(venue_id, section_id, display_order, default_price_amount_minor)
ON CONFLICT (venue_id, section_id) DO UPDATE
SET name = EXCLUDED.name,
    display_order = EXCLUDED.display_order,
    width = EXCLUDED.width,
    height = EXCLUDED.height,
    default_price_amount_minor = EXCLUDED.default_price_amount_minor;

WITH generated_seats AS (
    SELECT
        venue_id,
        section_id,
        row_label,
        seat_number,
        row_label || '-' || lpad(seat_number::text, 2, '0') AS seat_id,
        44 + (seat_number - 1) * 42 AS x,
        42 + (ascii(row_label) - ascii('A')) * 42 AS y,
        seat_number IN (1, 10) AS accessibility
    FROM (VALUES
        ('ven_velox_arena', 'A'),
        ('ven_velox_arena', 'B'),
        ('ven_north_pier', 'A'),
        ('ven_north_pier', 'B'),
        ('ven_civic_bowl', 'A'),
        ('ven_civic_bowl', 'B'),
        ('ven_moonlight', 'A'),
        ('ven_moonlight', 'B')
    ) AS sections(venue_id, section_id)
    CROSS JOIN unnest(ARRAY['A', 'B', 'C', 'D']) AS row_label
    CROSS JOIN generate_series(1, 10) AS seat_number
)
INSERT INTO catalog.venue_seats (
    venue_id,
    section_id,
    seat_id,
    row_label,
    seat_number,
    x,
    y,
    accessibility
)
SELECT
    venue_id,
    section_id,
    seat_id,
    row_label,
    seat_number,
    x,
    y,
    accessibility
FROM generated_seats
ON CONFLICT (venue_id, section_id, seat_id) DO UPDATE
SET row_label = EXCLUDED.row_label,
    seat_number = EXCLUDED.seat_number,
    x = EXCLUDED.x,
    y = EXCLUDED.y,
    accessibility = EXCLUDED.accessibility;

INSERT INTO catalog.event_sections (
    event_id,
    section_id,
    name,
    display_order,
    width,
    height,
    price_amount_minor
)
SELECT
    e.id,
    vs.section_id,
    vs.name,
    vs.display_order,
    vs.width,
    vs.height,
    vs.default_price_amount_minor
FROM catalog.events e
JOIN catalog.venue_sections vs ON vs.venue_id = e.venue_id
ON CONFLICT (event_id, section_id) DO UPDATE
SET name = EXCLUDED.name,
    display_order = EXCLUDED.display_order,
    width = EXCLUDED.width,
    height = EXCLUDED.height,
    price_amount_minor = EXCLUDED.price_amount_minor;

WITH generated_seats AS (
    SELECT
        e.id AS event_id,
        vs.section_id,
        vs.seat_id
    FROM catalog.events e
    JOIN catalog.venue_seats vs ON vs.venue_id = e.venue_id
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
        e.id AS event_id,
        vs.section_id,
        vs.seat_id,
        vs.row_label,
        vs.seat_number,
        vs.x,
        vs.y,
        vs.accessibility,
        es.price_amount_minor
    FROM catalog.events e
    JOIN catalog.venue_seats vs ON vs.venue_id = e.venue_id
    JOIN catalog.event_sections es
        ON es.event_id = e.id AND es.section_id = vs.section_id
)
INSERT INTO projection.seat_snapshots (
    event_id,
    section_id,
    seat_id,
    status,
    aggregate_version,
    price_amount_minor,
    row_label,
    seat_number,
    x,
    y,
    accessibility
)
SELECT
    event_id,
    section_id,
    seat_id,
    'AVAILABLE',
    0,
    price_amount_minor,
    row_label,
    seat_number,
    x,
    y,
    accessibility
FROM generated_seats
ON CONFLICT (event_id, section_id, seat_id) DO UPDATE
SET price_amount_minor = EXCLUDED.price_amount_minor,
    row_label = EXCLUDED.row_label,
    seat_number = EXCLUDED.seat_number,
    x = EXCLUDED.x,
    y = EXCLUDED.y,
    accessibility = EXCLUDED.accessibility;

COMMIT;
