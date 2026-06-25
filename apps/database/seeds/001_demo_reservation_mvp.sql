BEGIN;

INSERT INTO inventory.event_streams (stream_key, event_id, section_id, seat_id, current_version)
VALUES
    ('seat:evt_demo_rock:floor:A-1', 'evt_demo_rock', 'floor', 'A-1', 0),
    ('seat:evt_demo_rock:floor:A-2', 'evt_demo_rock', 'floor', 'A-2', 0),
    ('seat:evt_demo_rock:floor:A-3', 'evt_demo_rock', 'floor', 'A-3', 0),
    ('seat:evt_demo_rock:balcony:B-1', 'evt_demo_rock', 'balcony', 'B-1', 0),
    ('seat:evt_demo_rock:balcony:B-2', 'evt_demo_rock', 'balcony', 'B-2', 0)
ON CONFLICT (stream_key) DO NOTHING;

INSERT INTO projection.seat_snapshots (
    event_id,
    section_id,
    seat_id,
    status,
    aggregate_version
)
VALUES
    ('evt_demo_rock', 'floor', 'A-1', 'AVAILABLE', 0),
    ('evt_demo_rock', 'floor', 'A-2', 'AVAILABLE', 0),
    ('evt_demo_rock', 'floor', 'A-3', 'AVAILABLE', 0),
    ('evt_demo_rock', 'balcony', 'B-1', 'AVAILABLE', 0),
    ('evt_demo_rock', 'balcony', 'B-2', 'AVAILABLE', 0)
ON CONFLICT (event_id, section_id, seat_id) DO NOTHING;

COMMIT;
