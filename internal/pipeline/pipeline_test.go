package pipeline_test

import (
	"testing"

	"github.com/jenkins-infra/incrementals-publisher/internal/pipeline"
)

func TestProcessBuildMetadataSCMRevisionAction(t *testing.T) {
	data := []byte(`{
		"_class": "org.jenkinsci.plugins.workflow.job.WorkflowRun",
		"actions": [
			{"_class": "hudson.model.CauseAction"},
			{
				"_class": "jenkins.scm.api.SCMRevisionAction",
				"revision": {
					"_class": "jenkins.plugins.git.AbstractGitSCMSource$SCMRevisionImpl",
					"hash": "abc131cc3bf56309a05b3fe8b086b265d14f2a61"
				}
			},
			{"_class": "hudson.plugins.git.util.BuildData"}
		]
	}`)
	got := pipeline.ProcessBuildMetadata(data)
	if got.Hash != "abc131cc3bf56309a05b3fe8b086b265d14f2a61" {
		t.Errorf("expected hash abc131..., got %q", got.Hash)
	}
}

func TestProcessBuildMetadataPullHash(t *testing.T) {
	data := []byte(`{
		"actions": [{
			"_class": "jenkins.scm.api.SCMRevisionAction",
			"revision": {
				"_class": "org.jenkinsci.plugins.github_branch_source.PullRequestSCMRevision",
				"pullHash": "5055257e4d28adea76fc34fdde4e025347405bae"
			}
		}]
	}`)
	got := pipeline.ProcessBuildMetadata(data)
	if got.Hash != "5055257e4d28adea76fc34fdde4e025347405bae" {
		t.Errorf("expected pullHash, got %q", got.Hash)
	}
}

func TestProcessBuildMetadataNoHash(t *testing.T) {
	data := []byte(`{
		"_class": "org.jenkinsci.plugins.workflow.job.WorkflowRun",
		"actions": [
			{"_class": "hudson.model.CauseAction"},
			{
				"_class": "jenkins.scm.api.SCMRevisionAction",
				"revision": {"_class": "jenkins.plugins.git.AbstractGitSCMSource$SCMRevisionImpl"}
			},
			{"_class": "hudson.plugins.git.util.BuildData"}
		]
	}`)
	got := pipeline.ProcessBuildMetadata(data)
	if got.Hash != "" {
		t.Errorf("expected empty hash, got %q", got.Hash)
	}
}

func TestProcessBuildMetadataEmpty(t *testing.T) {
	got := pipeline.ProcessBuildMetadata([]byte(`{}`))
	if got.Hash != "" {
		t.Errorf("expected empty hash, got %q", got.Hash)
	}
}

func TestGetBuildAPIURL(t *testing.T) {
	buildURL := "https://ci.jenkins.io/job/structs-plugin/job/PR-36/3/"
	got := pipeline.GetBuildAPIURL(buildURL)
	if got != buildURL+"api/json?tree=actions[revision[hash,pullHash]]" {
		t.Errorf("unexpected URL: %q", got)
	}
}

func TestGetArchiveURL(t *testing.T) {
	buildURL := "https://ci.jenkins.io/job/structs-plugin/job/PR-36/3/"
	got := pipeline.GetArchiveURL(buildURL, "acbd4")
	want := "https://ci.jenkins.io/job/structs-plugin/job/PR-36/3/artifact/**/*a*cb*d4*/*a*cb*d4*/*zip*/archive.zip"
	if got != want {
		t.Errorf("expected %q\ngot      %q", want, got)
	}
}

func TestGetArchiveURLTruncates(t *testing.T) {
	buildURL := "https://ci.jenkins.io/job/x/1/"
	hash := "abc123def456789"
	got := pipeline.GetArchiveURL(buildURL, hash)
	// short hash = first 12 chars = "abc123def456"
	// after escaping: a→a*, b→b* → "a*b*c123def456"
	want := "https://ci.jenkins.io/job/x/1/artifact/**/*a*b*c123def456*/*a*b*c123def456*/*zip*/archive.zip"
	if got != want {
		t.Errorf("expected %q\ngot      %q", want, got)
	}
}
