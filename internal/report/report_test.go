package report

import (
	"strings"
	"testing"

	"github.com/pradumnasaraf/agent-pr-police/internal/rules"
)

func TestBuildContainsMarker(t *testing.T) {
	out := Build(Input{IsAgent: true, FailOn: rules.SeverityHigh})
	if !strings.HasPrefix(out, Marker) {
		t.Errorf("report must start with sticky marker")
	}
}

func TestBuildAgentHeader(t *testing.T) {
	named := Build(Input{IsAgent: true, Agent: "Claude Code", Signals: []string{"co-author trailer matched"}, FailOn: rules.SeverityHigh})
	if !strings.Contains(named, "**Claude Code** coding agent") {
		t.Errorf("expected named agent header, got: %s", named)
	}
	if !strings.Contains(named, "co-author trailer matched") {
		t.Errorf("expected detection signal in output")
	}

	unnamed := Build(Input{IsAgent: true, FailOn: rules.SeverityHigh})
	if !strings.Contains(unnamed, "an AI coding agent") {
		t.Errorf("expected generic agent header when no name, got: %s", unnamed)
	}

	human := Build(Input{IsAgent: false, FailOn: rules.SeverityHigh})
	if !strings.Contains(human, "**not** detected") {
		t.Errorf("expected non-agent header")
	}
}

func TestBuildNoFindings(t *testing.T) {
	out := Build(Input{IsAgent: true, FailOn: rules.SeverityHigh})
	if !strings.Contains(out, "No policy findings") {
		t.Errorf("expected clean message, got: %s", out)
	}
	if !strings.Contains(out, "Check passed") {
		t.Errorf("expected pass status")
	}
}

func TestBuildGroupsBySeverityAndFails(t *testing.T) {
	findings := []rules.Finding{
		{RuleID: "docker/runs-as-root", Severity: rules.SeverityHigh, File: "Dockerfile", Line: 1, Title: "root", Message: "why", FixHint: "fix"},
		{RuleID: "docker/sudo-usage", Severity: rules.SeverityMedium, File: "Dockerfile", Line: 3, Title: "sudo", Message: "why", FixHint: "fix"},
		{RuleID: "deps/new-dependency", Severity: rules.SeverityLow, File: "go.mod", Line: 5, Title: "dep", Message: "why", FixHint: "fix"},
	}
	out := Build(Input{IsAgent: true, Findings: findings, FailOn: rules.SeverityHigh, Failed: true})

	for _, want := range []string{"High severity", "Medium severity", "Low severity", "Check failed", "Dockerfile:1", "docker/runs-as-root", "What a human should double-check"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected output to contain %q", want)
		}
	}
	// High should appear before medium in the rendered output.
	if strings.Index(out, "High severity") > strings.Index(out, "Medium severity") {
		t.Errorf("high severity section should come first")
	}
}

func TestHumanChecklistDeduplicates(t *testing.T) {
	findings := []rules.Finding{
		{RuleID: "docker/runs-as-root", Severity: rules.SeverityHigh},
		{RuleID: "docker/sudo-usage", Severity: rules.SeverityMedium},
	}
	items := humanChecklist(findings)
	if len(items) != 1 {
		t.Errorf("expected 1 deduped docker checklist item, got %d: %v", len(items), items)
	}
}
