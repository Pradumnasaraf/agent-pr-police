package sarif

import (
	"encoding/json"
	"testing"

	"github.com/pradumnasaraf/agent-pr-police/internal/rules"
)

func TestBuild(t *testing.T) {
	findings := []rules.Finding{
		{RuleID: "docker/runs-as-root", Severity: rules.SeverityHigh, File: "Dockerfile", Line: 1, Title: "root", Message: "runs as root", FixHint: "add USER"},
		{RuleID: "docker/sudo-usage", Severity: rules.SeverityMedium, File: "Dockerfile", Line: 5, Title: "sudo", Message: "uses sudo", FixHint: "drop sudo"},
		{RuleID: "docker/runs-as-root", Severity: rules.SeverityHigh, File: "app/Dockerfile", Line: 2, Title: "root", Message: "runs as root", FixHint: "add USER"},
	}
	doc := Build(findings, "v1.2.3")

	if doc.Version != "2.1.0" {
		t.Errorf("version = %q, want 2.1.0", doc.Version)
	}
	if len(doc.Runs) != 1 {
		t.Fatalf("runs = %d, want 1", len(doc.Runs))
	}
	run := doc.Runs[0]
	if run.Tool.Driver.Version != "v1.2.3" {
		t.Errorf("driver version = %q", run.Tool.Driver.Version)
	}
	// Two unique rule ids despite three findings.
	if len(run.Tool.Driver.Rules) != 2 {
		t.Errorf("unique rules = %d, want 2", len(run.Tool.Driver.Rules))
	}
	if len(run.Results) != 3 {
		t.Errorf("results = %d, want 3", len(run.Results))
	}
	// ruleIndex must point at the matching reporting rule.
	for _, r := range run.Results {
		if r.RuleIndex < 0 || r.RuleIndex >= len(run.Tool.Driver.Rules) {
			t.Fatalf("ruleIndex %d out of range", r.RuleIndex)
		}
		if run.Tool.Driver.Rules[r.RuleIndex].ID != r.RuleID {
			t.Errorf("ruleIndex %d -> %q, but result ruleId is %q", r.RuleIndex, run.Tool.Driver.Rules[r.RuleIndex].ID, r.RuleID)
		}
	}
}

func TestBuildLevels(t *testing.T) {
	tests := []struct {
		sev   rules.Severity
		level string
	}{
		{rules.SeverityHigh, "error"},
		{rules.SeverityMedium, "warning"},
		{rules.SeverityLow, "note"},
	}
	for _, tt := range tests {
		doc := Build([]rules.Finding{{RuleID: "x", Severity: tt.sev, File: "f", Line: 1}}, "")
		if got := doc.Runs[0].Results[0].Level; got != tt.level {
			t.Errorf("severity %v -> level %q, want %q", tt.sev, got, tt.level)
		}
	}
}

func TestBuildClampsZeroLine(t *testing.T) {
	doc := Build([]rules.Finding{{RuleID: "x", Severity: rules.SeverityLow, File: "f", Line: 0}}, "")
	if got := doc.Runs[0].Results[0].Locations[0].PhysicalLocation.Region.StartLine; got != 1 {
		t.Errorf("startLine = %d, want clamped to 1", got)
	}
}

func TestMarshalIsValidJSON(t *testing.T) {
	doc := Build([]rules.Finding{{RuleID: "x", Severity: rules.SeverityHigh, File: "f", Line: 3, Message: "m", FixHint: "h"}}, "dev")
	data, err := Marshal(doc)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var back map[string]any
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if back["version"] != "2.1.0" {
		t.Errorf("round-trip version = %v", back["version"])
	}
}

func TestBuildEmptyFindings(t *testing.T) {
	doc := Build(nil, "dev")
	if len(doc.Runs[0].Results) != 0 {
		t.Errorf("expected 0 results")
	}
	if _, err := Marshal(doc); err != nil {
		t.Errorf("marshal empty: %v", err)
	}
}
