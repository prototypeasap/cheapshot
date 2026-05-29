package store

const schemaSQL = `
CREATE TABLE IF NOT EXISTS batches (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    provider        TEXT NOT NULL,
    input_file      TEXT NOT NULL,
    request_count   INTEGER,
    remote_file_id  TEXT,
    remote_batch_id TEXT,
    endpoint        TEXT,
    status          TEXT NOT NULL DEFAULT 'pending',
    error           TEXT,
    output_file     TEXT,
    remote_status   TEXT,
    remote_json     TEXT,
    succeeded_count INTEGER DEFAULT 0,
    failed_count    INTEGER DEFAULT 0,
    created_at      DATETIME DEFAULT (datetime('now')),
    uploaded_at     DATETIME,
    submitted_at    DATETIME,
    completed_at    DATETIME,
    downloaded_at   DATETIME,
    last_polled_at  DATETIME
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_remote_batch_id
    ON batches(remote_batch_id) WHERE remote_batch_id IS NOT NULL;
`
