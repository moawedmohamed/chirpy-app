-- name: CreateUser :one
insert into users (email)
values (
    $1
     )
    RETURNING *;

-- name: DeleteALLUsers :exec

delete from users;

-- name: CreateChirp :one
INSERT  INTO  chirps(body,user_id)
values($1,$2)
    RETURNING *  ;
