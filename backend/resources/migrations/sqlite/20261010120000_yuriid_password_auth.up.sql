-- YuriID: password authentication (hashes and single-use email tokens)
CREATE TABLE yuriid_user_passwords
(
    user_id       TEXT   NOT NULL PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    password_hash TEXT   NOT NULL,
    updated_at    BIGINT NOT NULL
);

CREATE TABLE yuriid_auth_tokens
(
    id         TEXT   NOT NULL PRIMARY KEY,
    user_id    TEXT   NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    purpose    TEXT   NOT NULL,
    token_hash TEXT   NOT NULL UNIQUE,
    expires_at BIGINT NOT NULL,
    used_at    BIGINT,
    created_at BIGINT NOT NULL
);

CREATE INDEX idx_yuriid_auth_tokens_user_purpose ON yuriid_auth_tokens (user_id, purpose);
CREATE INDEX idx_yuriid_auth_tokens_expires_at ON yuriid_auth_tokens (expires_at);
