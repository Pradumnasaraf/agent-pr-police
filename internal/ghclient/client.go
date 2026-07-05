package ghclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/pradumnasaraf/agent-pr-police/internal/summary"
)

type Client struct {
	token  string
	apiURL string
	http   *http.Client
}

func NewClient() *Client {
	api := os.Getenv("GITHUB_API_URL")
	if api == "" {
		api = "https://api.github.com"
	}
	return &Client{
		token:  os.Getenv("GITHUB_TOKEN"),
		apiURL: strings.TrimRight(api, "/"),
		http:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) do(method, path string, body io.Reader, out any) error {
	req, err := http.NewRequest(method, c.apiURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s %s: %s: %s", method, path, resp.Status, strings.TrimSpace(string(data)))
	}
	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("decoding %s %s: %w", method, path, err)
		}
	}
	return nil
}

type prFile struct {
	Filename  string `json:"filename"`
	Status    string `json:"status"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
}

func (c *Client) ChangedFiles(owner, repo string, number int) ([]summary.ChangedFile, error) {
	var files []prFile
	page := 1
	for {
		var pageFiles []prFile
		path := fmt.Sprintf("/repos/%s/%s/pulls/%d/files?per_page=100&page=%d", owner, repo, number, page)
		if err := c.do(http.MethodGet, path, nil, &pageFiles); err != nil {
			return nil, err
		}
		files = append(files, pageFiles...)
		if len(pageFiles) < 100 {
			break
		}
		page++
	}

	out := make([]summary.ChangedFile, 0, len(files))
	for _, f := range files {
		out = append(out, summary.ChangedFile{
			Path:      f.Filename,
			Status:    f.Status,
			Additions: f.Additions,
			Deletions: f.Deletions,
		})
	}
	return out, nil
}

type commit struct {
	Commit struct {
		Message string `json:"message"`
	} `json:"commit"`
}

func (c *Client) CommitMessages(owner, repo string, number int) ([]string, error) {
	var commits []commit
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d/commits?per_page=100", owner, repo, number)
	if err := c.do(http.MethodGet, path, nil, &commits); err != nil {
		return nil, err
	}
	msgs := make([]string, 0, len(commits))
	for _, cm := range commits {
		msgs = append(msgs, cm.Commit.Message)
	}
	return msgs, nil
}

func (c *Client) AddLabels(owner, repo string, number int, labels []string) error {
	payload, _ := json.Marshal(map[string][]string{"labels": labels})
	path := fmt.Sprintf("/repos/%s/%s/issues/%d/labels", owner, repo, number)
	return c.do(http.MethodPost, path, bytes.NewReader(payload), nil)
}

func (c *Client) RequestReviewers(owner, repo string, number int, users, teams []string) error {
	payload := map[string][]string{}
	if len(users) > 0 {
		payload["reviewers"] = users
	}
	if len(teams) > 0 {
		payload["team_reviewers"] = teams
	}
	if len(payload) == 0 {
		return nil
	}
	body, _ := json.Marshal(payload)
	path := fmt.Sprintf("/repos/%s/%s/pulls/%d/requested_reviewers", owner, repo, number)
	return c.do(http.MethodPost, path, bytes.NewReader(body), nil)
}

type Comment struct {
	ID   int64  `json:"id"`
	Body string `json:"body"`
}

func FindSticky(comments []Comment, marker string) int64 {
	for _, c := range comments {
		if strings.Contains(c.Body, marker) {
			return c.ID
		}
	}
	return 0
}

func (c *Client) UpsertStickyComment(owner, repo string, number int, marker, body string) error {
	var comments []Comment
	listPath := fmt.Sprintf("/repos/%s/%s/issues/%d/comments?per_page=100", owner, repo, number)
	if err := c.do(http.MethodGet, listPath, nil, &comments); err != nil {
		return err
	}
	payload, _ := json.Marshal(map[string]string{"body": body})
	if id := FindSticky(comments, marker); id != 0 {
		patchPath := fmt.Sprintf("/repos/%s/%s/issues/comments/%d", owner, repo, id)
		return c.do(http.MethodPatch, patchPath, bytes.NewReader(payload), nil)
	}
	postPath := fmt.Sprintf("/repos/%s/%s/issues/%d/comments", owner, repo, number)
	return c.do(http.MethodPost, postPath, bytes.NewReader(payload), nil)
}
