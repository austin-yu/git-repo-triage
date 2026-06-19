# Changelog

All notable changes to this project will be documented in this file.

Format based on [Keep a Changelog](https://keepachangelog.com/).

## [0.1.0] - 2026-06-20

### Added
- Structural Risk Matrix — scatter plot of file churn vs bug-fix frequency
- Bus Factor — donut chart showing commit share per contributor
- Sleeping Giants — bubble chart of file age vs size vs complexity
- Team Momentum — dual-axis line chart of monthly commits and hotfixes
- Operational Health panel with firefighting count and blast-radius commits (with SHA cross-reference)
- Dark mode (default) with light mode toggle
- Single-binary distribution with embedded Vue frontend via `go:embed`
- Makefile with `build`, `frontend`, `clean`, and `dev` targets
- Cross-compilation instructions for macOS, Linux, and Windows
- Language-agnostic analysis supporting 25+ file extensions
