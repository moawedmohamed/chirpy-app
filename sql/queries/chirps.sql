-- name: GetAllChirps :many

SELECT * FROM chirps;


-- name: CreateChirp :one
INSERT  INTO  chirps(body,user_id)
values($1,$2)
RETURNING * ;


-- name: GetChirpByID :one

SELECT * FROM chirps 
WHERE  id=$1;
