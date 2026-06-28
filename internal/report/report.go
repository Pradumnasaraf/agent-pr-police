// Package report renders human-readable Markdown summaries of findings for the
// sticky PR comment and the GitHub step summary.
//
// Markdown generation here is pure and testable; posting the comment to GitHub
// lives in the ghclient package.
package report

import (
	"fmt"
	"strings"

	"github.com/pradumnasaraf/agent-pr-police/internal/rules"
)

// Marker is the hidden HTML comment used to find and update the single sticky
// PR comment so the Action never posts duplicates.
const Marker = "<!-- agent-pr-police:sticky-comment -->"

// Input is everything needed to render a report.
type Input struct {
	// IsAgent indicates the PR was detected as Copilot-authored.
	IsAgent bool
	// Agent is the detected agent's display name (e.g. "Claude Code"); may be
	// empty when the PR was flagged without a specific named agent.
	Agent string
	// Signals are the detection reasons (empty for non-agent PRs).
	Signals []string
	// Findings are the policy findings, expected pre-sorted by severity.
	Findings []rules.Finding
	// FailOn is the threshold at or above which the check fails.
	FailOn rules.Severity
	// Failed indicates the overall check failed.
	Failed bool
}

// Build renders the full Markdown report, including the sticky marker.
func Build(in Input) string {
	var b strings.Builder
	b.WriteString(Marker)
	b.WriteString("\n## 🛡️ Agent PR Police\n\n")

	// Detection header.
	if in.IsAgent {
		author := "an AI coding agent"
		if in.Agent != "" {
			author = "the **" + in.Agent + "** coding agent"
		}
		b.WriteString("**This PR was detected as authored by " + author + ".** ")
		b.WriteString("Extra security policy was applied.\n\n")
		if len(in.Signals) > 0 {
			b.WriteString("<details><summary>Why this was flagged as an agent PR</summary>\n\n")
			for _, s := range in.Signals {
				b.WriteString("- " + s + "\n")
			}
			b.WriteString("\n</details>\n\n")
		}
	} else {
		b.WriteString("This PR was **not** detected as an AI-agent PR. ")
		b.WriteString("Standard policy checks were still run.\n\n")
	}

	counts := rules.Count(in.Findings)
	b.WriteString(statusLine(in, counts))
	b.WriteString("\n\n")

	if len(in.Findings) == 0 {
		b.WriteString("✅ No policy findings. Nothing to flag.\n")
		return b.String()
	}

	// Findings grouped by severity.
	writeGroup(&b, "🔴 High severity", in.Findings, rules.SeverityHigh)
	writeGroup(&b, "🟠 Medium severity", in.Findings, rules.SeverityMedium)
	writeGroup(&b, "🟡 Low severity", in.Findings, rules.SeverityLow)

	// Human checklist.
	b.WriteString("\n### 👀 What a human should double-check\n\n")
	for _, item := range humanChecklist(in.Findings) {
		b.WriteString("- [ ] " + item + "\n")
	}

	return b.String()
}

func statusLine(in Input, c rules.Counts) string {
	summary := fmt.Sprintf("**%d high, %d medium, %d low**", c.High, c.Medium, c.Low)
	if in.Failed {
		return fmt.Sprintf("❌ **Check failed.** Findings at or above `%s` are present.\n\n%s", in.FailOn, summary)
	}
	return fmt.Sprintf("✅ **Check passed.** No findings at or above `%s`.\n\n%s", in.FailOn, summary)
}

func writeGroup(b *strings.Builder, heading string, findings []rules.Finding, sev rules.Severity) {
	var group []rules.Finding
	for _, f := range findings {
		if f.Severity == sev {
			group = append(group, f)
		}
	}
	if len(group) == 0 {
		return
	}
	fmt.Fprintf(b, "### %s (%d)\n\n", heading, len(group))
	b.WriteString("| Location | Issue | How to fix |\n")
	b.WriteString("| --- | --- | --- |\n")
	for _, f := range group {
		loc := f.File
		if f.Line > 0 {
			loc = fmt.Sprintf("%s:%d", f.File, f.Line)
		}
		issue := fmt.Sprintf("**%s** `%s`<br>%s", f.Title, f.RuleID, cell(f.Message))
		fmt.Fprintf(b, "| `%s` | %s | %s |\n", cell(loc), issue, cell(f.FixHint))
	}
	b.WriteString("\n")
}

// cell escapes a value so it renders safely inside a Markdown table cell.
func cell(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}

// humanChecklist derives a short list of things a reviewer should verify based
// on which rule families fired.
func humanChecklist(findings []rules.Finding) []string {
	seen := map[string]bool{}
	var items []string
	add := func(key, text string) {
		if !seen[key] {
			seen[key] = true
			items = append(items, text)
		}
	}
	for _, f := range findings {
		switch {
		case strings.HasPrefix(f.RuleID, "docker/"):
			add("docker", "Confirm container changes are intentional and the image still runs as a non-root, pinned base.")
		case strings.HasPrefix(f.RuleID, "k8s/"):
			add("k8s", "Review the Kubernetes manifest changes for privilege, host access, and resource limits.")
		case strings.HasPrefix(f.RuleID, "sensitive-path/"):
			add("sensitive", "Verify edits to sensitive paths (workflows, infra, secrets) are expected from an automated agent.")
		case strings.HasPrefix(f.RuleID, "deps/"):
			add("deps", "Vet any newly added dependencies for provenance and necessity.")
		}
	}
	if len(items) == 0 {
		items = append(items, "Review the findings above and confirm they are acceptable.")
	}
	return items
}
