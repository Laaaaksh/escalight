-- Small key/value store for server-generated config that must survive
-- restarts (e.g. VAPID web push keys) but isn't worth a dedicated table.
CREATE TABLE settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
