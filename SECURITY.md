# Security Policy

## Scope

repo-triage is a **local-only** tool. It runs a web server on `localhost` and analyses Git repositories on your local filesystem. It is not designed to be exposed to the internet.

## Reporting a Vulnerability

If you find a security issue, please open a GitHub issue. Since this is a local tool with no network exposure by design, most issues are low severity — but we still want to know about them.

For anything you consider sensitive, email the maintainer directly rather than opening a public issue.

## Known Limitations

- **Shell execution:** The analysis engine runs Git commands via `bash -c`. Repo paths are not sanitised against shell injection. Only analyse repositories you trust, using paths you control.
- **No authentication:** The local HTTP server has no auth. Any process on your machine can call the API.
- **CORS:** Set to `*` for local convenience. Do not expose the server to untrusted networks.
