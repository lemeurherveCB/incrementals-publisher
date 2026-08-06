package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	gogithub "github.com/jenkins-infra/incrementals-publisher/internal/github"
)

var testLog = slog.New(slog.NewTextHandler(io.Discard, nil))

// stubGHClient satisfies the githubClient interface for unit tests.
type stubGHClient struct {
	commitExists    bool
	commitExistsErr error
	checkRunErr     error
}

func (s *stubGHClient) CommitExists(_ context.Context, _, _, _ string) (bool, error) {
	return s.commitExists, s.commitExistsErr
}
func (s *stubGHClient) CreateCheckRun(_ context.Context, _, _, _ string, _ []gogithub.ArtifactEntry) error {
	return s.checkRunErr
}

func TestValidateBuildURL(t *testing.T) {
	cases := []struct {
		url     string
		wantErr string
	}{
		{"https://ci.jenkins.io/job/Tools/job/bom/job/PR-22/5/", ""},
		{"https://example.com/foo/bar", "not supported"},
		{"https://ci.jenkins.io/job/../123/", "malformed"},
		{"https://ci.jenkins.io/job/./123/", "malformed"},
		{"https://ci.jenkins.io/job/ok/123//", "malformed"},
		{"https://ci.jenkins.io/job/hack?y/123/", "malformed"},
		{"https://ci.jenkins.io/job/hack#y/123/", "malformed"},
		{"https://ci.jenkins.io/job/hack%79/123/", "malformed"},
		{"https://ci.jenkins.io/junk/", "malformed"},
	}

	for _, tc := range cases {
		err := validateBuildURL(tc.url)
		if tc.wantErr == "" {
			if err != nil {
				t.Errorf("validateBuildURL(%q): unexpected error: %v", tc.url, err)
			}
		} else {
			if err == nil {
				t.Errorf("validateBuildURL(%q): expected error containing %q, got nil", tc.url, tc.wantErr)
			} else if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("validateBuildURL(%q): want %q in error, got %q", tc.url, tc.wantErr, err.Error())
			}
		}
	}
}

func TestLivenessHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/liveness", nil)
	w := httptest.NewRecorder()
	livenessHandler(testLog)(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "OK") {
		t.Errorf("expected 'OK' in body, got %q", w.Body.String())
	}
}

func TestPublishHandlerMissingBuildURL(t *testing.T) {
	t.Setenv("PRESHARED_KEY", "testkey")

	req := httptest.NewRequest(http.MethodPost, "/",
		strings.NewReader(`{"not_build_url":"value"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer testkey")

	w := httptest.NewRecorder()
	publishHandler(testLog, nil)(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "missing the build_url attribute") {
		t.Errorf("unexpected body: %q", w.Body.String())
	}
}

func TestPublishHandlerWrongHost(t *testing.T) {
	t.Setenv("PRESHARED_KEY", "testkey")

	req := httptest.NewRequest(http.MethodPost, "/",
		strings.NewReader(`{"build_url":"https://evil.example.com/job/x/1/"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer testkey")

	w := httptest.NewRecorder()
	publishHandler(testLog, nil)(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "not supported") {
		t.Errorf("unexpected body: %q", w.Body.String())
	}
}

func TestPublishHandlerUnauthorized(t *testing.T) {
	t.Setenv("PRESHARED_KEY", "correct-secret")

	req := httptest.NewRequest(http.MethodPost, "/",
		strings.NewReader(`{"build_url":"https://ci.jenkins.io/job/x/1/"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer wrong-secret")

	w := httptest.NewRecorder()
	publishHandler(testLog, nil)(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

// TestPublishHandlerPermissionsError covers the JS "error verifying permissions" test case:
// the permissions check fails → 400 with the expected message fragment.
func TestPublishHandlerPermissionsError(t *testing.T) {
	t.Setenv("PRESHARED_KEY", "testkey")

	buildMeta := map[string]any{
		"actions": []any{
			map[string]any{
				"_class": "jenkins.scm.api.SCMRevisionAction",
				"revision": map[string]any{
					"hash": "5055257e4d28adea76fc34fdde4e025347405bae",
				},
			},
		},
	}
	folderMeta := map[string]any{
		"sources": []any{
			map[string]any{
				"source": map[string]any{
					"repoOwner":  "jenkinsci",
					"repository": "bom",
				},
			},
		},
	}

	// Serve fake Jenkins endpoints and a permissions file that has no entry for jenkinsci/bom.
	jenkins := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "api/json") && strings.Contains(r.URL.RawQuery, "revision"):
			json.NewEncoder(w).Encode(buildMeta)
		case strings.Contains(r.URL.Path, "api/json"):
			json.NewEncoder(w).Encode(folderMeta)
		default:
			// Archive download — serve the good fixture.
			http.ServeFile(w, r, "../../test/fixtures-good-archive.zip")
		}
	}))
	defer jenkins.Close()

	permsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Empty permissions map → no entry for jenkinsci/bom.
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{}`))
	}))
	defer permsServer.Close()

	t.Setenv("JENKINS_HOST", jenkins.URL)
	t.Setenv("PERMISSIONS_URL", permsServer.URL)

	gh := &stubGHClient{commitExists: true}

	req := httptest.NewRequest(http.MethodPost, "/",
		strings.NewReader(`{"build_url":"`+jenkins.URL+`/job/Tools/job/bom/job/PR-22/5/"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer testkey")

	w := httptest.NewRecorder()
	publishHandler(testLog, gh)(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "Invalid archive") {
		t.Errorf("expected 'Invalid archive' in body, got %q", w.Body.String())
	}
}

// TestPublishHandlerSuccess covers the JS "success" test case:
// all checks pass, Artifactory returns 200 → response contains "Response from Artifactory".
func TestPublishHandlerSuccess(t *testing.T) {
	t.Setenv("PRESHARED_KEY", "testkey")

	buildMeta := map[string]any{
		"actions": []any{
			map[string]any{
				"_class": "jenkins.scm.api.SCMRevisionAction",
				"revision": map[string]any{
					"pullHash": "5055257e4d28adea76fc34fdde4e025347405bae",
				},
			},
		},
	}
	folderMeta := map[string]any{
		"sources": []any{
			map[string]any{
				"source": map[string]any{
					"repoOwner":  "jenkinsci",
					"repository": "bom",
				},
			},
		},
	}

	// Serve fake Jenkins endpoints.
	jenkins := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "api/json") && strings.Contains(r.URL.RawQuery, "revision"):
			json.NewEncoder(w).Encode(buildMeta)
		case strings.Contains(r.URL.Path, "api/json"):
			json.NewEncoder(w).Encode(folderMeta)
		default:
			http.ServeFile(w, r, "../../test/fixtures-good-archive.zip")
		}
	}))
	defer jenkins.Close()

	// Permissions server with jenkinsci/bom entry.
	permsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "../../test/fixtures-permissions.json")
	}))
	defer permsServer.Close()

	// Idempotency check → 404 (not yet deployed).
	// Artifactory upload → 200.
	artifactory := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Success"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer artifactory.Close()

	t.Setenv("JENKINS_HOST", jenkins.URL)
	t.Setenv("PERMISSIONS_URL", permsServer.URL)
	t.Setenv("INCREMENTAL_URL", artifactory.URL+"/")

	gh := &stubGHClient{commitExists: true}

	req := httptest.NewRequest(http.MethodPost, "/",
		strings.NewReader(`{"build_url":"`+jenkins.URL+`/job/Tools/job/bom/job/PR-22/5/"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer testkey")

	w := httptest.NewRecorder()
	publishHandler(testLog, gh)(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "Response from Artifactory") {
		t.Errorf("expected 'Response from Artifactory' in body, got %q", w.Body.String())
	}
}
