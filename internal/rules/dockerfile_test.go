package rules

import (
	"testing"
)

func TestIsDockerfile(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"Dockerfile", true},
		{"app/Dockerfile", true},
		{"Dockerfile.prod", true},
		{"build/Dockerfile.test", true},
		{"service.dockerfile", true},
		{"service.Dockerfile", true},
		{"main.go", false},
		{"docker-compose.yml", false},
		{"README.md", false},
	}
	for _, tt := range tests {
		if got := IsDockerfile(tt.path); got != tt.want {
			t.Errorf("IsDockerfile(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestSplitImageTag(t *testing.T) {
	tests := []struct {
		image    string
		wantName string
		wantTag  string
	}{
		{"alpine:3.19", "alpine", "3.19"},
		{"alpine", "alpine", ""},
		{"alpine:latest", "alpine", "latest"},
		{"library/nginx:1.25", "library/nginx", "1.25"},
		{"registry.io:5000/team/app:2.0", "registry.io:5000/team/app", "2.0"},
		{"registry.io:5000/team/app", "registry.io:5000/team/app", ""},
		{"alpine:3.19@sha256:abc", "alpine", "3.19"},
	}
	for _, tt := range tests {
		name, tag := splitImageTag(tt.image)
		if name != tt.wantName || tag != tt.wantTag {
			t.Errorf("splitImageTag(%q) = (%q, %q), want (%q, %q)", tt.image, name, tag, tt.wantName, tt.wantTag)
		}
	}
}

// findingByRule returns the first finding with the given rule id, or false.
func findingByRule(findings []Finding, ruleID string) (Finding, bool) {
	for _, f := range findings {
		if f.RuleID == ruleID {
			return f, true
		}
	}
	return Finding{}, false
}

func TestDockerfileRule(t *testing.T) {
	rule := NewDockerfileRule()

	tests := []struct {
		name        string
		content     string
		wantRule    string   // rule id expected to be present
		wantSev     Severity // expected severity of that rule (0 = don't care)
		wantAbsent  string   // rule id expected NOT to be present
		wantNoFinds bool     // expect zero findings overall
	}{
		{
			name:     "no USER directive runs as root",
			content:  "FROM alpine:3.19\nCMD [\"/bin/sh\"]\n",
			wantRule: "docker/runs-as-root",
			wantSev:  SeverityHigh,
		},
		{
			name:     "explicit USER root",
			content:  "FROM alpine:3.19\nUSER root\nCMD [\"/bin/sh\"]\n",
			wantRule: "docker/runs-as-root",
			wantSev:  SeverityHigh,
		},
		{
			name:       "non-root user is fine",
			content:    "FROM alpine:3.19@sha256:abc123\nRUN adduser -D app\nUSER app\nCMD [\"/bin/sh\"]\n",
			wantAbsent: "docker/runs-as-root",
		},
		{
			name:     "latest tag is high",
			content:  "FROM node:latest\nUSER app\n",
			wantRule: "docker/unpinned-base-image",
			wantSev:  SeverityHigh,
		},
		{
			name:     "untagged image is high",
			content:  "FROM ubuntu\nUSER app\n",
			wantRule: "docker/unpinned-base-image",
			wantSev:  SeverityHigh,
		},
		{
			name:     "tagged but no digest is medium",
			content:  "FROM golang:1.22\nUSER app\n",
			wantRule: "docker/unpinned-base-image",
			wantSev:  SeverityMedium,
		},
		{
			name:       "digest pinned is fine",
			content:    "FROM golang:1.22@sha256:deadbeef\nUSER app\n",
			wantAbsent: "docker/unpinned-base-image",
		},
		{
			name:       "multi-stage reference not flagged",
			content:    "FROM golang:1.22@sha256:deadbeef AS build\nFROM build\nUSER app\n",
			wantAbsent: "docker/unpinned-base-image",
		},
		{
			name:     "hardcoded secret in ENV",
			content:  "FROM alpine:3.19@sha256:abc\nENV API_KEY=sk_live_1234567890\nUSER app\n",
			wantRule: "docker/hardcoded-secret",
			wantSev:  SeverityHigh,
		},
		{
			name:       "ARG without value is not a secret",
			content:    "FROM alpine:3.19@sha256:abc\nARG GITHUB_TOKEN\nUSER app\n",
			wantAbsent: "docker/hardcoded-secret",
		},
		{
			name:       "secret from variable reference is not flagged",
			content:    "FROM alpine:3.19@sha256:abc\nENV PASSWORD=${DB_PASSWORD}\nUSER app\n",
			wantAbsent: "docker/hardcoded-secret",
		},
		{
			name:     "curl pipe to sh",
			content:  "FROM alpine:3.19@sha256:abc\nRUN curl -fsSL https://example.com/install.sh | sh\nUSER app\n",
			wantRule: "docker/remote-script-execution",
			wantSev:  SeverityHigh,
		},
		{
			name:     "wget pipe to bash",
			content:  "FROM alpine:3.19@sha256:abc\nRUN wget -qO- https://example.com/i.sh | bash\nUSER app\n",
			wantRule: "docker/remote-script-execution",
			wantSev:  SeverityHigh,
		},
		{
			name:     "ADD with remote URL",
			content:  "FROM alpine:3.19@sha256:abc\nADD https://example.com/app.tar.gz /app/\nUSER app\n",
			wantRule: "docker/add-remote-url",
			wantSev:  SeverityMedium,
		},
		{
			name:     "sudo usage",
			content:  "FROM alpine:3.19@sha256:abc\nRUN sudo apt-get update\nUSER app\n",
			wantRule: "docker/sudo-usage",
			wantSev:  SeverityMedium,
		},
		{
			name:     "whole context copy",
			content:  "FROM alpine:3.19@sha256:abc\nCOPY . .\nUSER app\n",
			wantRule: "docker/whole-context-copy",
			wantSev:  SeverityMedium,
		},
		{
			name:       "scoped copy is fine",
			content:    "FROM alpine:3.19@sha256:abc\nCOPY ./src /app/src\nUSER app\n",
			wantAbsent: "docker/whole-context-copy",
		},
		{
			name:        "fully hardened dockerfile has no findings",
			content:     "FROM alpine:3.19@sha256:abc\nRUN adduser -D app\nCOPY ./bin/app /app/app\nUSER app\nENTRYPOINT [\"/app/app\"]\n",
			wantNoFinds: true,
		},
		{
			name:     "line continuation is parsed",
			content:  "FROM alpine:3.19@sha256:abc\nRUN curl -fsSL https://x.sh \\\n    | sh\nUSER app\n",
			wantRule: "docker/remote-script-execution",
			wantSev:  SeverityHigh,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings := rule.Check(ChangedFile{Path: "Dockerfile", Content: tt.content})

			if tt.wantNoFinds && len(findings) != 0 {
				t.Fatalf("expected no findings, got %d: %+v", len(findings), findings)
			}
			if tt.wantRule != "" {
				f, ok := findingByRule(findings, tt.wantRule)
				if !ok {
					t.Fatalf("expected rule %q, got findings %+v", tt.wantRule, findings)
				}
				if tt.wantSev != SeverityNone && f.Severity != tt.wantSev {
					t.Errorf("rule %q severity = %v, want %v", tt.wantRule, f.Severity, tt.wantSev)
				}
				if f.Line == 0 {
					t.Errorf("rule %q reported line 0; expected a real line number", tt.wantRule)
				}
				if f.FixHint == "" || f.Message == "" {
					t.Errorf("rule %q missing message/fix hint: %+v", tt.wantRule, f)
				}
			}
			if tt.wantAbsent != "" {
				if _, ok := findingByRule(findings, tt.wantAbsent); ok {
					t.Errorf("did not expect rule %q, got findings %+v", tt.wantAbsent, findings)
				}
			}
		})
	}
}

func TestDockerfileRuleIgnoresNonDockerfiles(t *testing.T) {
	rule := NewDockerfileRule()
	findings := rule.Check(ChangedFile{Path: "main.go", Content: "FROM alpine:latest\n"})
	if len(findings) != 0 {
		t.Errorf("expected no findings for non-Dockerfile, got %+v", findings)
	}
}
