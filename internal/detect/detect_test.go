package detect

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIdentifyLogin(t *testing.T) {
	tests := []struct {
		login string
		want  string // expected agent name ("" means not an agent)
	}{
		{"Copilot", "GitHub Copilot"},
		{"copilot-swe-agent[bot]", "GitHub Copilot"},
		{"github-copilot[bot]", "GitHub Copilot"},
		{"devin-ai-integration[bot]", "Devin"},
		{"cursoragent", "Cursor"},
		{"google-labs-jules[bot]", "Google Jules"},
		{"sweep-ai[bot]", "Sweep"},
		{"chatgpt-codex-connector[bot]", "OpenAI Codex"},
		// Human names that look like agent names must NOT match by login.
		{"devin", ""},
		{"jules", ""},
		{"claude", ""},
		{"octocat", ""},
		{"github-actions[bot]", ""},
		{"dependabot[bot]", ""},
		{"", ""},
	}
	for _, tt := range tests {
		if got := IdentifyLogin(tt.login); got != tt.want {
			t.Errorf("IdentifyLogin(%q) = %q, want %q", tt.login, got, tt.want)
		}
	}
}

func coauthor(name, email string) string {
	return "do a thing\n\nCo-authored-by: " + name + " <" + email + ">"
}

func TestDetect(t *testing.T) {
	tests := []struct {
		name      string
		pr        PullRequest
		cfg       Config
		wantAgent bool
		wantName  string // expected Result.Agent ("" = don't care)
	}{
		{
			name:      "copilot bot author",
			pr:        PullRequest{AuthorLogin: "copilot-swe-agent[bot]", AuthorType: "Bot"},
			wantAgent: true,
			wantName:  "GitHub Copilot",
		},
		{
			name:      "claude code via co-author trailer",
			pr:        PullRequest{AuthorLogin: "octocat", CommitMessages: []string{coauthor("Claude", "noreply@anthropic.com")}},
			wantAgent: true,
			wantName:  "Claude Code",
		},
		{
			name:      "devin via bot login",
			pr:        PullRequest{AuthorLogin: "devin-ai-integration[bot]"},
			wantAgent: true,
			wantName:  "Devin",
		},
		{
			name:      "cursor via co-author trailer",
			pr:        PullRequest{AuthorLogin: "octocat", CommitMessages: []string{coauthor("Cursor Agent", "agent@cursor.com")}},
			wantAgent: true,
			wantName:  "Cursor",
		},
		{
			name:      "aider via co-author trailer",
			pr:        PullRequest{AuthorLogin: "octocat", CommitMessages: []string{coauthor("aider", "aider@aider.chat")}},
			wantAgent: true,
			wantName:  "Aider",
		},
		{
			name:      "human author no signals",
			pr:        PullRequest{AuthorLogin: "octocat", AuthorType: "User"},
			wantAgent: false,
		},
		{
			name:      "human named devin is not an agent",
			pr:        PullRequest{AuthorLogin: "devin", CommitMessages: []string{"normal commit"}},
			wantAgent: false,
		},
		{
			name:      "default agent label",
			pr:        PullRequest{AuthorLogin: "octocat", Labels: []string{"bug", "agent"}},
			wantAgent: true,
		},
		{
			name:      "custom label match",
			pr:        PullRequest{AuthorLogin: "octocat", Labels: []string{"copilot"}},
			cfg:       Config{Label: "copilot"},
			wantAgent: true,
		},
		{
			name:      "non-agent co-author trailer",
			pr:        PullRequest{AuthorLogin: "octocat", CommitMessages: []string{coauthor("Jane Dev", "jane@example.com")}},
			wantAgent: false,
		},
		{
			name:      "extra identifier matches login",
			pr:        PullRequest{AuthorLogin: "acme-ai-bot[bot]"},
			cfg:       Config{ExtraIdentifiers: []string{"acme-ai"}},
			wantAgent: true,
		},
		{
			name:      "treat-all bypasses detection",
			pr:        PullRequest{AuthorLogin: "octocat"},
			cfg:       Config{TreatAllAsAgent: true},
			wantAgent: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Detect(tt.pr, tt.cfg)
			if got.IsAgent != tt.wantAgent {
				t.Errorf("Detect() IsAgent = %v, want %v (signals: %v)", got.IsAgent, tt.wantAgent, got.Signals)
			}
			if got.IsAgent && len(got.Signals) == 0 {
				t.Error("agent PR should have at least one signal")
			}
			if tt.wantName != "" && got.Agent != tt.wantName {
				t.Errorf("Detect() Agent = %q, want %q", got.Agent, tt.wantName)
			}
		})
	}
}

func TestParseEventFixtures(t *testing.T) {
	tests := []struct {
		file       string
		wantLogin  string
		wantNumber int
		wantOwner  string
		wantAgent  bool
	}{
		{"copilot_pr.json", "copilot-swe-agent[bot]", 42, "acme", true},
		{"human_pr.json", "octocat", 7, "acme", false},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata", tt.file))
			if err != nil {
				t.Fatalf("reading fixture: %v", err)
			}
			ev, err := ParseEvent(data)
			if err != nil {
				t.Fatalf("ParseEvent: %v", err)
			}
			if ev.PR.AuthorLogin != tt.wantLogin {
				t.Errorf("AuthorLogin = %q, want %q", ev.PR.AuthorLogin, tt.wantLogin)
			}
			if ev.PRNumber != tt.wantNumber {
				t.Errorf("PRNumber = %d, want %d", ev.PRNumber, tt.wantNumber)
			}
			if ev.RepoOwner != tt.wantOwner {
				t.Errorf("RepoOwner = %q, want %q", ev.RepoOwner, tt.wantOwner)
			}
			if got := Detect(ev.PR, Config{}); got.IsAgent != tt.wantAgent {
				t.Errorf("Detect from fixture = %v, want %v", got.IsAgent, tt.wantAgent)
			}
		})
	}
}

func TestParseEventRejectsNonPR(t *testing.T) {
	if _, err := ParseEvent([]byte(`{"action":"push"}`)); err == nil {
		t.Error("expected error for payload without pull_request")
	}
}
