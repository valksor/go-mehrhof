# Add User Authentication

Implement user authentication using JWT tokens.

## Requirements

- Support email/password login
- Generate JWT access tokens (1 hour expiry)
- Generate refresh tokens (7 day expiry)
- Store refresh tokens securely
- Implement logout (revoke refresh token)

## Acceptance Criteria

- [ ] POST /api/auth/login returns tokens on success
- [ ] POST /api/auth/refresh exchanges refresh token for new access token
- [ ] POST /api/auth/logout revokes the refresh token
- [ ] Protected routes return 401 without valid access token
- [ ] All endpoints have tests with >80% coverage

## Technical Notes

- Use bcrypt for password hashing (cost factor 12)
- Store refresh tokens in database with user_id and expiry
- Access token should contain user_id and email claims
