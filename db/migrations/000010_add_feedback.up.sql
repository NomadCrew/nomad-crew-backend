CREATE TABLE IF NOT EXISTS feedback (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       VARCHAR(100) NOT NULL CHECK (char_length(name) >= 1),
    email      VARCHAR(255) NOT NULL,
    message    TEXT NOT NULL CHECK (char_length(message) >= 10 AND char_length(message) <= 5000),
    source     VARCHAR(50) DEFAULT 'landing',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_feedback_created_at ON feedback (created_at DESC);
