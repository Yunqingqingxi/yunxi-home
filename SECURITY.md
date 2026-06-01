# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in Yunxi Home, please report it by:

- **Opening a GitHub Issue** — [https://github.com/Yunqingqingxi/yunxi-home/issues](https://github.com/Yunqingqingxi/yunxi-home/issues) (mark with `security` label if possible)
- **Email** — Send details to the project maintainers via the email addresses listed on the GitHub profile.

Please include as much information as possible to help us reproduce and resolve the issue quickly:

- Type of vulnerability
- Steps to reproduce
- Affected versions
- Potential impact

We aim to acknowledge receipt within 48 hours and provide a fix or mitigation within a reasonable timeframe depending on severity.

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 3.0.x   | :white_check_mark: |
| < 3.0   | :x:                |

## Security Best Practices

- Run Yunxi Home behind a reverse proxy (e.g., Nginx) with HTTPS enabled.
- Use strong, unique API keys for all configured providers.
- Restrict file sandbox paths to isolate sensitive system files.
- Keep the binary and dependencies updated to the latest version.
