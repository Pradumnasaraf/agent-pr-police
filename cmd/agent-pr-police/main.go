package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode"

	"github.com/pradumnasaraf/agent-pr-police/internal/detect"
	"github.com/pradumnasaraf/agent-pr-police/internal/ghclient"
	"github.com/pradumnasaraf/agent-pr-police/internal/report"
	"github.com/pradumnasaraf/agent-pr-police/internal/summary"
)

type config struct {
	label            string
	addLabel         bool
	comment          bool
	treatAll         bool
	extraIdentifiers []string
	mention          []string
	requestReviewers []string
}

func main() {
	os.Exit(run())
}

func loadConfig() config {
	return config{
		label:            envOr("INPUT_LABEL", detect.DefaultLabel),
		addLabel:         envOr("INPUT_ADD_LABEL", "true") == "true",
		comment:          envOr("INPUT_COMMENT", "true") == "true",
		treatAll:         envOr("INPUT_TREAT_ALL_PRS_AS_AGENT", "false") == "true",
		extraIdentifiers: splitLines(os.Getenv("INPUT_EXTRA_AGENT_IDENTIFIERS")),
		mention:          splitList(os.Getenv("INPUT_MENTION")),
		requestReviewers: splitList(os.Getenv("INPUT_REQUEST_REVIEWERS")),
	}
}

func run() int {
	cfg := loadConfig()

	eventPath := os.Getenv("GITHUB_EVENT_PATH")
	if eventPath == "" {
		fmt.Fprintln(os.Stderr, "note: GITHUB_EVENT_PATH is not set; expected a pull_request event. Nothing to do.")
		return 0
	}
	data, err := os.ReadFile(eventPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "note: could not read event payload (%v). Nothing to do.\n", err)
		return 0
	}
	ev, err := detect.ParseEvent(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "note: %v Nothing to do.\n", err)
		return 0
	}

	client := ghclient.NewClient()

	if msgs, err := client.CommitMessages(ev.RepoOwner, ev.RepoName, ev.PRNumber); err == nil {
		ev.PR.CommitMessages = msgs
	} else {
		fmt.Fprintf(os.Stderr, "warning: could not fetch commits for detection: %v\n", err)
	}

	det := detect.Detect(ev.PR, detect.Config{
		Label:            cfg.label,
		TreatAllAsAgent:  cfg.treatAll,
		ExtraIdentifiers: cfg.extraIdentifiers,
	})

	writeOutputs(det)

	if !det.IsAgent {
		fmt.Println("Not an agent-authored PR; nothing to do.")
		return 0
	}

	var sum summary.Summary
	statsAvailable := true
	if files, err := client.ChangedFiles(ev.RepoOwner, ev.RepoName, ev.PRNumber); err == nil {
		sum = summary.Build(files)
	} else {
		fmt.Fprintf(os.Stderr, "warning: fetching changed files: %v\n", err)
		statsAvailable = false
	}

	mentions := dropAuthor(cfg.mention, ev.PR.AuthorLogin)

	rep := report.Build(report.Input{
		Agent:          det.Agent,
		Signals:        det.Signals,
		Summary:        sum,
		StatsAvailable: statsAvailable,
		Mentions:       mentions,
	})

	fmt.Println(rep)
	writeStepSummary(rep)

	if cfg.addLabel {
		if err := client.AddLabels(ev.RepoOwner, ev.RepoName, ev.PRNumber, []string{cfg.label}); err != nil {
			fmt.Fprintf(os.Stderr, "warning: adding label: %v\n", err)
		}
	}

	if cfg.comment {
		if err := client.UpsertStickyComment(ev.RepoOwner, ev.RepoName, ev.PRNumber, report.Marker, rep); err != nil {
			fmt.Fprintf(os.Stderr, "warning: posting PR comment: %v\n", err)
		}
	}

	requestReviews(client, ev, dropAuthor(cfg.requestReviewers, ev.PR.AuthorLogin))

	return 0
}

func requestReviews(client *ghclient.Client, ev *detect.Event, handles []string) {
	users, teams := parseReviewers(handles)
	for _, u := range users {
		if err := client.RequestReviewers(ev.RepoOwner, ev.RepoName, ev.PRNumber, []string{u}, nil); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not request review from %q: %v\n", u, err)
		}
	}
	for _, t := range teams {
		if err := client.RequestReviewers(ev.RepoOwner, ev.RepoName, ev.PRNumber, nil, []string{t}); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not request review from team %q: %v\n", t, err)
		}
	}
}

func writeOutputs(det detect.Result) {
	path := os.Getenv("GITHUB_OUTPUT")
	if path == "" {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()
	_, _ = fmt.Fprintf(f, "is-agent-pr=%s\n", strconv.FormatBool(det.IsAgent))
	_, _ = fmt.Fprintf(f, "agent=%s\n", det.Agent)
}

func writeStepSummary(rep string) {
	path := os.Getenv("GITHUB_STEP_SUMMARY")
	if path == "" {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()
	_, _ = fmt.Fprintln(f, rep)
}

func envOr(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func splitLines(raw string) []string {
	var out []string
	for line := range strings.SplitSeq(raw, "\n") {
		if v := strings.TrimSpace(line); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func splitList(raw string) []string {
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || unicode.IsSpace(r)
	})
	var out []string
	for _, f := range fields {
		if h := strings.TrimPrefix(strings.TrimSpace(f), "@"); h != "" {
			out = append(out, h)
		}
	}
	return out
}

func dropAuthor(handles []string, author string) []string {
	a := strings.ToLower(strings.TrimSpace(author))
	var out []string
	for _, h := range handles {
		if strings.ToLower(h) == a {
			continue
		}
		out = append(out, h)
	}
	return out
}

func parseReviewers(handles []string) (users, teams []string) {
	for _, h := range handles {
		if _, team, ok := strings.Cut(h, "/"); ok {
			if team != "" {
				teams = append(teams, team)
			}
			continue
		}
		users = append(users, h)
	}
	return users, teams
}
