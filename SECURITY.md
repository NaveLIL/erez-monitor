# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 1.0.x   | :white_check_mark: |

## Reporting a Vulnerability

If you discover a security vulnerability in EREZMonitor, please report it responsibly.

### How to Report

1. **Do NOT** create a public GitHub issue for security vulnerabilities
2. Send an email to the repository owner with details about the vulnerability
3. Include the following information:
   - Description of the vulnerability
   - Steps to reproduce
   - Potential impact
   - Any suggested fixes (optional)

### What to Expect

- **Response time**: We aim to respond within 48 hours
- **Resolution**: Critical vulnerabilities will be addressed as soon as possible
- **Credit**: If you wish, we will credit you in the release notes

## Security Best Practices

When using EREZMonitor:

1. **Download from official sources only** - Only download releases from the official GitHub repository
2. **Verify checksums** - When available, verify the SHA256 checksum of downloaded binaries
3. **Run with minimal privileges** - EREZMonitor requires Administrator privileges for some features, but avoid running as SYSTEM
4. **Keep updated** - Always use the latest version to benefit from security fixes
5. **Review autostart** - If using autostart feature, ensure only trusted copies of EREZMonitor are configured

## Known Security Considerations

### Data Collection
- EREZMonitor collects system metrics (CPU, RAM, GPU, Network, Disk usage) locally
- No data is transmitted to external servers
- Metrics are stored in local log files only

### Permissions Required
- **Administrator**: Required for some hardware monitoring features
- **Registry access**: Only for autostart configuration (optional)
- **Network**: Only for local network interface monitoring, no external connections

### File Locations
- Config: `%APPDATA%\EREZMonitor\config.yaml`
- Logs: `%APPDATA%\EREZMonitor\logs\`
- Metrics export: User-specified location

## Changelog

Security-related changes will be documented in release notes with the `[Security]` tag.
