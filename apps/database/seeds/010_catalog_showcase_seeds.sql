BEGIN;

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

UPDATE catalog.events
SET description = data.description,
    category = data.category,
    sale_starts_at = data.sale_starts_at::timestamptz,
    image_key = data.image_key,
    timezone = data.timezone
FROM (
    VALUES
        ('evt_neon_riot', 'Arena-scale synth and alt-pop with synchronized fan drops.', 'Concerts', '2026-07-19 18:00:00+00', 'event-final-whistle', 'America/Chicago'),
        ('evt_north_pier', 'A waterfront symphony program with limited reserved seating.', 'Concerts', '2026-08-01 17:00:00+00', 'event-zero-hour', 'America/Los_Angeles'),
        ('evt_civic_bowl', 'Championship football with live inventory across lower bowl sections.', 'Sports', '2026-08-20 16:00:00+00', 'event-zero-hour', 'America/Denver'),
        ('evt_summer_fests', 'Outdoor festival entry with reserved viewing sections.', 'Festivals', '2026-07-19 09:00:00+00', 'event-final-whistle', 'America/Chicago')
) AS data(event_id, description, category, sale_starts_at, image_key, timezone)
WHERE catalog.events.id = data.event_id;

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

COMMIT;
