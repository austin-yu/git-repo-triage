package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// --- Constants ---

const (
	TimePeriod          = "1 year ago"
	GitListLimit        = 30
	CouplingAlertsLimit = 5
	AnalysisTaskCount   = 4
	DefaultServerPort   = ":8080"
)

// File patterns to exclude from analysis
var (
	ExcludePatterns = []string{
		"node_modules",
		"go.sum",
		"yarn.lock",
		"vendor",
		"dist",
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

type RepoReport struct {
	RiskMatrix    []FileRisk    `json:"riskMatrix"`
	BusFactor     []Contributor `json:"busFactor"`
	Firefighting  int           `json:"firefightingIncidents"`
	CouplingAlert []string      `json:"couplingAlerts"`
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
		"git -C %s shortlog -sn --no-merges --since='%s'",
		repoPath, TimePeriod,
	)

	raw, err := executor.Run(cmd)
	if err != nil {
		return nil, fmt.Errorf("contributor analysis failed: %w", err)
	}

	var contributors []Contributor
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

// analyzeCoupling extracts the Architectural Blast Radius data
func analyzeCoupling(executor CommandExecutor, repoPath string) ([]string, error) {
	cmd := fmt.Sprintf(
		"git -C %s log --shortstat --oneline --since='%s' --no-merges | "+
			"grep -E 'files? changed' | sort -nr -k 5 | head -%d",
		repoPath, TimePeriod, CouplingAlertsLimit,
	)

	raw, err := executor.Run(cmd)
	if err != nil {
		return nil, fmt.Errorf("coupling analysis failed: %w", err)
	}

	if raw == "" {
		return []string{}, nil
	}

	return strings.Split(raw, "\n"), nil
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

	// Result channels for concurrent tasks
	churnChan := make(chan map[string]int)
	bugChan := make(chan map[string]int)
	contributorChan := make(chan []Contributor)
	firefightingChan := make(chan int)
	couplingChan := make(chan []string)

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

	wg.Wait()

	if firstErr != nil {
		return nil, fmt.Errorf("analysis failed: %w", firstErr)
	}

	// Collect results
	churnData := <-churnChan
	bugData := <-bugChan
	report.BusFactor = <-contributorChan
	report.Firefighting = <-firefightingChan
	report.CouplingAlert = <-couplingChan

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
	http.HandleFunc("/api/analyze", handleAnalyze)
	http.HandleFunc("/health", handleHealth)

	fmt.Printf("Repository Triage Server running on http://localhost%s\n", DefaultServerPort)
	fmt.Printf("  Endpoints:\n")
	fmt.Printf("    GET /api/analyze?path=/path/to/repo - Analyze a repository\n")
	fmt.Printf("    GET /health                         - Health check\n")

	if err := http.ListenAndServe(DefaultServerPort, nil); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}
