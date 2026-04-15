-- Auth (users + email identities + action sessions)

-- ================
-- User bootstrap
-- ================

-- name: CreateUser :one
INSERT INTO users DEFAULT VALUES
RETURNING id;

-- name: InitUserProfile :exec
INSERT INTO user_profiles (user_id)
VALUES ($1);

-- name: InitUserSettings :exec
INSERT INTO user_settings (user_id)
VALUES ($1);

-- ================
-- Email identity
-- ================

-- name: CreateUserEmailIdentity :one
INSERT INTO user_email_identities (user_id, email, email_verified, password_hash)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetUserEmailIdentityByUserID :one
SELECT *
FROM user_email_identities
WHERE user_id = $1;

-- name: GetEmailIdentityByEmail :one
SELECT *
FROM user_email_identities
WHERE email = $1;

-- name: MarkEmailVerified :one
UPDATE user_email_identities
SET email_verified = true
WHERE user_id = $1
RETURNING *;

-- name: SetPasswordHashByUserID :one
UPDATE user_email_identities
SET password_hash = $2
WHERE user_id = $1
RETURNING *;

-- ================
-- Email action sessions (signup/reset_password)
-- purpose: 1=signup, 2=reset_password
-- ================

-- name: CreateEmailActionSession :one
INSERT INTO email_action_sessions (purpose, email, token_hash, expires_at)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetEmailActionSessionByHash :one
SELECT *
FROM email_action_sessions
WHERE token_hash = $1
  AND used_at IS NULL
  AND expires_at > now()
LIMIT 1;

-- name: GetEmailActionSessionByHashForUpdate :one
SELECT *
FROM email_action_sessions
WHERE token_hash = $1
  AND used_at IS NULL
  AND expires_at > now()
LIMIT 1
FOR UPDATE;

-- name: MarkEmailActionSessionUsed :one
UPDATE email_action_sessions
SET used_at = now()
WHERE id = $1
  AND used_at IS NULL
RETURNING *;

-- name: InvalidateEmailActionSessionsByEmailPurpose :exec
UPDATE email_action_sessions
SET used_at = now()
WHERE email = $1
  AND purpose = $2
  AND used_at IS NULL;
