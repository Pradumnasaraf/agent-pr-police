package detect

import (
	"regexp"
	"strings"
)

const DefaultLabel = "pr-by-ai"

type PullRequest struct {
	AuthorLogin    string
	HeadRef        string
	Body           string
	Labels         []string
	CommitMessages []string
}

type Config struct {
	Label            string
	TreatAllAsAgent  bool
	ExtraIdentifiers []string
}

type Result struct {
	IsAgent bool
	Agent   string
	Signals []string
}

type Agent struct {
	Name            string
	LoginPatterns   []string
	TrailerPatterns []string
	BranchPatterns  []string
	BodyPatterns    []string
}

var Registry = []Agent{
	{
		Name:            "GitHub Copilot",
		LoginPatterns:   []string{"copilot-swe-agent", "github-copilot"},
		TrailerPatterns: []string{"copilot-swe-agent"},
		BranchPatterns:  []string{"copilot/"},
		BodyPatterns:    []string{"copilot coding agent"},
	},
	{
		Name:            "Claude Code",
		LoginPatterns:   []string{"claude-bot"},
		TrailerPatterns: []string{"anthropic"},
		BodyPatterns:    []string{"claude.com/claude-code", "claude.ai/code", "generated with claude code"},
	},
	{
		Name:            "Devin",
		LoginPatterns:   []string{"devin-ai-integration"},
		TrailerPatterns: []string{"devin", "cognition.ai"},
		BranchPatterns:  []string{"devin/"},
		BodyPatterns:    []string{"app.devin.ai"},
	},
	{
		Name:            "Cursor",
		LoginPatterns:   []string{"cursoragent", "cursor[bot]"},
		TrailerPatterns: []string{"cursoragent@cursor.com"},
		BranchPatterns:  []string{"cursor/"},
	},
	{
		Name:          "OpenAI Codex",
		LoginPatterns: []string{"chatgpt-codex", "codex-connector", "openai-codex"},
	},
	{
		Name:            "Aider",
		TrailerPatterns: []string{"aider@aider.chat"},
	},
	{
		Name:          "Google Jules",
		LoginPatterns: []string{"google-labs-jules"},
		BodyPatterns:  []string{"jules.google.com"},
	},
	{
		Name:            "Sourcegraph Amp",
		TrailerPatterns: []string{"ampcode.com"},
	},
	{
		Name:           "Sweep",
		LoginPatterns:  []string{"sweep-ai", "sweep-nightly"},
		BranchPatterns: []string{"sweep/"},
		BodyPatterns:   []string{"app.sweep.dev"},
	},
	{
		Name:           "Amazon Q",
		LoginPatterns:  []string{"amazon-q-developer"},
		BranchPatterns: []string{"q-dev-", "q-transform-"},
	},
	{
		Name:            "OpenHands",
		LoginPatterns:   []string{"openhands-agent"},
		TrailerPatterns: []string{"all-hands.dev"},
		BranchPatterns:  []string{"openhands-"},
		BodyPatterns:    []string{"all-hands.dev"},
	},
	{
		Name:          "Charlie",
		LoginPatterns: []string{"charliecreates"},
	},
	{
		Name:           "Ellipsis",
		LoginPatterns:  []string{"ellipsis-dev"},
		BranchPatterns: []string{"ellipsis/"},
		BodyPatterns:   []string{"ellipsis.dev"},
	},
	{
		Name:            "Factory",
		LoginPatterns:   []string{"factory-droid"},
		TrailerPatterns: []string{"factory-droid"},
	},
	{
		Name:            "Tembo",
		LoginPatterns:   []string{"tembo[bot]"},
		TrailerPatterns: []string{"tembo[bot]"},
		BranchPatterns:  []string{"tembo/"},
		BodyPatterns:    []string{"app.tembo.io"},
	},
	{
		Name:            "Zencoder",
		TrailerPatterns: []string{"zencoder.ai"},
		BodyPatterns:    []string{"zencoder.ai"},
	},
	{
		Name:           "Codegen",
		LoginPatterns:  []string{"codegen-sh"},
		BranchPatterns: []string{"codegen-bot/"},
		BodyPatterns:   []string{"codegen.com"},
	},
	{
		Name:           "v0",
		BranchPatterns: []string{"v0/main-"},
	},
}

var coAuthorRe = regexp.MustCompile(`(?im)^\s*co-authored-by:\s*(.+)$`)

func Detect(pr PullRequest, cfg Config) Result {
	var res Result

	if cfg.TreatAllAsAgent {
		res.IsAgent = true
		res.Signals = append(res.Signals, "treat-all-prs-as-agent is enabled")
	}

	if name, ok := matchLogin(pr.AuthorLogin, cfg.ExtraIdentifiers); ok {
		res.IsAgent = true
		setAgent(&res, name)
		res.Signals = append(res.Signals, "PR author login matches "+name+": "+pr.AuthorLogin)
	}

	label := cfg.Label
	if label == "" {
		label = DefaultLabel
	}
	for _, l := range pr.Labels {
		if strings.EqualFold(strings.TrimSpace(l), label) {
			res.IsAgent = true
			res.Signals = append(res.Signals, "PR carries the agent label: "+label)
			break
		}
	}

	if name, branch, ok := matchBranch(pr.HeadRef, cfg.ExtraIdentifiers); ok {
		res.IsAgent = true
		setAgent(&res, name)
		res.Signals = append(res.Signals, "the branch name matches "+name+": "+branch)
	}

	if name, trailer, ok := matchTrailers(pr.CommitMessages, cfg.ExtraIdentifiers); ok {
		res.IsAgent = true
		setAgent(&res, name)
		res.Signals = append(res.Signals, "a commit has an agent co-author trailer ("+name+"): "+trailer)
	}

	if name, marker, ok := matchBody(pr.Body); ok {
		res.IsAgent = true
		setAgent(&res, name)
		res.Signals = append(res.Signals, "the PR description contains an agent marker ("+name+"): "+marker)
	}

	return res
}

func setAgent(res *Result, name string) {
	if res.Agent == "" && name != "" && name != "custom agent" {
		res.Agent = name
	}
}

func matchLogin(login string, extra []string) (string, bool) {
	l := strings.ToLower(strings.TrimSpace(login))
	if l == "" {
		return "", false
	}
	for _, a := range Registry {
		for _, p := range a.LoginPatterns {
			if strings.Contains(l, strings.ToLower(p)) {
				return a.Name, true
			}
		}
	}
	for _, e := range extra {
		if e = strings.ToLower(strings.TrimSpace(e)); e != "" && strings.Contains(l, e) {
			return "custom agent", true
		}
	}
	return "", false
}

func matchTrailers(messages, extra []string) (name, trailer string, ok bool) {
	for _, m := range messages {
		for _, match := range coAuthorRe.FindAllStringSubmatch(m, -1) {
			value := strings.TrimSpace(match[1])
			lower := strings.ToLower(value)
			for _, a := range Registry {
				for _, p := range a.TrailerPatterns {
					if strings.Contains(lower, strings.ToLower(p)) {
						return a.Name, value, true
					}
				}
			}
			for _, e := range extra {
				if e = strings.ToLower(strings.TrimSpace(e)); e != "" && strings.Contains(lower, e) {
					return "custom agent", value, true
				}
			}
		}
	}
	return "", "", false
}

func matchBranch(branch string, extra []string) (name, matched string, ok bool) {
	b := strings.ToLower(strings.TrimSpace(branch))
	if b == "" {
		return "", "", false
	}
	for _, a := range Registry {
		for _, p := range a.BranchPatterns {
			if strings.HasPrefix(b, strings.ToLower(p)) {
				return a.Name, branch, true
			}
		}
	}
	for _, e := range extra {
		if e = strings.ToLower(strings.TrimSpace(e)); e != "" && strings.Contains(b, e) {
			return "custom agent", branch, true
		}
	}
	return "", "", false
}

func matchBody(body string) (name, marker string, ok bool) {
	b := strings.ToLower(body)
	if strings.TrimSpace(b) == "" {
		return "", "", false
	}
	for _, a := range Registry {
		for _, p := range a.BodyPatterns {
			if strings.Contains(b, strings.ToLower(p)) {
				return a.Name, p, true
			}
		}
	}
	return "", "", false
}

func IdentifyLogin(login string) string {
	name, _ := matchLogin(login, nil)
	return name
}
