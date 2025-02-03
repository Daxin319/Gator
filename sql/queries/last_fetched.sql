-- name: MarkFeedFetched :exec
UPDATE feeds SET
    last_fetched_at = $1,
    updated_at = $1
WHERE feeds.id = $2;

-- name: GetNextFeedToFetch :one
SELECT id, url, name FROM feeds
ORDER BY last_fetched_at ASC NULLS FIRST;
