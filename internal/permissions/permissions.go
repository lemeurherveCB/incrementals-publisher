package permissions

import (
	"archive/zip"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gobwas/glob"
)

var githubSCMURLRe = regexp.MustCompile(`^https?://github[.]com/.+$`)

// Entry holds the Maven coordinates for a POM found in the archive.
type Entry struct {
	GroupID    string
	ArtifactID string
	Version    string
	Packaging  string
	Path       string
}

// FetchPermissions retrieves and parses the permissions index.
// The index maps "owner/repo" → []string{allowed Maven path prefixes}.
func FetchPermissions(url string) (map[string][]string, error) {
	resp, err := http.Get(url) //nolint:gosec // URL comes from config, not user input
	if err != nil {
		return nil, fmt.Errorf("fetching permissions: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("permissions server returned %d", resp.StatusCode)
	}
	var perms map[string][]string
	if err := json.NewDecoder(resp.Body).Decode(&perms); err != nil {
		return nil, fmt.Errorf("decoding permissions: %w", err)
	}
	return perms, nil
}

type pomProject struct {
	GroupID    string `xml:"groupId"`
	ArtifactID string `xml:"artifactId"`
	Version    string `xml:"version"`
	Packaging  string `xml:"packaging"`
	SCM        *struct {
		URL string `xml:"url"`
		Tag string `xml:"tag"`
	} `xml:"scm"`
}

// Verify reads the archive ZIP and validates every entry against the permissions
// index and (for .pom files) the POM metadata. Validated POM entries are appended
// to entries. Returns an error on the first violation.
func Verify(log *slog.Logger, repoPath, archivePath string, entries *[]Entry, perms map[string][]string, hash string) error {
	applicable, ok := perms[repoPath]
	if !ok {
		return fmt.Errorf("no applicable permissions for %s, check jenkins-infra/repository-permissions-updater has the right configuration", repoPath)
	}

	// Pre-compile glob matchers once per applicable pattern.
	type matcher struct {
		prefix string
		glob   glob.Glob
	}
	matchers := make([]matcher, 0, len(applicable))
	for _, pattern := range applicable {
		// Compile without a separator so '*' crosses '/' — matching the JS
		// wildcard-match behaviour (separator: '|' means '/' is not special).
		g, err := glob.Compile(pattern)
		if err != nil {
			return fmt.Errorf("compiling glob %q: %w", pattern, err)
		}
		matchers = append(matchers, matcher{prefix: pattern, glob: g})
	}

	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("opening archive: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		// Normalize the entry name and reject any path traversal (Zip Slip defence).
		// path.Clean collapses ".." and "." components; if the result still escapes
		// the root (starts with "..") or is absolute, reject it.
		cleaned := path.Clean(f.Name)
		if path.IsAbs(cleaned) || strings.HasPrefix(cleaned, "..") {
			return fmt.Errorf("path traversal detected in ZIP entry %q", f.Name)
		}
		name := cleaned

		// Skip directory entries.
		if strings.HasSuffix(f.Name, "/") {
			continue
		}

		allowed := false
		for _, m := range matchers {
			if m.glob.Match(name) || strings.HasPrefix(name, m.prefix) {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("no permissions for %s", name)
		}

		if strings.HasSuffix(name, ".pom") {
			entry, err := validatePOM(log, f, name, hash)
			if err != nil {
				return err
			}
			// Ensure the POM path matches the expected Maven coordinate path.
			expectedPath := filepath.ToSlash(
				strings.ReplaceAll(entry.GroupID, ".", "/") + "/" +
					entry.ArtifactID + "/" + entry.Version + "/" +
					entry.ArtifactID + "-" + entry.Version + ".pom",
			)
			if expectedPath != name {
				return fmt.Errorf("wrong GAV: %s vs. %s", expectedPath, name)
			}
			*entries = append(*entries, entry)
		}
	}
	return nil
}

func validatePOM(log *slog.Logger, f *zip.File, name, hash string) (Entry, error) {
	rc, err := f.Open()
	if err != nil {
		return Entry{}, fmt.Errorf("opening ZIP entry %q: %w", name, err)
	}
	defer rc.Close()

	raw, err := io.ReadAll(rc)
	if err != nil {
		return Entry{}, fmt.Errorf("reading ZIP entry %q: %w", name, err)
	}

	var pom pomProject
	if err := xml.Unmarshal(raw, &pom); err != nil {
		return Entry{}, fmt.Errorf("parsing POM %q: %w", name, err)
	}

	if pom.SCM == nil {
		return Entry{}, fmt.Errorf("missing <scm> section in %s", name)
	}
	if pom.SCM.URL == "" {
		return Entry{}, fmt.Errorf("missing <url> section in <scm> of %s", name)
	}
	if pom.SCM.Tag == "" {
		return Entry{}, fmt.Errorf("missing <tag> section in <scm> of %s", name)
	}

	if pom.SCM.Tag != hash {
		return Entry{}, fmt.Errorf("wrong commit hash in /project/scm/tag, expected %s, got %s", hash, pom.SCM.Tag)
	}
	if !githubSCMURLRe.MatchString(pom.SCM.URL) {
		return Entry{}, fmt.Errorf("wrong URL in /project/scm/url")
	}

	log.Info("parsed POM", "file", name, "url", pom.SCM.URL, "tag", pom.SCM.Tag,
		"groupId", pom.GroupID, "artifactId", pom.ArtifactID, "version", pom.Version)

	return Entry{
		GroupID:    pom.GroupID,
		ArtifactID: pom.ArtifactID,
		Version:    pom.Version,
		Packaging:  pom.Packaging,
		Path:       name,
	}, nil
}
