package report

import (
	"strings"
	"testing"

	"github.com/pradumnasaraf/agent-pr-police/internal/summary"
)

func sampleSummary() summary.Summary {
	return summary.Build([]summary.ChangedFile{
		{Path: "internal/detect/detect.go", Status: "modified", Additions: 10, Deletions: 2},
		{Path: "internal/report/report.go", Status: "added", Additions: 40, Deletions: 0},
		{Path: "README.md", Status: "modified", Additions: 5, Deletions: 3},
	})
}

func TestBuildStartsWithMarker(t *testing.T) {
	out := Build(Input{Agent: "Claude Code", Summary: sampleSummary(), StatsAvailable: true})
	if !strings.HasPrefix(out, Marker) {
		t.Errorf("report must start with sticky marker")
	}
}

func TestBuildNamedAgentAndSignals(t *testing.T) {
	out := Build(Input{
		Agent:          "Claude Code",
		Signals:        []string{"co-author trailer matched"},
		Summary:        sampleSummary(),
		StatsAvailable: true,
	})
	if !strings.Contains(out, "**Claude Code** coding agent") {
		t.Errorf("expected named agent header, got: %s", out)
	}
	if !strings.Contains(out, "co-author trailer matched") {
		t.Errorf("expected detection signal in output")
	}
}

func TestBuildUnnamedAgent(t *testing.T) {
	out := Build(Input{Summary: sampleSummary(), StatsAvailable: true})
	if !strings.Contains(out, "an AI coding agent") {
		t.Errorf("expected generic agent header when no name, got: %s", out)
	}
}

func TestBuildSummaryContent(t *testing.T) {
	out := Build(Input{Agent: "Devin", Summary: sampleSummary(), StatsAvailable: true})
	for _, want := range []string{"| 3 |", "+55 / -5", "1 added", "2 modified", "**Top areas:**", "`internal/` (2)", "`(root)` (1)"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected output to contain %q, got: %s", want, out)
		}
	}
}

func TestBuildNoEmoji(t *testing.T) {
	out := Build(Input{Agent: "Cursor", Summary: sampleSummary(), StatsAvailable: true})
	if strings.Contains(out, "🤖") {
		t.Errorf("comment should not contain emoji, got: %s", out)
	}
}

func TestBuildSingleFile(t *testing.T) {
	out := Build(Input{
		Agent:          "Cursor",
		Summary:        summary.Build([]summary.ChangedFile{{Path: "main.go", Status: "modified", Additions: 1, Deletions: 1}}),
		StatsAvailable: true,
	})
	if !strings.Contains(out, "| 1 |") {
		t.Errorf("expected single-file count in table, got: %s", out)
	}
}

func TestBuildStatsUnavailable(t *testing.T) {
	out := Build(Input{Agent: "Devin", StatsAvailable: false})
	if !strings.Contains(out, "Change stats were unavailable") {
		t.Errorf("expected unavailable note, got: %s", out)
	}
	if strings.Contains(out, "Files changed |") {
		t.Errorf("should not render the stats table when unavailable, got: %s", out)
	}
}

func TestBuildMentions(t *testing.T) {
	out := Build(Input{
		Agent:          "Devin",
		Summary:        sampleSummary(),
		StatsAvailable: true,
		Mentions:       []string{"alice", "org/security-team"},
	})
	if !strings.Contains(out, "cc @alice @org/security-team") {
		t.Errorf("expected cc line with mentions, got: %s", out)
	}
}

func TestBuildNoMentionsWhenEmpty(t *testing.T) {
	out := Build(Input{Agent: "Devin", Summary: sampleSummary(), StatsAvailable: true})
	if strings.Contains(out, "\ncc ") {
		t.Errorf("should not render cc line when no mentions, got: %s", out)
	}
}
