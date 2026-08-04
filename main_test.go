package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCheckAuth(t *testing.T) {
	key := []byte("secret-token")

	cases := []struct {
		header string
		want   bool
	}{
		{"Bearer secret-token", true},
		{"Bearer wrong", false},
		{"secret-token", false},
		{"", false},
	}
	for _, tc := range cases {
		r := httptest.NewRequest(http.MethodPost, "/", nil)
		r.Header.Set("Authorization", tc.header)
		if got := checkAuth(r, key); got != tc.want {
			t.Errorf("header=%q: got %v, want %v", tc.header, got, tc.want)
		}
	}
}

func TestLiveness(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/liveness", nil)
	w := httptest.NewRecorder()
	handleLiveness(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, `"status"`) || !strings.Contains(body, `"version"`) {
		t.Errorf("unexpected body: %s", body)
	}
}

func TestBomResultsAuth(t *testing.T) {
	key := []byte("test-key")
	handler := handleBomResults(key)

	r := httptest.NewRequest(http.MethodPost, "/bom-results", strings.NewReader(`{}`))
	r.Header.Set("Authorization", "Bearer wrong")
	w := httptest.NewRecorder()
	handler(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestBomResultsMissingFields(t *testing.T) {
	key := []byte("test-key")
	handler := handleBomResults(key)

	body := `{"build_url":"http://ci.jenkins.io/job/foo/15/"}`
	r := httptest.NewRequest(http.MethodPost, "/bom-results", strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer test-key")
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestParseBuildURL(t *testing.T) {
	cases := []struct {
		url        string
		controller string
		jobName    string
		buildID    string
		wantErr    bool
	}{
		{
			url:        "http://ci.jenkins.io/job/foo/15/",
			controller: "ci.jenkins.io",
			jobName:    "foo",
			buildID:    "15",
		},
		{
			url:        "https://ci.jenkins.io/job/Plugins/job/bom/job/PR-1/153/",
			controller: "ci.jenkins.io",
			jobName:    "Plugins/bom/PR-1",
			buildID:    "153",
		},
		{
			url:        "https://server:8080/jenkins/job/my-job/42/",
			controller: "server",
			jobName:    "my-job",
			buildID:    "42",
		},
		{url: "not-a-url", wantErr: true},
		{url: "http://ci.jenkins.io/job/foo/", wantErr: true},
		{url: "http://ci.jenkins.io/notjob/foo/15/", wantErr: true},
	}
	for _, tc := range cases {
		controller, jobName, buildID, err := parseBuildURL(tc.url)
		if tc.wantErr {
			if err == nil {
				t.Errorf("%q: expected error, got none", tc.url)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q: unexpected error: %v", tc.url, err)
			continue
		}
		if controller != tc.controller || jobName != tc.jobName || buildID != tc.buildID {
			t.Errorf("%q: got (%s, %s, %s), want (%s, %s, %s)",
				tc.url, controller, jobName, buildID, tc.controller, tc.jobName, tc.buildID)
		}
	}
}

func TestValidateJobName(t *testing.T) {
	valid := []string{"Plugins/bom/PR-1", "my-job", "job_1", "a.b/c"}
	for _, s := range valid {
		if err := validateJobName(s); err != nil {
			t.Errorf("%q: unexpected error: %v", s, err)
		}
	}
	invalid := []string{"../etc/passwd", "job/../secret", "job name", "job;drop", ""}
	for _, s := range invalid {
		if err := validateJobName(s); err == nil {
			t.Errorf("%q: expected error, got none", s)
		}
	}
}

func TestValidateBuildID(t *testing.T) {
	valid := []string{"1", "21", "1000"}
	for _, s := range valid {
		if err := validateBuildID(s); err != nil {
			t.Errorf("%q: unexpected error: %v", s, err)
		}
	}
	invalid := []string{"abc", "1a", "1.0", "", "-1"}
	for _, s := range invalid {
		if err := validateBuildID(s); err == nil {
			t.Errorf("%q: expected error, got none", s)
		}
	}
}

func TestSecureHeaders(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	secureHeaders(inner).ServeHTTP(w, r)
	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("missing X-Content-Type-Options header")
	}
	if w.Header().Get("X-Frame-Options") != "DENY" {
		t.Error("missing X-Frame-Options header")
	}
}
