-- MCP Server Schema
-- Three tables for persistent round-by-round game context.

CREATE TABLE IF NOT EXISTS game_settings (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    lobby_id    TEXT    NOT NULL UNIQUE,
    theme       TEXT    NOT NULL DEFAULT 'classic noir',
    players     TEXT    NOT NULL, -- JSON array of player names
    role_config TEXT    NOT NULL, -- JSON object e.g. {"mafia":2,"medic":1}
    created_at  TEXT    NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS game_events (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    lobby_id    TEXT    NOT NULL,
    round       INTEGER NOT NULL,
    event_type  TEXT    NOT NULL, -- kill, save, investigate, vote, eliminate
    actor       TEXT    NOT NULL,
    target      TEXT    NOT NULL,
    result      TEXT    NOT NULL, -- killed, saved, investigated, eliminated, etc.
    created_at  TEXT    NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_game_events_lobby_round
    ON game_events (lobby_id, round);

CREATE TABLE IF NOT EXISTS narratives (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    lobby_id    TEXT    NOT NULL,
    round       INTEGER NOT NULL,
    story_type  TEXT    NOT NULL, -- game_intro, night_recap, day_intro, vote_recap
    story       TEXT    NOT NULL,
    created_at  TEXT    NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_narratives_lobby_round
    ON narratives (lobby_id, round);
