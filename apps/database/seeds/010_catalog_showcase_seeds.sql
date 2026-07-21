BEGIN;

INSERT INTO catalog.venue_sections (
    venue_id,
    section_id,
    name,
    display_order,
    width,
    height
)
SELECT
    venue_id,
    section_id,
    section_id || ' Section',
    display_order,
    464,
    204
FROM (
    VALUES
        ('ven_velox_arena', 'A', 1),
        ('ven_velox_arena', 'B', 2),
        ('ven_north_pier', 'A', 1),
        ('ven_north_pier', 'B', 2),
        ('ven_civic_bowl', 'A', 1),
        ('ven_civic_bowl', 'B', 2),
        ('ven_moonlight', 'A', 1),
        ('ven_moonlight', 'B', 2)
) AS sections(venue_id, section_id, display_order)
ON CONFLICT (venue_id, section_id) DO UPDATE
SET name = EXCLUDED.name,
    display_order = EXCLUDED.display_order,
    width = EXCLUDED.width,
    height = EXCLUDED.height;

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
    timezone = data.timezone
FROM (
    VALUES
        ('evt_neon_riot', 'Arena-scale synth and alt-pop with synchronized fan drops.', 'Concerts', 'America/Chicago'),
        ('evt_north_pier', 'A waterfront symphony program with limited reserved seating.', 'Concerts', 'America/Los_Angeles'),
        ('evt_civic_bowl', 'Championship football with live inventory across lower bowl sections.', 'Sports', 'America/Denver'),
        ('evt_summer_fests', 'Outdoor festival entry with reserved viewing sections.', 'Festivals', 'America/Chicago')
) AS data(event_id, description, category, timezone)
WHERE catalog.events.id = data.event_id;

INSERT INTO catalog.event_sections (
    event_id,
    section_id,
    name,
    display_order,
    width,
    height
)
SELECT
    e.id,
    vs.section_id,
    vs.name,
    vs.display_order,
    vs.width,
    vs.height
FROM catalog.events e
JOIN catalog.venue_sections vs ON vs.venue_id = e.venue_id
ON CONFLICT (event_id, section_id) DO UPDATE
SET name = EXCLUDED.name,
    display_order = EXCLUDED.display_order,
    width = EXCLUDED.width,
    height = EXCLUDED.height;

COMMIT;
