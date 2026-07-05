package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

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
	}
}

func run() int {
	cfg := loadConfig()

	eventPath := os.Getenv("GITHUB_EVENT_PATH")
	if eventPath == "" {
		fmt.Fprintln(os.Stderr, "error: GITHUB_EVENT_PATH is not set; this action expects a pull_request event")
		return 1
	}
	data, err := os.ReadFile(eventPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: reading event payload: %v\n", err)
		return 1
	}
	ev, err := detect.ParseEvent(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 1
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

	files, err := client.ChangedFiles(ev.RepoOwner, ev.RepoName, ev.PRNumber)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: fetching changed files: %v\n", err)
		return 1
	}

	rep := report.Build(report.Input{
		Agent:   det.Agent,
		Signals: det.Signals,
		Summary: summary.Build(files),
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

	return 0
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
