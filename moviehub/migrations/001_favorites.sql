-- For databases created before favorites was added to schema.sql.
BEGIN;

CREATE TABLE IF NOT EXISTS favorites (
    member_id BIGINT NOT NULL REFERENCES members (id) ON DELETE CASCADE,
    movie_id  BIGINT NOT NULL REFERENCES movies (id) ON DELETE CASCADE,
    added_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (member_id, movie_id)
);

CREATE INDEX IF NOT EXISTS idx_favorites_member_added ON favorites (member_id, added_at DESC);

COMMIT;
