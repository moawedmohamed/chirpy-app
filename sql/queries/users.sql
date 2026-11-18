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
