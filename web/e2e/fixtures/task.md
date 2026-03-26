# Build bookmark manager API

Build a bookmark manager HTTP API in Go.

The API stores bookmarks in memory and serves a single-page HTML dashboard for browsing them.

A bookmark has a URL, title, optional tags, and timestamps for when it was created and last updated.

The API should support:
- Listing all bookmarks as JSON
- Adding a new bookmark (URL, title, tags)
- Getting a single bookmark by ID
- Updating a bookmark
- Deleting a bookmark
- Searching bookmarks by title or URL substring (case-insensitive)
- Serving an HTML dashboard at the root that lists bookmarks, has a form to add new ones, and a search input

The HTML dashboard should be embedded in the binary using Go embed. It should show bookmarks in a table with clickable titles, tags, and creation dates. Include basic styling (centered layout, simple table, form).

Tests should verify every endpoint works correctly and edge cases like missing IDs return proper errors.

`go build ./...` and `go test ./...` must both pass.
