-- Escalight initial schema. SQLite, WAL mode assumed (set by db.Open).

CREATE TABLE users (
    id            TEXT PRIMARY KEY,
    email         TEXT NOT NULL UNIQUE,
    name          TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    is_admin      INTEGER NOT NULL DEFAULT 0,
    slack_user_id TEXT NOT NULL DEFAULT '',
    created_at    TEXT NOT NULL
);

CREATE TABLE sessions (
    token      TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL
);
CREATE INDEX idx_sessions_user ON sessions(user_id);

CREATE TABLE push_subscriptions (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    endpoint   TEXT NOT NULL,
    p256dh     TEXT NOT NULL,
    auth       TEXT NOT NULL,
    created_at TEXT NOT NULL,
    UNIQUE(user_id, endpoint)
);

CREATE TABLE escalation_policies (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    repeat      INTEGER NOT NULL DEFAULT 0, -- how many times to repeat the whole policy after the last step (0 = stop)
    created_at  TEXT NOT NULL
);

CREATE TABLE escalation_steps (
    id          TEXT PRIMARY KEY,
    policy_id   TEXT NOT NULL REFERENCES escalation_policies(id) ON DELETE CASCADE,
    step_order  INTEGER NOT NULL,
    wait_minutes INTEGER NOT NULL DEFAULT 5,
    UNIQUE(policy_id, step_order)
);

-- A step can target a specific user or a schedule (resolved to "whoever is on call" at fire time).
CREATE TABLE escalation_step_targets (
    id           TEXT PRIMARY KEY,
    step_id      TEXT NOT NULL REFERENCES escalation_steps(id) ON DELETE CASCADE,
    target_type  TEXT NOT NULL CHECK (target_type IN ('user', 'schedule')),
    target_id    TEXT NOT NULL,
    via_email    INTEGER NOT NULL DEFAULT 1,
    via_push     INTEGER NOT NULL DEFAULT 1,
    via_slack    INTEGER NOT NULL DEFAULT 0,
    via_discord  INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE schedules (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    timezone   TEXT NOT NULL DEFAULT 'UTC',
    created_at TEXT NOT NULL
);

-- One active rotation per schedule for v1 (a schedule has at most one rotation definition).
CREATE TABLE schedule_rotations (
    id            TEXT PRIMARY KEY,
    schedule_id   TEXT NOT NULL UNIQUE REFERENCES schedules(id) ON DELETE CASCADE,
    rotation_type TEXT NOT NULL CHECK (rotation_type IN ('daily', 'weekly')),
    handoff_time  TEXT NOT NULL DEFAULT '09:00', -- HH:MM in schedule timezone
    start_at      TEXT NOT NULL,                 -- RFC3339 instant the rotation begins (first user's shift start)
    user_order    TEXT NOT NULL                  -- JSON array of user IDs, rotation cycles through in order
);

CREATE TABLE schedule_overrides (
    id          TEXT PRIMARY KEY,
    schedule_id TEXT NOT NULL REFERENCES schedules(id) ON DELETE CASCADE,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    start_at    TEXT NOT NULL, -- RFC3339
    end_at      TEXT NOT NULL, -- RFC3339
    created_at  TEXT NOT NULL
);
CREATE INDEX idx_overrides_schedule ON schedule_overrides(schedule_id, start_at, end_at);

CREATE TABLE services (
    id                    TEXT PRIMARY KEY,
    name                  TEXT NOT NULL,
    escalation_policy_id  TEXT NOT NULL REFERENCES escalation_policies(id) ON DELETE RESTRICT,
    webhook_key           TEXT NOT NULL UNIQUE, -- opaque key embedded in the generic/alertmanager webhook URL
    created_at            TEXT NOT NULL
);

CREATE TABLE incidents (
    id               TEXT PRIMARY KEY,
    service_id       TEXT NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    title            TEXT NOT NULL,
    description      TEXT NOT NULL DEFAULT '',
    source           TEXT NOT NULL DEFAULT 'generic', -- generic | alertmanager | email
    fingerprint      TEXT NOT NULL DEFAULT '',         -- used to dedupe re-fired alerts into the same open incident
    status           TEXT NOT NULL DEFAULT 'triggered' CHECK (status IN ('triggered', 'acknowledged', 'resolved')),
    current_step     INTEGER NOT NULL DEFAULT 0,       -- index into the policy's ordered steps
    repeat_count     INTEGER NOT NULL DEFAULT 0,
    next_escalation_at TEXT,                            -- RFC3339, NULL once acked/resolved or no further steps
    acknowledged_by  TEXT REFERENCES users(id),
    acknowledged_at  TEXT,
    resolved_by      TEXT REFERENCES users(id),
    resolved_at      TEXT,
    created_at       TEXT NOT NULL
);
CREATE INDEX idx_incidents_service ON incidents(service_id);
CREATE INDEX idx_incidents_status ON incidents(status);
CREATE INDEX idx_incidents_fingerprint ON incidents(service_id, fingerprint) WHERE status != 'resolved';
CREATE INDEX idx_incidents_next_escalation ON incidents(next_escalation_at) WHERE status = 'triggered';

CREATE TABLE incident_events (
    id          TEXT PRIMARY KEY,
    incident_id TEXT NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    event_type  TEXT NOT NULL, -- created | notified | acknowledged | escalated | resolved | notify_failed
    actor       TEXT NOT NULL DEFAULT '', -- user name/email, or "system"
    detail      TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL
);
CREATE INDEX idx_events_incident ON incident_events(incident_id, created_at);
