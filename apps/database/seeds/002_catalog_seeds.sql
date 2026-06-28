BEGIN;

INSERT INTO catalog.venues (id, name, city, address, capacity)
VALUES ('ven_northstar', 'Northstar Arena', 'Chicago', '123 Stadium Dr', 40)
ON CONFLICT (id) DO NOTHING;

INSERT INTO catalog.user_venues (user_id, venue_id, venue_role)
VALUES ('usr_vendor_1', 'ven_northstar', 'OWNER')
ON CONFLICT (user_id, venue_id) DO NOTHING;

DO $$
DECLARE
    r text;
    n int;
    sid text;
BEGIN
    FOR s IN 1..2 LOOP
        IF s = 1 THEN sid := 'A'; ELSE sid := 'B'; END IF;
        FOREACH r IN ARRAY ARRAY['A', 'B'] LOOP
            FOR n IN 1..10 LOOP
                INSERT INTO catalog.venue_seats (venue_id, section_id, seat_id)
                VALUES ('ven_northstar', sid, r || '-' || n)
                ON CONFLICT (venue_id, section_id, seat_id) DO NOTHING;
            END LOOP;
        END LOOP;
    END LOOP;
END $$;

COMMIT;
