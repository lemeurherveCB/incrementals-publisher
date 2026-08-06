package github

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/bradleyfalzon/ghinstallation/v2"
	gogithub "github.com/google/go-github/v66/github"
)

// Client wraps the GitHub API for incrementals operations.
type Client struct {
	gh *gogithub.Client
}

// NewClient creates an authenticated GitHub App installation client.
func NewClient(appID, privateKey, installationID string) (*Client, error) {
	id, err := strconv.ParseInt(appID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parsing GITHUB_APP_ID: %w", err)
	}
	instID, err := strconv.ParseInt(installationID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parsing GITHUB_APP_INSTALLATION_ID: %w", err)
	}

	transport, err := ghinstallation.New(http.DefaultTransport, id, instID, []byte(privateKey))
	if err != nil {
		return nil, fmt.Errorf("creating GitHub App transport: %w", err)
	}

	httpClient := &http.Client{Transport: transport}
	gh := gogithub.NewClient(httpClient)
	return &Client{gh: gh}, nil
}

// CommitExists returns true when the given ref (commit SHA) is present in the repo.
func (c *Client) CommitExists(ctx context.Context, owner, repo, ref string) (bool, error) {
	commit, _, err := c.gh.Repositories.GetCommit(ctx, owner, repo, ref, nil)
	if err != nil {
		return false, fmt.Errorf("checking commit %s/%s@%s: %w", owner, repo, ref, err)
	}
	return commit != nil, nil
}

// ArtifactEntry carries the Maven coordinates for one published artifact.
type ArtifactEntry struct {
	ArtifactID string
	GroupID    string
	Version    string
	Packaging  string
	URL        string
}

// CreateCheckRun creates a GitHub Checks "Incrementals" run on the given commit.
func (c *Client) CreateCheckRun(ctx context.Context, owner, repo, headSHA string, entries []ArtifactEntry) error {
	if len(entries) == 0 {
		return fmt.Errorf("no entries to report")
	}

	opts := gogithub.CreateCheckRunOptions{
		Name:       "Incrementals",
		HeadSHA:    headSHA,
		Status:     ptr("completed"),
		Conclusion: ptr("success"),
		DetailsURL: ptr(entries[0].URL),
		Output: &gogithub.CheckRunOutput{
			Title:   ptr(fmt.Sprintf("Deployed version %s to Incrementals", entries[0].Version)),
			Summary: ptr(summary(entries)),
			Text:    ptr(text(entries)),
		},
	}
	_, _, err := c.gh.Checks.CreateCheckRun(ctx, owner, repo, opts)
	return err
}

func summary(entries []ArtifactEntry) string {
	plural := ""
	if len(entries) > 1 {
		plural = "s"
	}
	links := make([]string, 0, len(entries))
	for _, e := range entries {
		links = append(links, fmt.Sprintf("- [%s](%s)", e.ArtifactID, e.URL))
	}
	result := fmt.Sprintf("#### Download link%s:\n%s", plural, strings.Join(links, "\n"))

	var hpiCoords []string
	for _, e := range entries {
		if e.Packaging == "hpi" {
			hpiCoords = append(hpiCoords, fmt.Sprintf("%s:incrementals;%s;%s", e.ArtifactID, e.GroupID, e.Version))
		}
	}
	if len(hpiCoords) > 0 {
		result += "\n\n#### Plugin Installation Manager input format: ([documentation](https://github.com/jenkinsci/plugin-installation-manager-tool/#plugin-input-format))\n<pre>" +
			strings.Join(hpiCoords, "\n") + "</pre>"
	}
	return result
}

func text(entries []ArtifactEntry) string {
	parts := make([]string, 0, len(entries))
	for _, e := range entries {
		parts = append(parts, fmt.Sprintf(
			"&#60;dependency>&#xA;  &#60;groupId>%s&#60;/groupId>&#xA;  &#60;artifactId>%s&#60;/artifactId>&#xA;  &#60;version>%s&#60;/version>&#xA;&#60;/dependency>",
			e.GroupID, e.ArtifactID, e.Version,
		))
	}
	return "<pre>" + strings.Join(parts, "&#xA;") + "</pre>"
}

func ptr[T any](v T) *T { return &v }
