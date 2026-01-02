# Security Policy

## Reporting Vulnerabilities

If you discover a security vulnerability in Mehrhof, please **do not** create a public issue.

Instead, send your report privately:

1. **GitHub Security Advisory** (Recommended):
   - Visit https://github.com/valksor/go-mehrhof/security/advisories
   - Click "Report a vulnerability"
   - Fill in the details
   - Your report will be private and visible only to maintainers

2. **Email**:
   - Send details to packages@valksor.com
   - Include "SECURITY: Mehrhof" in the subject line

### What to Include

Please include as much detail as possible:

- Description of the vulnerability
- Steps to reproduce the issue
- Potential impact
- Suggested mitigation (if known)
- Your name/handle for credit (optional)

## Response Timeline

- **Initial response**: Within 48 hours
- **Detailed assessment**: Within 7 days
- **Patch release**: Based on severity, typically within 14 days

You will be notified when:
- We confirm the vulnerability
- A fix is being developed
- A patch is released

## Supported Versions

Security updates are provided for the **current major version** only.

| Version | Support Status |
|---------|----------------|
| 0.x     | Supported      |

When a new major version is released, security updates for the previous version may be provided for a limited transition period (typically 3 months).

## Security Updates

### Release Process

1. **Private Fix**: We develop a fix privately
2. **Coordinated Disclosure**: We coordinate disclosure timeline with you
3. **Patch Release**: We release a patch version (e.g., 0.1.0 → 0.1.1)
4. **Public Disclosure**: After a grace period (typically 7 days), we publish the security advisory

### Announcements

Security updates are announced via:
- GitHub Security Advisories
- Release notes
- Commit messages (marked `[security]`)

## Security Best Practices

### For Users

- **Keep updated**: Install the latest version to get security fixes
- **Review permissions**: Only grant necessary API tokens and permissions
- **Secure secrets**: Store API keys in environment variables or `.mehrhof/.env` (never commit secrets)
- **Audit dependencies**: Run `make quality` which includes `govulncheck`

### For Developers

- **Input validation**: Always validate and sanitize user input
- **No credentials in logs**: Never log API keys, tokens, or sensitive data
- **Use context**: Always pass `context.Context` for cancelable operations
- **Error handling**: Don't expose sensitive information in error messages
- **Dependency updates**: Regularly update dependencies and run security scans

## Security-Related Features

- **Secrets management**: API keys stored in `.mehrhof/.env` (gitignored)
- **No credential leakage**: Secrets are never logged or included in error messages
- **Dependency scanning**: `govulncheck` runs in `make quality`
- **HTTPS only**: All provider communications use HTTPS/TLS

## Contact

For security-related questions not involving vulnerability disclosure:

- **General security inquiries**: security@valksor.com
- **GitHub Security Advisories**: https://github.com/valksor/go-mehrhof/security/advisories

Thank you for helping keep Mehrhof secure!
