-- name: CreateChrip :one
INSERT INTO chirps (id, body, user_id, created_at, updated_at)
VALUES (
  gen_random_uuid(),
  $1,
  $2,
  NOW(),  
  NOW()
)
RETURNING *;

-- name: GetAllChirps :many
SELECT * FROM chirps ORDER BY created_at;

-- name: GetChirp :one
SELECT * FROM chirps WHERE id = $1;