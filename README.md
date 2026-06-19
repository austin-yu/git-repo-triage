# repo-triage

Visualise the social, architectural, and operational reality of a codebase using Git history.

## What It Does

Point repo-triage at any local Git repository and get four diagnostic visualisations:

| Visualisation | Chart Type | Insight |
|---|---|---|
| **Structural Risk Matrix** | Scatter (Churn vs Bugs) | Files in the top-right quadrant change often and break often — prime refactor targets |
| **Bus Factor** | Donut (Contributor Share) | A single large slice means one person holds most of the knowledge |
| **Sleeping Giants** | Bubble (Age vs Size vs Complexity) | Large, complex files nobody has touched in months — the team may be avoiding them |
| **Team Momentum** | Dual-axis Line (Commits + Hotfixes over time) | Drops in velocity or spikes in hotfixes signal process or staffing issues |

## Prerequisites

- [Go](https://go.dev/dl/) 1.25+
- [Node.js](https://nodejs.org/) 18+ and npm
- Git
- Bash (used by the analysis engine to run git commands)

## Building

```bash
# Clone the repo
git clone https://github.com/austin-yu/repo-triage.git
cd repo-triage

# Build everything (frontend + Go binary)
make build
```

This produces a single `bin/repo-triage` executable (~8 MB) with the UI embedded.

### Cross-compiling

Go supports cross-compilation out of the box. Set `GOOS` and `GOARCH` before building:

```bash
# Build the frontend first
make frontend

# macOS (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -o bin/repo-triage-darwin-arm64

# macOS (Intel)
GOOS=darwin GOARCH=amd64 go build -o bin/repo-triage-darwin-amd64

# Linux (x86_64)
GOOS=linux GOARCH=amd64 go build -o bin/repo-triage-linux-amd64

# Linux (ARM64, e.g. AWS Graviton)
GOOS=linux GOARCH=arm64 go build -o bin/repo-triage-linux-arm64

# Windows (x86_64)
GOOS=windows GOARCH=amd64 go build -o bin/repo-triage-windows-amd64.exe
```

Other make targets:

| Command | Description |
|---|---|
| `make build` | Build frontend and Go binary to `bin/` |
| `make frontend` | Build just the frontend |
| `make clean` | Remove all build artifacts |
| `make dev` | Run the Go backend for development |

## Usage

### macOS / Linux

```bash
./bin/repo-triage
```

### Windows

```powershell
.\bin\repo-triage-windows-amd64.exe
```

> **Note:** On Windows, Git and Bash (Git Bash) must be installed and available on the system PATH. Git for Windows includes both.

Open **http://localhost:8080** in your browser, paste the absolute path to any local Git repository, and click **Analyze Repository**.

### API

The analysis endpoint can also be called directly:

```bash
curl 'http://localhost:8080/api/analyze?path=/path/to/your/repo'
```

### Health Check

```bash
curl http://localhost:8080/health
```

## Development

To run the frontend and backend separately during development:

```bash
# Terminal 1 — Backend
go run main.go

# Terminal 2 — Frontend (with hot reload and API proxy)
cd web && npm run dev
```

The Vite dev server runs on `http://localhost:5173` and proxies `/api` requests to the Go backend on port 8080.

## Supported Languages

repo-triage is language-agnostic. It analyses Git history, not source code syntax. The Sleeping Giants chart uses file extensions to find source files and currently recognises:

Go, TypeScript, JavaScript, Vue, Python, Java, Rust, Ruby, C, C++, C#, Swift, Kotlin, Scala, PHP, Elixir, Erlang, Haskell, OCaml, Clojure, Dart, Lua, R, Julia, Zig, and Nim.

All other visualisations (Risk Matrix, Bus Factor, Momentum) work with any repository regardless of language.

## License

Apache-2.0 — see [LICENSE](LICENSE) for details.
