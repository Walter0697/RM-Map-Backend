-- Resets and backfills normalized station line tables:
--   train_station_lines
--   train_station_station_lines
--
-- NOTE: This is intentionally destructive for the two tables above.

BEGIN;

ALTER TABLE train_station_maps
    ADD COLUMN IF NOT EXISTS map_label text;
UPDATE train_station_maps
SET map_label = map_name
WHERE map_label IS NULL OR map_label = '';
ALTER TABLE train_station_maps
    ADD COLUMN IF NOT EXISTS icon_path text;

ALTER TABLE train_stations
    DROP COLUMN IF EXISTS icon_path;

DROP TABLE IF EXISTS train_station_station_lines;
DROP TABLE IF EXISTS train_station_lines;

CREATE TABLE train_station_lines (
    id bigserial PRIMARY KEY,
    created_at timestamptz NULL,
    deleted_at timestamptz NULL,
    created_uid bigint NULL REFERENCES users(id),
    updated_at timestamptz NULL,
    updated_uid bigint NULL REFERENCES users(id),
    map_name text NOT NULL,
    name text NOT NULL,
    local_name text NOT NULL,
    colour text NOT NULL
);

CREATE UNIQUE INDEX idx_train_station_lines_unique
    ON train_station_lines (map_name, name, local_name, colour);
CREATE INDEX idx_train_station_lines_map_name
    ON train_station_lines (map_name);
CREATE INDEX idx_train_station_lines_deleted_at
    ON train_station_lines (deleted_at);

CREATE TABLE train_station_station_lines (
    id bigserial PRIMARY KEY,
    created_at timestamptz NULL,
    deleted_at timestamptz NULL,
    created_uid bigint NULL REFERENCES users(id),
    updated_at timestamptz NULL,
    updated_uid bigint NULL REFERENCES users(id),
    station_id bigint NOT NULL REFERENCES train_stations(id),
    line_id bigint NOT NULL REFERENCES train_station_lines(id),
    position integer NOT NULL DEFAULT 0
);

CREATE UNIQUE INDEX idx_train_station_station_lines_unique
    ON train_station_station_lines (station_id, line_id);
CREATE INDEX idx_train_station_station_lines_station_id
    ON train_station_station_lines (station_id);
CREATE INDEX idx_train_station_station_lines_line_id
    ON train_station_station_lines (line_id);
CREATE INDEX idx_train_station_station_lines_deleted_at
    ON train_station_station_lines (deleted_at);

-- Backfill line catalog from existing train_stations.line_info JSON.
INSERT INTO train_station_lines (map_name, name, local_name, colour, created_at, updated_at)
SELECT DISTINCT
    ts.map_name,
    COALESCE(elem->>'name', ''),
    COALESCE(elem->>'localName', ''),
    COALESCE(elem->>'colour', ''),
    NOW(),
    NOW()
FROM train_stations ts
CROSS JOIN LATERAL jsonb_array_elements(
    CASE
        WHEN ts.line_info IS NULL OR ts.line_info = '' THEN '[]'::jsonb
        ELSE ts.line_info::jsonb
    END
) elem
ON CONFLICT (map_name, name, local_name, colour) DO NOTHING;

-- Backfill station-line relations with per-station position.
INSERT INTO train_station_station_lines (station_id, line_id, position, created_at, updated_at)
SELECT
    ts.id,
    tsl.id,
    COALESCE((elem->>'position')::int, 0),
    NOW(),
    NOW()
FROM train_stations ts
CROSS JOIN LATERAL jsonb_array_elements(
    CASE
        WHEN ts.line_info IS NULL OR ts.line_info = '' THEN '[]'::jsonb
        ELSE ts.line_info::jsonb
    END
) elem
JOIN train_station_lines tsl
    ON tsl.map_name = ts.map_name
   AND tsl.name = COALESCE(elem->>'name', '')
   AND tsl.local_name = COALESCE(elem->>'localName', '')
   AND tsl.colour = COALESCE(elem->>'colour', '')
ON CONFLICT (station_id, line_id)
DO UPDATE SET
    position = EXCLUDED.position,
    updated_at = NOW();

COMMIT;
