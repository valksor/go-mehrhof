# Fix: Pagination Returns Wrong Total Count

## Bug Description

The `/api/users` endpoint returns incorrect `total_count` when filters are applied. The count includes all users instead of only filtered users.

## Steps to Reproduce

1. Create 100 users, 50 with role "admin", 50 with role "user"
2. GET /api/users?role=admin&page=1&limit=10
3. Response shows `total_count: 100` instead of `total_count: 50`

## Expected Behavior

`total_count` should reflect the count after applying filters.

## Root Cause

The `CountUsers()` function doesn't receive the filter parameters and always counts all users.

## Fix

Update `CountUsers(filters)` to accept and apply the same filters used in the list query.

## Affected Files

- `internal/repository/user.go` - CountUsers function
- `internal/handler/user.go` - handler that calls the repository
- `internal/repository/user_test.go` - tests
