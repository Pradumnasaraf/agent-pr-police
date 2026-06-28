// Package ghclient is the GitHub-specific glue: it reads the event payload and
// talks to the REST API to fetch changed files, commit messages, and to manage
// the sticky PR comment. The reusable rule core does not depend on this package.
package ghclient

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/pradumnasaraf/agent-pr-guard/internal/rules"
)

// Client is a minimal GitHub REST client using only the standard library.
type Client struct {
	token  string
	apiURL string
	http   *http.Client
}

// NewClient builds a client from GITHUB_TOKEN and GITHUB_API_URL (falling back
// to the public API).
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
	defer resp.Body.Close()
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

// prFile mirrors an entry from the pulls/{n}/files endpoint.
type prFile struct {
	Filename string `json:"filename"`
	Status   string `json:"status"`
	SHA      string `json:"sha"`
}

// ChangedFiles returns the PR's changed files with their full content at the
// given head ref. Removed files are returned with empty content so path-based
// rules can still see them.
func (c *Client) ChangedFiles(owner, repo string, number int, headRef string) ([]rules.ChangedFile, error) {
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

	out := make([]rules.ChangedFile, 0, len(files))
	for _, f := range files {
		cf := rules.ChangedFile{Path: f.Filename, Status: f.Status}
		if f.Status != "removed" {
			content, err := c.fileContent(owner, repo, f.Filename, headRef)
			if err == nil {
				cf.Content = content
			}
			// A content fetch failure is non-fatal: path-based rules still run.
		}
		out = append(out, cf)
	}
	return out, nil
}

// contentResponse mirrors the contents API response.
type contentResponse struct {
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
}

func (c *Client) fileContent(owner, repo, path, ref string) (string, error) {
	apiPath := fmt.Sprintf("/repos/%s/%s/contents/%s", owner, repo, urlPathEscape(path))
	if ref != "" {
		apiPath += "?ref=" + url.QueryEscape(ref)
	}
	var cr contentResponse
	if err := c.do(http.MethodGet, apiPath, nil, &cr); err != nil {
		return "", err
	}
	if cr.Encoding == "base64" {
		decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(cr.Content, "\n", ""))
		if err != nil {
			return "", err
		}
		return string(decoded), nil
	}
	return cr.Content, nil
}

// commit mirrors an entry from the pulls/{n}/commits endpoint.
type commit struct {
	Commit struct {
		Message string `json:"message"`
	} `json:"commit"`
}

// CommitMessages returns the messages of all commits in the PR.
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

// Comment mirrors an issue comment.
type Comment struct {
	ID   int64  `json:"id"`
	Body string `json:"body"`
}

// FindSticky returns the id of the first comment containing marker, or 0.
func FindSticky(comments []Comment, marker string) int64 {
	for _, c := range comments {
		if strings.Contains(c.Body, marker) {
			return c.ID
		}
	}
	return 0
}

// UpsertStickyComment creates or updates the single sticky comment identified by
// marker.
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

// urlPathEscape escapes each path segment but keeps the slashes.
func urlPathEscape(p string) string {
	parts := strings.Split(p, "/")
	for i, seg := range parts {
		parts[i] = url.PathEscape(seg)
	}
	return strings.Join(parts, "/")
}
