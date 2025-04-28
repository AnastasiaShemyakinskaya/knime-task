CREATE TABLE messages (
    id SERIAL PRIMARY KEY,
    message BYTEA NOT NULL,
    message_type INTEGER DEFAULT 0,
    topic VARCHAR(256),
    message_id INTEGER UNIQUE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMP NULL
);

CREATE INDEX idx_messages_unprocessed ON messages (processed_at) WHERE processed_at IS NULL;

CREATE TABLE example (
    id SERIAL PRIMARY KEY,
    text TEXT NOT NULL,
    isSent BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_example_notsent ON example (isSent) WHERE isSent IS FALSE;
