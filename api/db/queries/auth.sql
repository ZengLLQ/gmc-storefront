-- name: CreateUser :one
INSERT INTO users (id, email, name, phone, password_hash, role, is_active, must_change_password)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: UpdateUserProfile :one
UPDATE users
SET name = $2, email = $3, phone = $4, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateUserPassword :exec
UPDATE users
SET password_hash = $2, must_change_password = false, updated_at = now()
WHERE id = $1;

-- name: UpdateUserLastLogin :exec
UPDATE users SET last_login_at = now() WHERE id = $1;

-- name: SetUserActive :exec
UPDATE users SET is_active = $2, updated_at = now() WHERE id = $1;

-- name: SetUserMustChangePassword :exec
UPDATE users SET must_change_password = $2, updated_at = now() WHERE id = $1;

-- name: CountActiveAdmins :one
SELECT count(*) FROM users WHERE role = 'admin' AND is_active = true;

-- name: ListStaffUsers :many
SELECT * FROM users WHERE role IN ('admin', 'staff') ORDER BY created_at ASC;

-- name: UpdateStaffUser :one
UPDATE users
SET name = COALESCE(sqlc.narg('name'), name),
    phone = COALESCE(sqlc.narg('phone'), phone),
    role = COALESCE(sqlc.narg('role'), role),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: CreateSession :one
INSERT INTO sessions (
  id, user_id, family_id, token_hash, user_agent, ip, expires_at, is_admin_session, absolute_expires_at
) VALUES ($1, $2, $3, $4, $5, $6::inet, $7, $8, $9)
RETURNING *;

-- name: GetSessionByTokenHash :one
SELECT * FROM sessions WHERE token_hash = $1;

-- name: GetSessionByID :one
SELECT * FROM sessions WHERE id = $1;

-- name: MarkSessionRotated :exec
UPDATE sessions
SET rotated_at = now(), replaced_by = $2
WHERE id = $1;

-- name: RevokeSessionByID :exec
UPDATE sessions SET revoked_at = now() WHERE id = $1;

-- name: RevokeSessionFamily :exec
UPDATE sessions SET revoked_at = now() WHERE family_id = $1 AND revoked_at IS NULL;

-- name: RevokeUserSessionsExcept :exec
UPDATE sessions SET revoked_at = now() WHERE user_id = $1 AND id <> $2 AND revoked_at IS NULL;

-- name: RevokeAllUserSessions :exec
UPDATE sessions SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL;

-- name: ListActiveAdminSessions :many
SELECT * FROM sessions
WHERE user_id = $1 AND is_admin_session = true AND revoked_at IS NULL AND rotated_at IS NULL AND expires_at > now()
ORDER BY created_at DESC;

-- name: UpsertUserTotp :exec
INSERT INTO user_totp (user_id, secret_enc, confirmed_at, backup_codes_hash)
VALUES ($1, $2, $3, $4)
ON CONFLICT (user_id) DO UPDATE
SET secret_enc = EXCLUDED.secret_enc,
    confirmed_at = EXCLUDED.confirmed_at,
    backup_codes_hash = EXCLUDED.backup_codes_hash;

-- name: GetUserTotp :one
SELECT * FROM user_totp WHERE user_id = $1;

-- name: DeleteUserTotp :exec
DELETE FROM user_totp WHERE user_id = $1;

-- name: ListUserAddresses :many
SELECT * FROM addresses WHERE user_id = $1 ORDER BY is_default DESC, created_at DESC;

-- name: CountUserAddresses :one
SELECT count(*) FROM addresses WHERE user_id = $1;

-- name: ClearUserDefaultAddress :exec
UPDATE addresses SET is_default = false WHERE user_id = $1 AND is_default = true;

-- name: CreateAddress :one
INSERT INTO addresses (id, user_id, label, recipient_name, phone, street, city, province, postal_code, is_default)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING *;

-- name: GetAddressByIDAndUser :one
SELECT * FROM addresses WHERE id = $1 AND user_id = $2;

-- name: UpdateAddress :one
UPDATE addresses
SET label = $3, recipient_name = $4, phone = $5, street = $6, city = $7, province = $8, postal_code = $9, is_default = $10
WHERE id = $1 AND user_id = $2
RETURNING *;

-- name: DeleteAddress :execrows
DELETE FROM addresses WHERE id = $1 AND user_id = $2;
