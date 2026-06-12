# Security Policy

## Supported versions

`tele` is in active development. Security fixes are applied to the latest
released version. Please make sure you are on the most recent
[release](https://github.com/sorokin-vladimir/tele/releases) before reporting.

## Reporting a vulnerability

**Please do not report security vulnerabilities through public GitHub issues.**

Instead, report them privately through GitHub's
[Security Advisories](https://github.com/sorokin-vladimir/tele/security/advisories/new)
("Report a vulnerability"). If that is not possible, email
**v.sorokin@hey.com** with the details.

Please include:

- a description of the issue and its impact,
- steps to reproduce or a proof of concept,
- the version of `tele` and your OS / terminal.

You can expect an initial response within a few days. We will keep you informed
as the issue is investigated and fixed, and credit you in the release notes
unless you prefer to remain anonymous.

## Handling sensitive data

`tele` talks to Telegram on your behalf and stores a session locally
(`~/.config/tele/session.json` by default). A few things worth knowing:

- The **session file grants access to your Telegram account** — treat it like a
  password. Do not share it or commit it.
- The `--trace` flag logs sensitive metadata (peer IDs, message lengths). Never
  use it on shared or synced file systems.
- API credentials (`buildAPIID` / `buildAPIHash`) are compiled into the binary
  and should be kept private if you distribute custom builds.

If you find a way these protections can be bypassed, please report it using the
process above.
