// Command agent-pr-guard is the entrypoint binary run by the composite
// GitHub Action. It has two modes:
//
//	agent-pr-guard           run in GitHub Action / PR context (default)
//	agent-pr-guard scan PATH run the rules over local paths (CI self-test)
package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pradumnasaraf/agent-pr-guard/internal/detect"
	"github.com/pradumnasaraf/agent-pr-guard/internal/ghclient"
	"github.com/pradumnasaraf/agent-pr-guard/internal/report"
	"github.com/pradumnasaraf/agent-pr-guard/internal/rules"
	"github.com/pradumnasaraf/agent-pr-guard/internal/sarif"
)

// version is overridden at build time via -ldflags.
var version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "scan" {
		os.Exit(runScan(os.Args[2:]))
	}
	os.Exit(runAction())
}

// config holds resolved inputs.
type config struct {
	failOn            rules.Severity
	commentOnPR       bool
	label             string
	treatAll          bool
	extraIdentifiers  []string
	sensitivePaths    []string
	sensitivePathMode string
	sarifFile         string
}

func loadConfig() config {
	cfg := config{
		failOn:            rules.ParseSeverity(envOr("INPUT_FAIL_ON", "high")),
		commentOnPR:       envOr("INPUT_COMMENT_ON_PR", "true") == "true",
		label:             envOr("INPUT_LABEL", "agent"),
		treatAll:          envOr("INPUT_TREAT_ALL_PRS_AS_AGENT", "false") == "true",
		sensitivePathMode: envOr("INPUT_SENSITIVE_PATH_MODE", "block"),
		sarifFile:         envOr("INPUT_SARIF_FILE", "agent-pr-guard.sarif"),
	}
	cfg.extraIdentifiers = splitLines(os.Getenv("INPUT_EXTRA_AGENT_IDENTIFIERS"))
	if raw := strings.TrimSpace(os.Getenv("INPUT_SENSITIVE_PATHS")); raw != "" {
		cfg.sensitivePaths = splitLines(raw)
	} else {
		cfg.sensitivePaths = rules.DefaultSensitivePaths
	}
	if cfg.failOn == rules.SeverityNone && envOr("INPUT_FAIL_ON", "high") != "none" {
		cfg.failOn = rules.SeverityHigh
	}
	return cfg
}

func runAction() int {
	cfg := loadConfig()
	eventPath := os.Getenv("GITHUB_EVENT_PATH")
	if eventPath == "" {
		fmt.Fprintln(os.Stderr, "error: GITHUB_EVENT_PATH is not set; this binary expects a pull_request event")
		return 1
	}
	data, err := os.ReadFile(eventPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: reading event payload: %v\n", err)
		return 1
	}
	ev, err := detect.ParseEvent(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}

	client := ghclient.NewClient()

	// Enrich detection with commit messages (co-author trailers).
	if msgs, err := client.CommitMessages(ev.RepoOwner, ev.RepoName, ev.PRNumber); err == nil {
		ev.PR.CommitMessages = msgs
	} else {
		fmt.Fprintf(os.Stderr, "warning: could not fetch commits for detection: %v\n", err)
	}

	det := detect.Detect(ev.PR, detect.Config{
		Label:            cfg.label,
		TreatAllAsAgent:  cfg.treatAll,
		ExtraIdentifiers: cfg.extraIdentifiers,
	})

	files, err := client.ChangedFiles(ev.RepoOwner, ev.RepoName, ev.PRNumber, ev.HeadSHA)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: fetching changed files: %v\n", err)
		return 1
	}

	findings := rules.NewEngine(rules.Config{
		SensitivePaths:    cfg.sensitivePaths,
		SensitivePathMode: cfg.sensitivePathMode,
	}).Run(files)

	failed := rules.MaxSeverity(findings) >= cfg.failOn && cfg.failOn != rules.SeverityNone

	rep := report.Build(report.Input{
		IsAgent:  det.IsAgent,
		Agent:    det.Agent,
		Signals:  det.Signals,
		Findings: findings,
		FailOn:   cfg.failOn,
		Failed:   failed,
	})

	// Console output.
	fmt.Println(rep)

	// SARIF file.
	if err := writeSARIF(cfg.sarifFile, findings); err != nil {
		fmt.Fprintf(os.Stderr, "warning: writing SARIF: %v\n", err)
	}

	// Step summary.
	writeStepSummary(rep)

	// PR comment.
	if cfg.commentOnPR {
		if err := client.UpsertStickyComment(ev.RepoOwner, ev.RepoName, ev.PRNumber, report.Marker, rep); err != nil {
			fmt.Fprintf(os.Stderr, "warning: posting PR comment: %v\n", err)
		}
	}

	// Outputs.
	writeOutputs(det.IsAgent, findings)

	if failed {
		return 1
	}
	return 0
}

// runScan scans local paths and reports findings without any GitHub calls.
func runScan(paths []string) int {
	cfg := loadConfig()
	if len(paths) == 0 {
		fmt.Fprintln(os.Stderr, "usage: agent-pr-guard scan <path> [path...]")
		return 2
	}
	files, err := collectLocalFiles(paths)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
	}
	findings := rules.NewEngine(rules.Config{
		SensitivePaths:    cfg.sensitivePaths,
		SensitivePathMode: cfg.sensitivePathMode,
	}).Run(files)

	failed := rules.MaxSeverity(findings) >= cfg.failOn && cfg.failOn != rules.SeverityNone

	rep := report.Build(report.Input{
		IsAgent:  cfg.treatAll,
		Findings: findings,
		FailOn:   cfg.failOn,
		Failed:   failed,
	})
	fmt.Println(rep)

	if err := writeSARIF(cfg.sarifFile, findings); err != nil {
		fmt.Fprintf(os.Stderr, "warning: writing SARIF: %v\n", err)
	}

	if failed {
		return 1
	}
	return 0
}

// collectLocalFiles walks the provided paths and reads file content.
func collectLocalFiles(paths []string) ([]rules.ChangedFile, error) {
	var out []rules.ChangedFile
	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return nil, err
		}
		if info.IsDir() {
			err := filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() {
					if d.Name() == ".git" {
						return fs.SkipDir
					}
					return nil
				}
				out = append(out, readLocalFile(path))
				return nil
			})
			if err != nil {
				return nil, err
			}
			continue
		}
		out = append(out, readLocalFile(p))
	}
	return out, nil
}

func readLocalFile(path string) rules.ChangedFile {
	content, err := os.ReadFile(path)
	cf := rules.ChangedFile{Path: filepath.ToSlash(path), Status: "modified"}
	if err == nil {
		cf.Content = string(content)
	}
	return cf
}

func writeSARIF(path string, findings []rules.Finding) error {
	doc := sarif.Build(findings, version)
	data, err := sarif.Marshal(doc)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func writeStepSummary(rep string) {
	path := os.Getenv("GITHUB_STEP_SUMMARY")
	if path == "" {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintln(f, rep)
}

func writeOutputs(isAgent bool, findings []rules.Finding) {
	path := os.Getenv("GITHUB_OUTPUT")
	if path == "" {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return
	}
	defer f.Close()

	c := rules.Count(findings)
	fmt.Fprintf(f, "is-agent-pr=%s\n", strconv.FormatBool(isAgent))
	fmt.Fprintf(f, "high-count=%d\n", c.High)
	fmt.Fprintf(f, "medium-count=%d\n", c.Medium)
	fmt.Fprintf(f, "low-count=%d\n", c.Low)

	findingsJSON, _ := json.Marshal(findings)
	// Multiline-safe output using a random-ish delimiter.
	delim := "ghadelimiter_findings"
	fmt.Fprintf(f, "findings<<%s\n%s\n%s\n", delim, string(findingsJSON), delim)
}

func envOr(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

// splitLines splits a newline-separated value into trimmed, non-empty entries.
func splitLines(raw string) []string {
	var out []string
	for _, line := range strings.Split(raw, "\n") {
		if v := strings.TrimSpace(line); v != "" {
			out = append(out, v)
		}
	}
	return out
}
