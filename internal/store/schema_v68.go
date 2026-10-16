package store

// This additive observation-cache migration is developed as one unshipped
// change. Private cache rows are not part of pulled transcript manifests.
const migrationV68 = `
CREATE TABLE collective_observation_scopes (
    village_url TEXT NOT NULL,
    user_id TEXT NOT NULL,
    incarnation TEXT NOT NULL UNIQUE,
    invocation_seq INTEGER NOT NULL DEFAULT 0 CHECK (invocation_seq >= 0),
    attempt_generation INTEGER NOT NULL DEFAULT 0 CHECK (attempt_generation >= 0),
    PRIMARY KEY (village_url, user_id)
) WITHOUT ROWID;
CREATE TABLE collective_attempt_sources (
    village_url TEXT NOT NULL,
    user_id TEXT NOT NULL,
    collective_id TEXT NOT NULL,
    transcript_id TEXT NOT NULL,
    installed_seq INTEGER NOT NULL CHECK (installed_seq > 0),
    observed_at TEXT NOT NULL,
    PRIMARY KEY (village_url, user_id, collective_id, transcript_id),
    FOREIGN KEY (village_url, user_id) REFERENCES collective_observation_scopes(village_url, user_id)
) WITHOUT ROWID;
CREATE TABLE remote_contribution_attempts (
    village_url TEXT NOT NULL,
    user_id TEXT NOT NULL,
    collective_id TEXT NOT NULL,
    transcript_id TEXT NOT NULL,
    event_num INTEGER NOT NULL CHECK (event_num BETWEEN 1 AND 2147483647),
    status TEXT NOT NULL CHECK (status IN ('pending','approved','rejected','retracted','revoked')),
    recorded_at TEXT NOT NULL,
    decided_at TEXT,
    decided_by_actor TEXT NOT NULL CHECK (decided_by_actor IN ('','owner','collective','moderator')),
    first_seen_at TEXT NOT NULL,
    last_validated_at TEXT NOT NULL,
    PRIMARY KEY (village_url, user_id, collective_id, transcript_id, event_num),
    FOREIGN KEY (village_url, user_id) REFERENCES collective_observation_scopes(village_url, user_id)
) WITHOUT ROWID;
`
