// Package detect decides whether a pull request was authored by an AI coding
// agent (GitHub Copilot, Claude Code, Devin, Cursor, Codex, and others).
//
// The decision logic is pure: it operates on a PullRequest value populated by
// the caller (from the event payload and/or the GitHub API) and never performs
// I/O itself, which keeps it fully unit-testable with fixtures. Recognized
// agents live in a data-driven Registry so adding a new agent is trivial.
package detect

import (
	"regexp"
	"strings"
)

// PullRequest is the subset of PR metadata used for agent detection.
type PullRequest struct {
	// AuthorLogin is the PR author's login (e.g. "copilot-swe-agent[bot]").
	AuthorLogin string
	// AuthorType is the GitHub user type, typically "User" or "Bot".
	AuthorType string
	// Labels are the label names currently on the PR.
	Labels []string
	// CommitMessages are the full messages of the PR's commits, used to scan
	// for agent co-author trailers.
	CommitMessages []string
}

// Config tunes detection behavior.
type Config struct {
	// Label marks a PR as agent-authored (default "agent").
	Label string
	// TreatAllAsAgent bypasses detection and treats every PR as an agent PR.
	TreatAllAsAgent bool
	// ExtraIdentifiers are user-supplied substrings matched against both the
	// author login and co-author trailers, for agents not in the registry.
	ExtraIdentifiers []string
}

// Result is the outcome of detection.
type Result struct {
	// IsAgent is true if the PR is considered agent-authored.
	IsAgent bool
	// Agent is the best-guess agent name (e.g. "Claude Code"); empty if the PR
	// is not an agent PR or the agent could not be named.
	Agent string
	// Signals lists the human-readable reasons the decision was made.
	Signals []string
}

// Agent describes a known AI coding agent and how to recognize it.
type Agent struct {
	// Name is the display name, e.g. "GitHub Copilot".
	Name string
	// LoginPatterns are distinctive, case-insensitive substrings matched
	// against the PR author login. These are chosen to be unlikely to collide
	// with human usernames (full bot logins or distinctive tokens).
	LoginPatterns []string
	// TrailerPatterns are case-insensitive substrings matched against
	// Co-authored-by trailer values.
	TrailerPatterns []string
}

// Registry is the built-in set of recognized agents. Agents whose tooling
// commits under a human account (Claude Code, Aider) are detected by their
// co-author trailer rather than a login pattern, to avoid false positives on
// human names.
var Registry = []Agent{
	{
		Name:            "GitHub Copilot",
		LoginPatterns:   []string{"copilot"},
		TrailerPatterns: []string{"copilot"},
	},
	{
		Name:            "Claude Code",
		LoginPatterns:   []string{"claude-bot", "anthropic"},
		TrailerPatterns: []string{"claude", "anthropic"},
	},
	{
		Name:            "Devin",
		LoginPatterns:   []string{"devin-ai-integration", "devin-ai"},
		TrailerPatterns: []string{"devin"},
	},
	{
		Name:            "Cursor",
		LoginPatterns:   []string{"cursoragent", "cursor-com", "cursor[bot]"},
		TrailerPatterns: []string{"cursor"},
	},
	{
		Name:            "OpenAI Codex",
		LoginPatterns:   []string{"chatgpt-codex", "codex-connector", "openai-codex"},
		TrailerPatterns: []string{"codex"},
	},
	{
		Name:            "Aider",
		LoginPatterns:   nil,
		TrailerPatterns: []string{"aider"},
	},
	{
		Name:            "Google Jules",
		LoginPatterns:   []string{"google-labs-jules"},
		TrailerPatterns: []string{"jules"},
	},
	{
		Name:            "Sourcegraph Cody",
		LoginPatterns:   []string{"sourcegraph-cody", "sourcegraph-bot"},
		TrailerPatterns: []string{"sourcegraph cody"},
	},
	{
		Name:            "Sweep",
		LoginPatterns:   []string{"sweep-ai"},
		TrailerPatterns: []string{"sweep-ai"},
	},
}

// coAuthorRe matches a "Co-authored-by:" trailer line.
var coAuthorRe = regexp.MustCompile(`(?im)^\s*co-authored-by:\s*(.+)$`)

// Detect applies all detection signals. Any single matching signal classifies
// the PR as agent-authored.
func Detect(pr PullRequest, cfg Config) Result {
	var res Result

	if cfg.TreatAllAsAgent {
		res.IsAgent = true
		res.Signals = append(res.Signals, "treat-all-prs-as-agent is enabled")
	}

	// Author login.
	if name, ok := matchLogin(pr.AuthorLogin, cfg.ExtraIdentifiers); ok {
		res.IsAgent = true
		setAgent(&res, name)
		res.Signals = append(res.Signals, "PR author login matches "+name+": "+pr.AuthorLogin)
	}

	// Agent label.
	label := cfg.Label
	if label == "" {
		label = "agent"
	}
	for _, l := range pr.Labels {
		if strings.EqualFold(strings.TrimSpace(l), label) {
			res.IsAgent = true
			res.Signals = append(res.Signals, "PR carries the agent label: "+label)
			break
		}
	}

	// Co-author trailers.
	if name, trailer, ok := matchTrailers(pr.CommitMessages, cfg.ExtraIdentifiers); ok {
		res.IsAgent = true
		setAgent(&res, name)
		res.Signals = append(res.Signals, "a commit has an agent co-author trailer ("+name+"): "+trailer)
	}

	return res
}

// setAgent records the first concretely-named agent.
func setAgent(res *Result, name string) {
	if res.Agent == "" && name != "" && name != "custom agent" {
		res.Agent = name
	}
}

// matchLogin reports whether a login matches a known agent or an extra
// identifier, returning the agent name.
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

// matchTrailers scans Co-authored-by trailers for an agent identifier and
// returns the agent name and the matched trailer.
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

// IdentifyLogin returns the agent name for a login, or "" if not an agent. It is
// exported for callers that only want the login signal.
func IdentifyLogin(login string) string {
	name, _ := matchLogin(login, nil)
	return name
}
