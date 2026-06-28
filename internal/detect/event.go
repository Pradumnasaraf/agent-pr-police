package detect

import (
	"encoding/json"
	"fmt"
)

// Event is the parsed subset of a GitHub pull_request event payload that the
// Action needs.
type Event struct {
	Action    string
	PRNumber  int
	RepoOwner string
	RepoName  string
	HeadSHA   string
	// PR carries the author and labels parsed from the payload. CommitMessages
	// is left empty here and is populated separately from the API.
	PR PullRequest
}

// rawEvent mirrors the relevant fields of the pull_request webhook payload.
type rawEvent struct {
	Action      string `json:"action"`
	PullRequest struct {
		Number int `json:"number"`
		User   struct {
			Login string `json:"login"`
			Type  string `json:"type"`
		} `json:"user"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
		Head struct {
			SHA string `json:"sha"`
		} `json:"head"`
	} `json:"pull_request"`
	Repository struct {
		Name  string `json:"name"`
		Owner struct {
			Login string `json:"login"`
		} `json:"owner"`
	} `json:"repository"`
}

// ParseEvent reads a pull_request event payload into an Event.
func ParseEvent(data []byte) (*Event, error) {
	var raw rawEvent
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing event payload: %w", err)
	}
	if raw.PullRequest.Number == 0 {
		return nil, fmt.Errorf("event payload has no pull_request (is this a pull_request event?)")
	}
	labels := make([]string, 0, len(raw.PullRequest.Labels))
	for _, l := range raw.PullRequest.Labels {
		labels = append(labels, l.Name)
	}
	return &Event{
		Action:    raw.Action,
		PRNumber:  raw.PullRequest.Number,
		RepoOwner: raw.Repository.Owner.Login,
		RepoName:  raw.Repository.Name,
		HeadSHA:   raw.PullRequest.Head.SHA,
		PR: PullRequest{
			AuthorLogin: raw.PullRequest.User.Login,
			AuthorType:  raw.PullRequest.User.Type,
			Labels:      labels,
		},
	}, nil
}
