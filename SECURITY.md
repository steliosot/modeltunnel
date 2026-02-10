# Security Policy

## Supported Versions

We release patches for security vulnerabilities. Which versions are eligible
receiving such patches depend on the CVSS v3.0 Rating:

| Version | Supported          |
| ------- | ------------------ |
| 1.x.x   | :white_check_mark: |
| < 1.0   | :x:                |

## Reporting a Vulnerability

Please report security vulnerabilities to **steliosot@gmail.com**.

Please include:
- Description of the vulnerability
- Steps to reproduce the issue
- Possible impact of the vulnerability
- Any possible mitigations you've identified

We will acknowledge receipt of your vulnerability report within 48 hours
and send you regular updates about our progress.

## Disclosure Policy

When we receive a security bug report, we will:

1. Confirm the problem and determine the affected versions
2. Audit code to find any similar problems
3. Prepare fixes for all supported versions
4. Release new versions as soon as possible

We will publicly disclose the issue after releasing a fix.

## Security Best Practices

### For Users

1. **Keep your software updated** - Always use the latest version
2. **Use strong API keys** - Keys are automatically generated with high entropy
3. **Protect your config** - The config file contains sensitive information
4. **Use HTTPS** - When exposing publicly, use a tunnel with HTTPS (LocalTunnel)
5. **Rate limiting** - Enable appropriate rate limits for your use case
6. **Restrict upstreams** - Limit which upstreams each key can access

### For Administrators

1. **File permissions** - Ensure `~/.config/modeltunnel/` has appropriate permissions (600)
2. **Database security** - The SQLite database contains API keys
3. **Network security** - Don't expose the server directly to the internet without authentication
4. **Monitor logs** - Check request logs for suspicious activity
5. **Regular key rotation** - Revoke and recreate keys periodically

### Configuration Security

```yaml
# Good: Restrictive configuration
policies:
  strict:
    rate_limit: 60/min
    max_tokens: 2048
    
keys:
  - name: production
    key: mt_sk_production_...
    allowed_upstreams: ["default"]
    policy: strict
```

### Common Security Concerns

#### API Key Storage
- Keys are stored in SQLite database at `~/.config/modeltunnel/keys.db`
- Config file also keeps a backup of keys
- Both files should have restricted permissions (600)

#### Rate Limiting
- Prevents abuse and DoS attacks
- Per-model rate limits allow fine-grained control
- Rate limit headers inform clients of their status

#### Authentication
- All API endpoints require valid API keys (except /health)
- Invalid or missing keys return 401 Unauthorized
- Keys can be revoked instantly

#### Model Access Control
- Keys can be restricted to specific upstreams
- Prevents access to unauthorized models
- Supports wildcard matching (e.g., `tinyllama:*`)

## Known Security Considerations

1. **Local Development** - Default configuration binds to localhost only (127.0.0.1)
2. **Tunnel Security** - Public tunnels (LocalTunnel) provide HTTPS automatically
3. **No Encryption at Rest** - SQLite database is not encrypted (keys are stored as-is)
4. **Hot Reload** - Config changes apply without restart (ensure file permissions)

## Security Checklist Before Production

- [ ] Change default policies for production use
- [ ] Set up appropriate rate limits
- [ ] Restrict key access to necessary upstreams only
- [ ] Enable tunnel for public access (HTTPS)
- [ ] Set proper file permissions on config directory
- [ ] Regularly review and rotate API keys
- [ ] Monitor request logs for anomalies
- [ ] Keep software updated to latest version

## Contact

Security issues: **steliosot@gmail.com**

General questions: Open a GitHub issue (not for security issues)

---

Thank you for helping keep Modeltunnel and our users safe!
