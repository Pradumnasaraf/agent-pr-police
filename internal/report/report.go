package report

import (
	"fmt"
	"strings"

	"github.com/pradumnasaraf/agent-pr-police/internal/summary"
)

const Marker = "<!-- agent-pr-police:sticky-comment -->"

type Input struct {
	Agent   string
	Signals []string
	Summary summary.Summary
}

func Build(in Input) string {
	var b strings.Builder
	b.WriteString(Marker)
	b.WriteString("\n## Agent PR Police\n\n")

	who := "an AI coding agent"
	if in.Agent != "" {
		who = "the **" + in.Agent + "** coding agent"
	}
	fmt.Fprintf(&b, "This pull request was opened by %s.\n\n", who)

	s := in.Summary
	bd := breakdown(s)
	if bd == "" {
		bd = "no file changes"
	}
	b.WriteString("| Files changed | Lines | Breakdown |\n")
	b.WriteString("| :-- | :-- | :-- |\n")
	fmt.Fprintf(&b, "| %d | `+%d / -%d` | %s |\n", s.Files, s.Additions, s.Deletions, bd)

	if len(s.TopDirs) > 0 {
		fmt.Fprintf(&b, "\n**Top areas:** %s\n", topAreas(s.TopDirs))
	}

	if len(in.Signals) > 0 {
		b.WriteString("\n<details>\n<summary>Why this was flagged as an agent PR</summary>\n\n")
		for _, sig := range in.Signals {
			b.WriteString("- " + sig + "\n")
		}
		b.WriteString("\n</details>\n")
	}

	return b.String()
}

func topAreas(dirs []summary.DirCount) string {
	parts := make([]string, 0, len(dirs))
	for _, d := range dirs {
		parts = append(parts, fmt.Sprintf("`%s` (%d)", d.Dir, d.Files))
	}
	return strings.Join(parts, ", ")
}

func breakdown(s summary.Summary) string {
	var parts []string
	if s.Added > 0 {
		parts = append(parts, fmt.Sprintf("%d added", s.Added))
	}
	if s.Modified > 0 {
		parts = append(parts, fmt.Sprintf("%d modified", s.Modified))
	}
	if s.Removed > 0 {
		parts = append(parts, fmt.Sprintf("%d removed", s.Removed))
	}
	if s.Renamed > 0 {
		parts = append(parts, fmt.Sprintf("%d renamed", s.Renamed))
	}
	return strings.Join(parts, ", ")
}
