package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

var testLog = slog.New(slog.NewTextHandler(io.Discard, nil))

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
	// We need the handler but can't construct a real ghClient in unit tests.
	// Test the validation layer via a direct HTTP test using a nil client;
	// the auth check happens before the client is used so we can fake the key.
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
