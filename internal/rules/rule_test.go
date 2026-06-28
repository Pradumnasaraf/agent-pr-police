package rules

import "testing"

func TestParseSeverity(t *testing.T) {
	tests := []struct {
		in   string
		want Severity
	}{
		{"low", SeverityLow},
		{"LOW", SeverityLow},
		{" medium ", SeverityMedium},
		{"high", SeverityHigh},
		{"none", SeverityNone},
		{"bogus", SeverityNone},
		{"", SeverityNone},
	}
	for _, tt := range tests {
		if got := ParseSeverity(tt.in); got != tt.want {
			t.Errorf("ParseSeverity(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestSeverityOrdering(t *testing.T) {
	if !(SeverityHigh > SeverityMedium && SeverityMedium > SeverityLow && SeverityLow > SeverityNone) {
		t.Fatal("severity ordering is wrong")
	}
}

func TestCountAndMaxSeverity(t *testing.T) {
	findings := []Finding{
		{Severity: SeverityHigh},
		{Severity: SeverityHigh},
		{Severity: SeverityMedium},
		{Severity: SeverityLow},
	}
	c := Count(findings)
	if c.High != 2 || c.Medium != 1 || c.Low != 1 {
		t.Errorf("Count = %+v, want {2 1 1}", c)
	}
	if MaxSeverity(findings) != SeverityHigh {
		t.Errorf("MaxSeverity = %v, want high", MaxSeverity(findings))
	}
	if MaxSeverity(nil) != SeverityNone {
		t.Errorf("MaxSeverity(nil) = %v, want none", MaxSeverity(nil))
	}
}

func TestSortFindings(t *testing.T) {
	findings := []Finding{
		{Severity: SeverityLow, File: "b", Line: 1},
		{Severity: SeverityHigh, File: "z", Line: 9},
		{Severity: SeverityHigh, File: "a", Line: 2},
		{Severity: SeverityMedium, File: "m", Line: 1},
	}
	SortFindings(findings)
	if findings[0].Severity != SeverityHigh || findings[0].File != "a" {
		t.Errorf("expected high/a first, got %+v", findings[0])
	}
	if findings[len(findings)-1].Severity != SeverityLow {
		t.Errorf("expected low last, got %+v", findings[len(findings)-1])
	}
}

func TestEngineRunSortsAndAggregates(t *testing.T) {
	eng := NewEngineWith(NewDockerfileRule())
	files := []ChangedFile{
		{Path: "Dockerfile", Content: "FROM node:latest\nADD https://x/y.tar.gz /y\n"},
	}
	findings := eng.Run(files)
	if len(findings) == 0 {
		t.Fatal("expected findings")
	}
	// Highest severity must sort first.
	if findings[0].Severity != SeverityHigh {
		t.Errorf("expected high severity first, got %v", findings[0].Severity)
	}
}
