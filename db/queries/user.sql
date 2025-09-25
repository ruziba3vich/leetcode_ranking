-- name: CreateUser :one
INSERT INTO user_data (
  username, user_slug, user_avatar, country_code, country_name, real_name, typename,
  total_problems_solved, total_submissions
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9
)
RETURNING *;

-- name: UpsertUser :one
INSERT INTO user_data (
  username, user_slug, user_avatar, country_code, country_name, real_name, typename,
  total_problems_solved, total_submissions
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9
)
ON CONFLICT (username) DO UPDATE
SET
  user_slug = EXCLUDED.user_slug,
  user_avatar = EXCLUDED.user_avatar,
  country_code = EXCLUDED.country_code,
  country_name = EXCLUDED.country_name,
  real_name = EXCLUDED.real_name,
  typename = EXCLUDED.typename,
  total_problems_solved = EXCLUDED.total_problems_solved,
  total_submissions = EXCLUDED.total_submissions
RETURNING *;

-- name: GetUserByUsername :one
SELECT * FROM user_data
WHERE username = $1
LIMIT 1;

-- name: ListUsers :many
SELECT * FROM user_data
WHERE country_code IS NOT NULL AND country_code != ''
ORDER BY total_problems_solved DESC, total_submissions ASC
LIMIT $1 OFFSET $2;

-- name: GetUsersByCountry :many
SELECT * FROM user_data
WHERE country_code = $1
ORDER BY 
  total_problems_solved DESC,
  total_submissions ASC,
  username ASC
LIMIT $2 OFFSET $3;

-- name: UpdateUserByUsername :one
UPDATE user_data
SET
  user_slug = COALESCE($2, user_slug),
  user_avatar = COALESCE($3, user_avatar),
  country_code = COALESCE($4, country_code),
  country_name = COALESCE($5, country_name),
  real_name = COALESCE($6, real_name),
  typename = COALESCE($7, typename),
  total_problems_solved = COALESCE($8, total_problems_solved),
  total_submissions = COALESCE($9, total_submissions)
WHERE username = $1
RETURNING *;

-- name: DeleteUserByUsername :exec
DELETE FROM user_data
WHERE username = $1;

-- name: GetAllUsersCountByCountry :one
SELECT COUNT(*) FROM user_data
WHERE country_code = $1;

-- name: GetAllUsers
