-- name: CreateFeed :one
INSERT INTO feeds (id, created_at, updated_at, name, url, user_id)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6
)
RETURNING *;

-- name: GetCreator :one
SELECT name FROM users
WHERE users.id = $1;

-- name: GetFeeds :many
SELECT name, url, user_id FROM feeds;

-- name: URLLookup :one
SELECT name, id FROM feeds
WHERE url = $1;