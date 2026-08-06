package pipeline

import (
	"encoding/json"
	"strings"
)

type BuildMetadata struct {
	Hash string
}

type FolderMetadata struct {
	Owner string
	Repo  string
}

// Raw Jenkins API response shapes — only the fields we need.

type buildAction struct {
	Class    string          `json:"_class"`
	Revision *buildRevision  `json:"revision,omitempty"`
	Build    *buildDataBuild `json:"build,omitempty"`
}

type buildRevision struct {
	Hash     *string `json:"hash"`
	PullHash *string `json:"pullHash"`
}

type buildDataBuild struct {
	Revision *buildDataRevision `json:"revision,omitempty"`
}

type buildDataRevision struct {
	SHA1 *string `json:"SHA1"`
}

type BuildAPIResponse struct {
	Actions []json.RawMessage `json:"actions"`
}

type folderSource struct {
	Source struct {
		RepoOwner  string `json:"repoOwner"`
		Repository string `json:"repository"`
	} `json:"source"`
}

type FolderAPIResponse struct {
	Sources []folderSource `json:"sources"`
}

func ProcessBuildMetadata(data []byte) BuildMetadata {
	var resp BuildAPIResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return BuildMetadata{}
	}

	for _, raw := range resp.Actions {
		var action buildAction
		if err := json.Unmarshal(raw, &action); err != nil {
			continue
		}
		if action.Class == "jenkins.scm.api.SCMRevisionAction" && action.Revision != nil {
			if action.Revision.Hash != nil && *action.Revision.Hash != "" {
				return BuildMetadata{Hash: *action.Revision.Hash}
			}
			if action.Revision.PullHash != nil && *action.Revision.PullHash != "" {
				return BuildMetadata{Hash: *action.Revision.PullHash}
			}
		}
		if action.Class == "hudson.plugins.git.util.BuildDetails" && action.Build != nil &&
			action.Build.Revision != nil && action.Build.Revision.SHA1 != nil {
			return BuildMetadata{Hash: *action.Build.Revision.SHA1}
		}
	}
	return BuildMetadata{}
}

// ProcessFolderMetadata extracts owner/repo from the Jenkins folder API response.
// If the folder has multiple SCM sources the loop takes the last one — matching
// the JS forEach behaviour. In practice pipelines have a single source.
func ProcessFolderMetadata(data []byte) FolderMetadata {
	var resp FolderAPIResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return FolderMetadata{}
	}
	var result FolderMetadata
	for _, s := range resp.Sources {
		result.Owner = s.Source.RepoOwner
		result.Repo = s.Source.Repository
	}
	return result
}

// GetBuildAPIURL returns the Jenkins API URL for build metadata.
// Note: the tree filter only requests SCMRevisionAction fields (revision[hash,pullHash]);
// it does not include build[revision[SHA1]] for hudson.plugins.git.util.BuildDetails.
// This mirrors the JS getBuildApiUrl behaviour — the BuildDetails fallback in
// ProcessBuildMetadata is therefore unreachable via this default URL.
func GetBuildAPIURL(buildURL string) string {
	return buildURL + "api/json?tree=actions[revision[hash,pullHash]]"
}

func GetFolderAPIURL(buildURL string) string {
	return buildURL + "../../../api/json?tree=sources[source[repoOwner,repository]]"
}

func GetArchiveURL(buildURL, hash string) string {
	shortHash := hash
	if len(shortHash) > 12 {
		shortHash = shortHash[:12]
	}
	// Escape 'a' and 'b' as glob patterns — see https://github.com/jenkinsci/incrementals-tools/pull/24
	versionPattern := strings.NewReplacer("a", "a*", "b", "b*").Replace(shortHash)
	return buildURL + "artifact/**/*" + versionPattern + "*/*" + versionPattern + "*/*zip*/archive.zip"
}
