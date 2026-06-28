// Package rules contains the security policy engine for agent-pr-police.
//
// This package is intentionally free of any GitHub-specific dependencies so
// that the rule core can be reused outside of this Action (for example, as a
// Copilot custom skill or an MCP server). Everything it needs is provided via
// the ChangedFile value type; callers are responsible for fetching file
// content and feeding it in.
package rules

import (
	"sort"
	"strings"
)

// Severity describes how serious a finding is. The zero value is SeverityNone.
type Severity int

const (
	// SeverityNone means "no finding" / used as a baseline for fail-on=none.
	SeverityNone Severity = iota
	// SeverityLow is informational; worth a human glance.
	SeverityLow
	// SeverityMedium is a real concern that should usually be addressed.
	SeverityMedium
	// SeverityHigh is a blocking-class issue for autonomous agent PRs.
	SeverityHigh
)

// ParseSeverity converts a string such as "high" into a Severity. Unknown
// values map to SeverityNone so that callers fail open to "report everything".
func ParseSeverity(s string) Severity {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "low":
		return SeverityLow
	case "medium":
		return SeverityMedium
	case "high":
		return SeverityHigh
	default:
		return SeverityNone
	}
}

// String returns the lowercase name of the severity.
func (s Severity) String() string {
	switch s {
	case SeverityLow:
		return "low"
	case SeverityMedium:
		return "medium"
	case SeverityHigh:
		return "high"
	default:
		return "none"
	}
}

// ChangedFile is the unit of input to the rule engine. It represents a single
// file that was added or modified in a pull request. Content is the full new
// content of the file (best-effort; may be empty for binary or deleted files).
type ChangedFile struct {
	// Path is the repository-relative path, using forward slashes.
	Path string
	// Content is the full new file content. Empty if unavailable.
	Content string
	// Status is the change status as reported by the source (added, modified,
	// removed, renamed). Optional; rules may use it but should not require it.
	Status string
}

// Lines splits the file content into lines (without trailing newlines).
func (f ChangedFile) Lines() []string {
	if f.Content == "" {
		return nil
	}
	return strings.Split(strings.ReplaceAll(f.Content, "\r\n", "\n"), "\n")
}

// Finding is a single policy violation discovered by a rule.
type Finding struct {
	// RuleID is a stable identifier such as "docker/runs-as-root".
	RuleID string `json:"ruleId"`
	// Severity is the seriousness of the finding.
	Severity Severity `json:"severity"`
	// File is the path the finding refers to.
	File string `json:"file"`
	// Line is the 1-based line number, or 0 if not line-specific.
	Line int `json:"line"`
	// Title is a short human-readable summary.
	Title string `json:"title"`
	// Message explains why the issue matters.
	Message string `json:"message"`
	// FixHint is a concrete suggestion for how to resolve it.
	FixHint string `json:"fixHint"`
}

// MarshalJSON renders Severity as its string form in JSON output.
func (s Severity) MarshalJSON() ([]byte, error) {
	return []byte(`"` + s.String() + `"`), nil
}

// DefaultSensitivePaths is the zero-config list of glob patterns treated as
// sensitive. Edits to these by an autonomous agent should get a human's eyes.
var DefaultSensitivePaths = []string{
	".github/workflows/**",
	"**/Dockerfile",
	"infra/**",
	"terraform/**",
	"**/*.tf",
	"**/secrets*",
	"**/*.pem",
	"**/auth/**",
}

// Config holds the tunable policy settings shared across rules.
type Config struct {
	// SensitivePaths is a list of glob patterns considered sensitive.
	SensitivePaths []string
	// SensitivePathMode is "block" (HIGH, fails the check) or "warn" (MEDIUM).
	SensitivePathMode string
}

// Rule inspects a single changed file and returns any findings. A rule that
// does not apply to a given file must return nil.
type Rule interface {
	// ID returns the rule family identifier (used for diagnostics/registry).
	ID() string
	// Check evaluates a single file and returns zero or more findings.
	Check(file ChangedFile) []Finding
}

// Engine runs a set of rules over a set of changed files.
type Engine struct {
	rules []Rule
}

// NewEngine builds an Engine with the default rule set, configured by cfg.
func NewEngine(cfg Config) *Engine {
	return &Engine{rules: DefaultRules(cfg)}
}

// NewEngineWith builds an Engine from an explicit rule list (useful in tests).
func NewEngineWith(rules ...Rule) *Engine {
	return &Engine{rules: rules}
}

// Run evaluates every rule against every file and returns all findings sorted
// by severity (high first), then file, then line.
func (e *Engine) Run(files []ChangedFile) []Finding {
	var findings []Finding
	for _, f := range files {
		for _, r := range e.rules {
			findings = append(findings, r.Check(f)...)
		}
	}
	SortFindings(findings)
	return findings
}

// SortFindings orders findings by descending severity, then path, then line.
func SortFindings(findings []Finding) {
	sort.SliceStable(findings, func(i, j int) bool {
		a, b := findings[i], findings[j]
		if a.Severity != b.Severity {
			return a.Severity > b.Severity
		}
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.RuleID < b.RuleID
	})
}

// Counts returns the number of findings at each severity level.
type Counts struct {
	High   int `json:"high"`
	Medium int `json:"medium"`
	Low    int `json:"low"`
}

// Count tallies findings by severity.
func Count(findings []Finding) Counts {
	var c Counts
	for _, f := range findings {
		switch f.Severity {
		case SeverityHigh:
			c.High++
		case SeverityMedium:
			c.Medium++
		case SeverityLow:
			c.Low++
		}
	}
	return c
}

// MaxSeverity returns the highest severity among the findings (SeverityNone if
// there are none).
func MaxSeverity(findings []Finding) Severity {
	max := SeverityNone
	for _, f := range findings {
		if f.Severity > max {
			max = f.Severity
		}
	}
	return max
}

// DefaultRules returns the standard rule set wired with cfg. Adding a new rule
// is as simple as appending it here.
func DefaultRules(cfg Config) []Rule {
	return []Rule{
		NewDockerfileRule(),
		// k8s, sensitive-path, and dependency rules are wired in Stage 2.
	}
}
