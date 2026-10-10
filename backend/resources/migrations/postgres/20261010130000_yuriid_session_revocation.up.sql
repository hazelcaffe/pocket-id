-- YuriID: per-user session revocation. Access tokens issued at or before revoked_at are rejected,
-- which lets a password reset sign the user out everywhere.
CREATE TABLE yuriid_session_revocations
(
    user_id    UUID   NOT NULL PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    revoked_at BIGINT NOT NULL
);
