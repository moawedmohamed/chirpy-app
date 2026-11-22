-- name: GetAllChirps :many

SELECT * FROM chirps;
-- name: GetAllChirpsDesc :many

SELECT * FROM chirps
ORDER BY created_at DESC
;

-- name: GetChirpsByUserID :many
SELECT * FROM chirps
WHERE user_id=$1
ORDER BY created_at ;

-- name: CreateChirp :one
INSERT  INTO  chirps(body,user_id)
values($1,$2)
RETURNING * ;

-- name: GetChirpsByUserIDDesc :many
SELECT *
FROM chirps
WHERE user_id = $1
ORDER BY created_at DESC;

-- name: GetChirpByID :one

SELECT * FROM chirps 
WHERE  id=$1;


-- name: DeleteChirpByID :exec
DELETE FROM chirps
WHERE id=$1;
