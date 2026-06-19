# Contributing

Thanks for your interest in repo-triage.

## Development Setup

Prerequisites: Go 1.25+, Node.js 18+, Git, Bash.

```bash
git clone https://github.com/austin-yu/repo-triage.git
cd repo-triage

# Run backend
go run main.go

# In another terminal, run frontend with hot reload
cd web && npm install && npm run dev
```

The frontend dev server runs on `http://localhost:5173` and proxies API calls to the Go backend on port 8080.

## Building

```bash
make build        # frontend + Go binary → bin/repo-triage
make clean        # remove build artifacts
```

## Submitting Changes

1. Fork the repository
2. Create a branch from `main`
3. Make your changes
4. Ensure `make build` succeeds
5. Open a pull request with a clear description of what changed and why

## Reporting Bugs

Open an issue with:
- What you expected to happen
- What actually happened
- Steps to reproduce
- OS, Go version, Node version

## License

By contributing, you agree that your contributions will be licensed under the Apache-2.0 License.
