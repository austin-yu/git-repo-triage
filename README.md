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
# 1. Clone the repo
git clone https://github.com/austin-yu/repo-triage.git
cd repo-triage

# 2. Build the frontend
cd web && npm install && npm run build && cd ..

# 3. Build the single binary (embeds the frontend)
go build -o repo-triage
```

This produces a single `repo-triage` executable (~8 MB) with the UI embedded.

## Usage

```bash
# Run the server
./repo-triage
```

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
