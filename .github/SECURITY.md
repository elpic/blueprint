# Security Policy

Blueprint is a system automation tool that executes arbitrary commands, manages
secrets, and modifies system state. Security is a first-class concern.

## Supported Versions

Only the latest release receives security updates. There are no LTS releases.

| Version | Supported |
|---------|-----------|
| latest  | ✅        |
| < latest | ❌       |

## Reporting a Vulnerability

**Do not open a public GitHub issue for security vulnerabilities.**

Instead, report privately via one of these channels:

- **Email**: [elpicx@gmail.com](mailto:elpicx@gmail.com)
- **GitHub Security Advisory**: Use the
  ["Report a vulnerability"](https://github.com/elpic/blueprint/security/advisories/new)
  form under the repository's Security tab.

You should receive a response within 48 hours. If you don't, please follow up.

### What to include

- Description of the vulnerability and impact
- Steps to reproduce (config file, blueprint, command)
- Affected versions
- Any suggested fix (optional)

### What to expect

1. Acknowledgment of receipt within 48 hours
2. Assessment and validation of the report
3. A fix and release timeline
4. Credit in the release notes (if desired)

## Security Posture

### What Blueprint does with your data

- **Secrets**: Decryption happens only in-memory using a password you provide.
  Decrypted secrets are never written to disk.
- **Credentials**: SSH keys, API tokens, and other secrets are only passed to
  the tools that need them (git, curl, etc.) and are not logged.
- **State**: `~/.blueprint/status.json` tracks what was installed; it contains
  formula names, clone URLs, and timestamps — **not** secrets or passwords.

### CI Security Checks

Every pull request runs automated security scanning:

- **[gosec](https://github.com/securego/gosec)** — static analysis for Go
  security issues (G204 subprocess injection, G304 file paths, G115 integer
  overflows, etc.)
- **[CodeQL](https://codeql.github.com/)** — GitHub's semantic code analysis
  for Go, JavaScript, and Actions
- **Dependency review** — checks new dependencies for known vulnerabilities

### Baseline Practices

- Homebrew commands are executed via `sh -c` and should never use unsanitized
  user input as command arguments (existing G204 exclusions are audited)
- File permissions default to `0600` for sensitive files (config files,
  SSH-related data)
- GPG-encrypted files (`*.enc`) are decrypted in memory and never written as
  plaintext
- `sudo` usage is scoped to the minimum necessary commands
- No telemetry, no analytics, no network calls beyond explicit blueprint rules

## Security-Related Roadmap

Items from the project backlog that improve security posture:

- [ ] [`replace` action](/.brain/backlog.md) — patch-managed files like
  `go.mod` without full-file overwrites (reduces risk of malformed writes)
- [ ] [`line-match` drift check](/.brain/backlog.md) — validate specific lines
  in managed files rather than trusting full-file content
- [ ] Dependency updates automation for Go modules and GitHub Actions
- [ ] Signed commits requirement for releases

## Responsible Disclosure

We believe in responsible disclosure. Please give us a reasonable window to
fix and release before publishing vulnerabilities publicly.
