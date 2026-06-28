// Package sarif converts rule findings into a SARIF 2.1.0 document suitable
// for upload to the GitHub code-scanning / Security tab.
package sarif

import (
	"encoding/json"

	"github.com/pradumnasaraf/agent-pr-police/internal/rules"
)

const (
	toolName           = "agent-pr-police"
	informationURI     = "https://github.com/pradumnasaraf/agent-pr-police"
	schemaURI          = "https://json.schemastore.org/sarif-2.1.0.json"
	helpURIBase        = "https://github.com/pradumnasaraf/agent-pr-police#rules"
	sarifSchemaVersion = "2.1.0"
)

// Document is the root SARIF object.
type Document struct {
	Schema  string `json:"$schema"`
	Version string `json:"version"`
	Runs    []Run  `json:"runs"`
}

// Run is a single analysis run.
type Run struct {
	Tool    Tool     `json:"tool"`
	Results []Result `json:"results"`
}

// Tool describes the analyzer.
type Tool struct {
	Driver Driver `json:"driver"`
}

// Driver is the tool driver and its rule metadata.
type Driver struct {
	Name           string          `json:"name"`
	InformationURI string          `json:"informationUri"`
	Version        string          `json:"version"`
	Rules          []ReportingRule `json:"rules"`
}

// ReportingRule is rule metadata referenced by results.
type ReportingRule struct {
	ID                   string               `json:"id"`
	Name                 string               `json:"name"`
	ShortDescription     Message              `json:"shortDescription"`
	HelpURI              string               `json:"helpUri"`
	DefaultConfiguration DefaultConfiguration `json:"defaultConfiguration"`
}

// DefaultConfiguration carries the default severity level for a rule.
type DefaultConfiguration struct {
	Level string `json:"level"`
}

// Result is a single finding.
type Result struct {
	RuleID    string     `json:"ruleId"`
	RuleIndex int        `json:"ruleIndex"`
	Level     string     `json:"level"`
	Message   Message    `json:"message"`
	Locations []Location `json:"locations"`
}

// Message is a SARIF text message.
type Message struct {
	Text string `json:"text"`
}

// Location points to a place in a file.
type Location struct {
	PhysicalLocation PhysicalLocation `json:"physicalLocation"`
}

// PhysicalLocation is the artifact + region.
type PhysicalLocation struct {
	ArtifactLocation ArtifactLocation `json:"artifactLocation"`
	Region           *Region          `json:"region,omitempty"`
}

// ArtifactLocation is a file URI.
type ArtifactLocation struct {
	URI string `json:"uri"`
}

// Region is a line range.
type Region struct {
	StartLine int `json:"startLine"`
}

// levelFor maps a severity to a SARIF level.
func levelFor(s rules.Severity) string {
	switch s {
	case rules.SeverityHigh:
		return "error"
	case rules.SeverityMedium:
		return "warning"
	case rules.SeverityLow:
		return "note"
	default:
		return "none"
	}
}

// Build converts findings into a SARIF document. version is the tool version
// (e.g. a release tag or commit SHA).
func Build(findings []rules.Finding, version string) Document {
	if version == "" {
		version = "dev"
	}

	// Collect unique rules in first-seen order so results can reference them
	// by index.
	ruleIndex := map[string]int{}
	var reporting []ReportingRule
	for _, f := range findings {
		if _, ok := ruleIndex[f.RuleID]; ok {
			continue
		}
		ruleIndex[f.RuleID] = len(reporting)
		reporting = append(reporting, ReportingRule{
			ID:                   f.RuleID,
			Name:                 f.RuleID,
			ShortDescription:     Message{Text: f.Title},
			HelpURI:              helpURIBase,
			DefaultConfiguration: DefaultConfiguration{Level: levelFor(f.Severity)},
		})
	}

	results := make([]Result, 0, len(findings))
	for _, f := range findings {
		startLine := f.Line
		if startLine < 1 {
			startLine = 1
		}
		results = append(results, Result{
			RuleID:    f.RuleID,
			RuleIndex: ruleIndex[f.RuleID],
			Level:     levelFor(f.Severity),
			Message:   Message{Text: f.Message + " Fix: " + f.FixHint},
			Locations: []Location{{
				PhysicalLocation: PhysicalLocation{
					ArtifactLocation: ArtifactLocation{URI: f.File},
					Region:           &Region{StartLine: startLine},
				},
			}},
		})
	}

	return Document{
		Schema:  schemaURI,
		Version: sarifSchemaVersion,
		Runs: []Run{{
			Tool: Tool{Driver: Driver{
				Name:           toolName,
				InformationURI: informationURI,
				Version:        version,
				Rules:          reporting,
			}},
			Results: results,
		}},
	}
}

// Marshal renders the document as indented JSON.
func Marshal(doc Document) ([]byte, error) {
	return json.MarshalIndent(doc, "", "  ")
}
