package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

//go:embed web/dist
var embeddedUI embed.FS

// --- Constants ---

const (
	TimePeriod          = "1 year ago"
	GitListLimit        = 30
	CouplingAlertsLimit = 5
	AnalysisTaskCount   = 6
	DefaultServerPort   = ":8080"
)

// File patterns to exclude from analysis
var (
	ExcludePatterns = []string{
		"node_modules",
		"vendor",
		"dist",
		".git",
		"__pycache__",
		".venv",
		"target",
		"build",
		".gradle",
		".next",
		"coverage",
		".yarn",
		"go.sum",
		"yarn.lock",
		"package-lock.json",
		"Cargo.lock",
		"Gemfile.lock",
		"poetry.lock",
		"pnpm-lock.yaml",
	}

	FirefightingPatterns = []string{
		"revert",
		"hotfix",
		"emergency",
		"rollback",
	}
)

// --- Data Models ---

type FileRisk struct {
	Name  string `json:"name"`
	Churn int    `json:"churn"`
	Bugs  int    `json:"bugs"`
}

type Contributor struct {
	Name    string `json:"name"`
	Commits int    `json:"commits"`
}

type SleepingGiant struct {
	Name       string `json:"name"`
	Lines      int    `json:"lines"`
	DaysSince  int    `json:"daysSinceLastCommit"`
	Complexity int    `json:"complexity"`
}

type MonthlyActivity struct {
	Month    string `json:"month"`
	Commits  int    `json:"commits"`
	Hotfixes int    `json:"hotfixes"`
}

type CouplingAlert struct {
	SHA          string `json:"sha"`
	Subject      string `json:"subject"`
	FilesChanged int    `json:"filesChanged"`
	Insertions   int    `json:"insertions"`
	Deletions    int    `json:"deletions"`
}

type RepoReport struct {
	RiskMatrix      []FileRisk        `json:"riskMatrix"`
	BusFactor       []Contributor     `json:"busFactor"`
	Firefighting    int               `json:"firefightingIncidents"`
	CouplingAlerts  []CouplingAlert   `json:"couplingAlerts"`
	SleepingGiants  []SleepingGiant   `json:"sleepingGiants"`
	MonthlyActivity []MonthlyActivity `json:"monthlyActivity"`
}

// --- Execution Layer ---

// CommandExecutor handles shell command execution (allows testing via dependency injection)
type CommandExecutor interface {
	Run(cmd string) (string, error)
}

// DefaultExecutor implements CommandExecutor
type DefaultExecutor struct{}

func (e *DefaultExecutor) Run(cmdString string) (string, error) {
	cmd := exec.Command("bash", "-c", cmdString)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("command execution failed: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// --- Parsing Helpers ---

// parseCountList takes standard `git log | sort | uniq -c` output and returns a map
func parseCountList(raw string) map[string]int {
	result := make(map[string]int)
	lines := strings.Split(raw, "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			count, err := strconv.Atoi(parts[0])
			if err == nil {
				name := strings.Join(parts[1:], " ")
				result[name] = count
			}
		}
	}
	return result
}

// buildExcludePattern creates a grep-compatible exclude pattern
func buildExcludePattern(patterns []string) string {
	return strings.Join(patterns, "|")
}

// --- Input Validation ---

// isValidGitRepo checks if the path is a valid git repository
func isValidGitRepo(repoPath string) error {
	// Check if path exists
	info, err := os.Stat(repoPath)
	if err != nil {
		return fmt.Errorf("repository path does not exist: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("repository path is not a directory")
	}

	// Check if .git exists
	gitDir := filepath.Join(repoPath, ".git")
	if _, err := os.Stat(gitDir); err != nil {
		return fmt.Errorf("not a git repository: %s", repoPath)
	}

	return nil
}

// --- Analysis Tasks (Independently Testable) ---

// analyzeChurnAndBugs extracts the Risk Matrix data
func analyzeChurnAndBugs(executor CommandExecutor, repoPath string) (map[string]int, map[string]int, error) {
	excludePattern := buildExcludePattern(ExcludePatterns)

	// Build churn command: files changed in last year
	churnCmd := fmt.Sprintf(
		"git -C %s log --format=format: --name-only --since='%s' --no-merges | "+
			"grep -vE '%s' | sort | uniq -c | sort -nr | head -%d",
		repoPath, TimePeriod, excludePattern, GitListLimit,
	)

	churnRaw, err := executor.Run(churnCmd)
	if err != nil {
		return nil, nil, fmt.Errorf("churn analysis failed: %w", err)
	}

	// Build bugs command: commits mentioning bug/fix
	bugsCmd := fmt.Sprintf(
		"git -C %s log -i -E --grep='fix|bug|broken' --name-only --format='' "+
			"--since='%s' --no-merges | grep -vE '%s' | sort | uniq -c",
		repoPath, TimePeriod, excludePattern,
	)

	bugsRaw, err := executor.Run(bugsCmd)
	if err != nil {
		return nil, nil, fmt.Errorf("bug analysis failed: %w", err)
	}

	return parseCountList(churnRaw), parseCountList(bugsRaw), nil
}

// analyzeContributors extracts the Bus Factor data
func analyzeContributors(executor CommandExecutor, repoPath string) ([]Contributor, error) {
	cmd := fmt.Sprintf(
		"git -C %s shortlog -sn --no-merges --since='%s' HEAD",
		repoPath, TimePeriod,
	)

	raw, err := executor.Run(cmd)
	if err != nil {
		return nil, fmt.Errorf("contributor analysis failed: %w", err)
	}

	contributors := make([]Contributor, 0)
	for name, commits := range parseCountList(raw) {
		contributors = append(contributors, Contributor{Name: name, Commits: commits})
	}

	return contributors, nil
}

// analyzeFirefighting extracts the Firefighting Metrics
func analyzeFirefighting(executor CommandExecutor, repoPath string) (int, error) {
	patterns := strings.Join(FirefightingPatterns, "|")

	cmd := fmt.Sprintf(
		"git -C %s log --oneline --since='%s' --no-merges | "+
			"grep -iE '%s' | wc -l",
		repoPath, TimePeriod, patterns,
	)

	raw, err := executor.Run(cmd)
	if err != nil {
		return 0, fmt.Errorf("firefighting analysis failed: %w", err)
	}

	count, _ := strconv.Atoi(strings.TrimSpace(raw))
	return count, nil
}

// analyzeCoupling extracts the Architectural Blast Radius data with commit SHAs
func analyzeCoupling(executor CommandExecutor, repoPath string) ([]CouplingAlert, error) {
	// Use a delimiter to pair each commit line with its stat line
	cmd := fmt.Sprintf(
		"git -C %s log --format='COMMIT %%H %%s' --shortstat --since='%s' --no-merges",
		repoPath, TimePeriod,
	)

	raw, err := executor.Run(cmd)
	if err != nil {
		return make([]CouplingAlert, 0), nil
	}

	alerts := parseCouplingAlerts(raw)

	// Sort by files changed descending
	for i := 1; i < len(alerts); i++ {
		for j := i; j > 0 && alerts[j].FilesChanged > alerts[j-1].FilesChanged; j-- {
			alerts[j], alerts[j-1] = alerts[j-1], alerts[j]
		}
	}

	// Limit to top N
	if len(alerts) > CouplingAlertsLimit {
		alerts = alerts[:CouplingAlertsLimit]
	}

	return alerts, nil
}

// parseCouplingAlerts parses the paired COMMIT/stat lines from git log output
func parseCouplingAlerts(raw string) []CouplingAlert {
	alerts := make([]CouplingAlert, 0)
	lines := strings.Split(raw, "\n")

	var current *CouplingAlert
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "COMMIT ") {
			rest := strings.TrimPrefix(line, "COMMIT ")
			parts := strings.SplitN(rest, " ", 2)
			sha := ""
			subject := ""
			if len(parts) >= 1 {
				sha = parts[0]
			}
			if len(parts) >= 2 {
				subject = parts[1]
			}
			current = &CouplingAlert{SHA: sha, Subject: subject}
		} else if current != nil && strings.Contains(line, "changed") {
			// Parse stat line like: "3 files changed, 45 insertions(+), 12 deletions(-)"
			current.FilesChanged = extractStatNumber(line, "file")
			current.Insertions = extractStatNumber(line, "insertion")
			current.Deletions = extractStatNumber(line, "deletion")
			alerts = append(alerts, *current)
			current = nil
		}
	}

	return alerts
}

// extractStatNumber pulls the number before a keyword in a git stat line
func extractStatNumber(line string, keyword string) int {
	parts := strings.Fields(line)
	for i, part := range parts {
		if strings.HasPrefix(part, keyword) && i > 0 {
			n, _ := strconv.Atoi(parts[i-1])
			return n
		}
	}
	return 0
}

// analyzeSleepingGiants finds large, complex files that haven't been touched recently
func analyzeSleepingGiants(executor CommandExecutor, repoPath string) ([]SleepingGiant, error) {
	excludePattern := buildExcludePattern(ExcludePatterns)

	// Find source files, get line counts, sorted by size descending
	// Covers major language ecosystems
	filesCmd := fmt.Sprintf(
		"find %s -type f \\( -name '*.go' -o -name '*.ts' -o -name '*.tsx' -o -name '*.js' -o -name '*.jsx' -o -name '*.vue' "+
			"-o -name '*.py' -o -name '*.java' -o -name '*.rs' -o -name '*.rb' -o -name '*.cpp' -o -name '*.hpp' "+
			"-o -name '*.c' -o -name '*.h' -o -name '*.cs' -o -name '*.swift' -o -name '*.kt' -o -name '*.kts' "+
			"-o -name '*.scala' -o -name '*.php' -o -name '*.ex' -o -name '*.exs' -o -name '*.erl' "+
			"-o -name '*.hs' -o -name '*.ml' -o -name '*.clj' -o -name '*.dart' -o -name '*.lua' "+
			"-o -name '*.r' -o -name '*.R' -o -name '*.jl' -o -name '*.zig' -o -name '*.nim' \\) "+
			"| grep -vE '%s' | head -50",
		repoPath, excludePattern,
	)

	filesRaw, err := executor.Run(filesCmd)
	if err != nil || filesRaw == "" {
		return make([]SleepingGiant, 0), nil
	}

	files := strings.Split(filesRaw, "\n")
	giants := make([]SleepingGiant, 0)

	for _, file := range files {
		if file == "" {
			continue
		}

		// Get line count
		wcCmd := fmt.Sprintf("wc -l < '%s'", file)
		wcRaw, err := executor.Run(wcCmd)
		if err != nil {
			continue
		}
		lines, _ := strconv.Atoi(strings.TrimSpace(wcRaw))
		if lines < 50 {
			continue // skip small files
		}

		// Get days since last commit for this file
		relPath := strings.TrimPrefix(file, repoPath+"/")
		daysCmd := fmt.Sprintf(
			"git -C %s log -1 --format='%%cr' -- '%s' 2>/dev/null",
			repoPath, relPath,
		)
		daysRaw, err := executor.Run(daysCmd)
		if err != nil || daysRaw == "" {
			continue
		}
		daysSince := parseRelativeTime(daysRaw)

		// Rough complexity: count functions/methods/class definitions as a proxy
		// Covers Go, Python, JS/TS, Java/Kotlin/Scala, Rust, Ruby, C#, PHP, Elixir, etc.
		complexCmd := fmt.Sprintf(
			"grep -cE '(^func |^def |^fn |function |class |interface |trait |impl |module |struct |enum |pub fn |private |protected |public )' '%s' 2>/dev/null || echo 0",
			file,
		)
		complexRaw, _ := executor.Run(complexCmd)
		complexity, _ := strconv.Atoi(strings.TrimSpace(complexRaw))

		giants = append(giants, SleepingGiant{
			Name:       relPath,
			Lines:      lines,
			DaysSince:  daysSince,
			Complexity: complexity,
		})
	}

	return giants, nil
}

// parseRelativeTime converts git's relative time (e.g. "3 months ago") to approximate days
func parseRelativeTime(relative string) int {
	parts := strings.Fields(relative)
	if len(parts) < 2 {
		return 0
	}
	num, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0
	}
	unit := parts[1]
	switch {
	case strings.HasPrefix(unit, "year"):
		return num * 365
	case strings.HasPrefix(unit, "month"):
		return num * 30
	case strings.HasPrefix(unit, "week"):
		return num * 7
	case strings.HasPrefix(unit, "day"):
		return num
	case strings.HasPrefix(unit, "hour"):
		return 0
	case strings.HasPrefix(unit, "minute"), strings.HasPrefix(unit, "second"):
		return 0
	}
	return 0
}

// analyzeMonthlyActivity generates monthly commit and hotfix counts for the past 12 months
func analyzeMonthlyActivity(executor CommandExecutor, repoPath string) ([]MonthlyActivity, error) {
	// Get commits per month — use --date=format with %ad to avoid shell escaping issues
	commitsCmd := "git -C " + repoPath + " log --date=format:'%Y-%m' --format='%ad' --since='" + TimePeriod + "' --no-merges | sort | uniq -c"
	commitsRaw, err := executor.Run(commitsCmd)
	if err != nil {
		return make([]MonthlyActivity, 0), nil
	}

	commitsByMonth := parseCountList(commitsRaw)

	// Get hotfixes per month
	patterns := strings.Join(FirefightingPatterns, "|")
	hotfixCmd := "git -C " + repoPath + " log --date=format:'%Y-%m' --format='%ad %s' --since='" + TimePeriod + "' --no-merges | " +
		"grep -iE '" + patterns + "' | awk '{print $1}' | sort | uniq -c"
	hotfixRaw, _ := executor.Run(hotfixCmd)
	hotfixByMonth := parseCountList(hotfixRaw)

	// Merge into a sorted list of all months
	allMonths := make(map[string]bool)
	for m := range commitsByMonth {
		allMonths[m] = true
	}
	for m := range hotfixByMonth {
		allMonths[m] = true
	}

	activity := make([]MonthlyActivity, 0, len(allMonths))
	for month := range allMonths {
		activity = append(activity, MonthlyActivity{
			Month:    month,
			Commits:  commitsByMonth[month],
			Hotfixes: hotfixByMonth[month],
		})
	}

	// Sort by month ascending
	sortMonthlyActivity(activity)
	return activity, nil
}

func sortMonthlyActivity(a []MonthlyActivity) {
	for i := 1; i < len(a); i++ {
		for j := i; j > 0 && a[j].Month < a[j-1].Month; j-- {
			a[j], a[j-1] = a[j-1], a[j]
		}
	}
}

// --- Core Analysis Orchestrator ---

// AnalysisOrchestrator coordinates concurrent analysis tasks with proper error handling
type AnalysisOrchestrator struct {
	executor CommandExecutor
}

// NewAnalysisOrchestrator creates a new orchestrator with the given executor
func NewAnalysisOrchestrator(executor CommandExecutor) *AnalysisOrchestrator {
	return &AnalysisOrchestrator{executor: executor}
}

// Analyze runs all analysis tasks concurrently and assembles the final report
func (ao *AnalysisOrchestrator) Analyze(repoPath string) (*RepoReport, error) {
	// Validate repository first
	if err := isValidGitRepo(repoPath); err != nil {
		return nil, fmt.Errorf("invalid repository: %w", err)
	}

	report := &RepoReport{}
	var wg sync.WaitGroup
	var errMutex sync.Mutex
	var firstErr error

	// Capture the first error from any goroutine
	recordError := func(err error) {
		if err != nil {
			errMutex.Lock()
			if firstErr == nil {
				firstErr = err
			}
			errMutex.Unlock()
		}
	}

	// Buffered channels so sends don't block before wg.Wait() completes
	churnChan := make(chan map[string]int, 1)
	bugChan := make(chan map[string]int, 1)
	contributorChan := make(chan []Contributor, 1)
	firefightingChan := make(chan int, 1)
	couplingChan := make(chan []CouplingAlert, 1)
	giantsChan := make(chan []SleepingGiant, 1)
	activityChan := make(chan []MonthlyActivity, 1)

	wg.Add(AnalysisTaskCount)

	// 1. Analyze Churn and Bugs
	go func() {
		defer wg.Done()
		churn, bugs, err := analyzeChurnAndBugs(ao.executor, repoPath)
		recordError(err)
		if err == nil {
			churnChan <- churn
			bugChan <- bugs
		}
	}()

	// 2. Analyze Contributors (Bus Factor)
	go func() {
		defer wg.Done()
		contributors, err := analyzeContributors(ao.executor, repoPath)
		recordError(err)
		if err == nil {
			contributorChan <- contributors
		}
	}()

	// 3. Analyze Firefighting Incidents
	go func() {
		defer wg.Done()
		incidents, err := analyzeFirefighting(ao.executor, repoPath)
		recordError(err)
		if err == nil {
			firefightingChan <- incidents
		}
	}()

	// 4. Analyze Coupling Alerts
	go func() {
		defer wg.Done()
		alerts, err := analyzeCoupling(ao.executor, repoPath)
		recordError(err)
		if err == nil {
			couplingChan <- alerts
		}
	}()

	// 5. Analyze Sleeping Giants
	go func() {
		defer wg.Done()
		giants, err := analyzeSleepingGiants(ao.executor, repoPath)
		recordError(err)
		if err == nil {
			giantsChan <- giants
		}
	}()

	// 6. Analyze Monthly Activity
	go func() {
		defer wg.Done()
		activity, err := analyzeMonthlyActivity(ao.executor, repoPath)
		recordError(err)
		if err == nil {
			activityChan <- activity
		}
	}()

	wg.Wait()

	if firstErr != nil {
		return nil, fmt.Errorf("analysis failed: %w", firstErr)
	}

	// Collect results
	churnData := <-churnChan
	bugData := <-bugChan
	report.BusFactor = <-contributorChan
	report.Firefighting = <-firefightingChan
	report.CouplingAlerts = <-couplingChan
	report.SleepingGiants = <-giantsChan
	report.MonthlyActivity = <-activityChan

	// Build Risk Matrix by joining churn and bug data
	for name, churn := range churnData {
		report.RiskMatrix = append(report.RiskMatrix, FileRisk{
			Name:  name,
			Churn: churn,
			Bugs:  bugData[name], // Defaults to 0 if not found
		})
	}

	return report, nil
}

// --- HTTP Server ---

// JSONError represents a structured error response
type JSONError struct {
	Error string `json:"error"`
	Code  string `json:"code,omitempty"`
}

// respondJSON sends a JSON response with the given status code
func respondJSON(w http.ResponseWriter, status int, data interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(data)
}

func handleAnalyze(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")

	repoPath := r.URL.Query().Get("path")
	if repoPath == "" {
		respondJSON(w, http.StatusBadRequest, JSONError{
			Error: "Missing 'path' query parameter",
			Code:  "MISSING_PATH",
		})
		return
	}

	// Initialize the orchestrator with default executor
	orchestrator := NewAnalysisOrchestrator(&DefaultExecutor{})

	// Run analysis
	report, err := orchestrator.Analyze(repoPath)
	if err != nil {
		log.Printf("Analysis error: %v", err)
		respondJSON(w, http.StatusInternalServerError, JSONError{
			Error: err.Error(),
			Code:  "ANALYSIS_FAILED",
		})
		return
	}

	respondJSON(w, http.StatusOK, report)
}

// handleHealth provides a simple health check endpoint
func handleHealth(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func main() {
	// Serve API routes
	http.HandleFunc("/api/analyze", handleAnalyze)
	http.HandleFunc("/health", handleHealth)

	// Serve embedded frontend (built from web/dist)
	uiFS, err := fs.Sub(embeddedUI, "web/dist")
	if err != nil {
		log.Fatal("Failed to load embedded UI:", err)
	}
	http.Handle("/", spaHandler{fs: http.FileServer(http.FS(uiFS)), static: uiFS})

	fmt.Printf("Repository Triage running on http://localhost%s\n", DefaultServerPort)

	if err := http.ListenAndServe(DefaultServerPort, nil); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}

// spaHandler serves static files and falls back to index.html for client-side routing
type spaHandler struct {
	fs     http.Handler
	static fs.FS
}

func (h spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Try serving the file directly
	path := r.URL.Path
	if path == "/" {
		path = "index.html"
	} else {
		path = strings.TrimPrefix(path, "/")
	}

	// If the file exists, serve it; otherwise fall back to index.html (SPA)
	if _, err := fs.Stat(h.static, path); err == nil {
		h.fs.ServeHTTP(w, r)
	} else {
		r.URL.Path = "/"
		h.fs.ServeHTTP(w, r)
	}
}
