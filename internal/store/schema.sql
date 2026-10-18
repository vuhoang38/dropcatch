CREATE TABLE IF NOT EXISTS settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS products (
    id                 INTEGER PRIMARY KEY,
    name               TEXT NOT NULL,
    paused             INTEGER NOT NULL DEFAULT 0,
    drop_pct           REAL,
    window_days        INTEGER,
    min_history_days   INTEGER,
    last_alerted_cents INTEGER,
    created_at         INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS links (
    id              INTEGER PRIMARY KEY,
    product_id      INTEGER NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    url             TEXT NOT NULL,
    site            TEXT NOT NULL,
    title           TEXT NOT NULL DEFAULT '',
    seller          TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'pending',
    last_error      TEXT NOT NULL DEFAULT '',
    last_checked_at INTEGER,
    UNIQUE (product_id, url)
);

CREATE TABLE IF NOT EXISTS variants (
    id          INTEGER PRIMARY KEY,
    link_id     INTEGER NOT NULL REFERENCES links(id) ON DELETE CASCADE,
    external_id TEXT NOT NULL,
    name        TEXT NOT NULL DEFAULT '',
    tracked     INTEGER NOT NULL DEFAULT 0,
    is_new      INTEGER NOT NULL DEFAULT 0,
    gone        INTEGER NOT NULL DEFAULT 0,
    UNIQUE (link_id, external_id)
);

CREATE TABLE IF NOT EXISTS readings (
    id               INTEGER PRIMARY KEY,
    variant_id       INTEGER NOT NULL REFERENCES variants(id) ON DELETE CASCADE,
    at               INTEGER NOT NULL,
    price_cents      INTEGER NOT NULL,
    best_cents       INTEGER NOT NULL,
    original_cents   INTEGER,
    best_is_estimate INTEGER NOT NULL DEFAULT 0,
    in_stock         INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS readings_variant_at ON readings (variant_id, at);

CREATE TABLE IF NOT EXISTS check_log (
    id      INTEGER PRIMARY KEY,
    link_id INTEGER NOT NULL REFERENCES links(id) ON DELETE CASCADE,
    at      INTEGER NOT NULL,
    ok      INTEGER NOT NULL,
    error   TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS sessions (
    site       TEXT PRIMARY KEY,
    status     TEXT NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS notices (
    key     TEXT PRIMARY KEY,
    active  INTEGER NOT NULL,
    sent_at INTEGER NOT NULL
);
