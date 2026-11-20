-- name: CreateUser :one
insert into users (email,hashed_password)
values (
    $1,
$2
    )
    RETURNING *;

-- name: DeleteALLUsers :exec

delete from users;


-- name: GetUserByEmail :one

SELECT * from users 
WHERE email=$1;

-- name: CreateRefreshToken :one
insert into refresh_tokens (user_id,token,expires_at)
values (
    $1,
$2,
    $3
    )
    RETURNING *; 

-- name: GetRefreshToken :one
SELECT * from refresh_tokens
WHERE token=$1
AND revoked_at IS NULL
AND expires_at > NOW();

-- name: RevokeRefreshToken :exec
UPDATE refresh_tokens
SET revoked_at = NOW(),
    updated_at = NOW()
WHERE token=$1;

