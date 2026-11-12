-- name: CreateUser :one
insert into users (email)
values (
    $1
     )
    RETURNING *;

-- name: DeleteALLUsers :exec

delete from users;
