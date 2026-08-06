package permissions_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/jenkins-infra/incrementals-publisher/internal/permissions"
)

var discardLog = slog.New(slog.NewTextHandler(io.Discard, nil))

func readJSON(t *testing.T, path string) map[string][]string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	var m map[string][]string
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
	return m
}

func TestFailsBadSCMURL(t *testing.T) {
	perms := readJSON(t, "../../test/fixtures-permissions.json")
	var entries []permissions.Entry
	err := permissions.Verify(
		discardLog,
		"jenkinsci/bom",
		"../../test/fixtures-bad-scm-url-archive.zip",
		&entries,
		perms,
		"149af85f094da863ddc294e50b5d8caaab549f95",
	)
	if err == nil {
		t.Fatal("expected error for bad SCM URL, got nil")
	}
}

func TestFailsMissingPermissions(t *testing.T) {
	perms := readJSON(t, "../../test/fixtures-permissions-missing-path.json")
	var entries []permissions.Entry
	err := permissions.Verify(
		discardLog,
		"jenkinsci/bom",
		"../../test/fixtures-good-archive.zip",
		&entries,
		perms,
		"5055257e4d28adea76fc34fdde4e025347405bae",
	)
	if err == nil {
		t.Fatal("expected permissions error, got nil")
	}
}

func TestSucceedsGoodPOM(t *testing.T) {
	perms := readJSON(t, "../../test/fixtures-permissions.json")
	var entries []permissions.Entry
	err := permissions.Verify(
		discardLog,
		"jenkinsci/bom",
		"../../test/fixtures-good-archive.zip",
		&entries,
		perms,
		"5055257e4d28adea76fc34fdde4e025347405bae",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Zip Slip / path traversal tests ---

func TestRejectsPathTraversalDotDot(t *testing.T) {
	// Entry name: ../../etc/passwd
	zipPath := createTraversalZIP(t, "../../etc/passwd")
	perms := map[string][]string{"jenkinsci/bom": {"../../etc"}}
	var entries []permissions.Entry
	err := permissions.Verify(discardLog, "jenkinsci/bom", zipPath, &entries, perms, "aabbccdd1234")
	if err == nil {
		t.Fatal("expected path traversal error, got nil")
	}
	if !strings.Contains(err.Error(), "path traversal") {
		t.Errorf("expected 'path traversal' in error, got: %v", err)
	}
}

func TestRejectsAbsoluteEntryPath(t *testing.T) {
	// Some ZIP writers produce absolute paths like /etc/passwd.
	zipPath := createTraversalZIP(t, "/etc/passwd")
	perms := map[string][]string{"jenkinsci/bom": {"/etc"}}
	var entries []permissions.Entry
	err := permissions.Verify(discardLog, "jenkinsci/bom", zipPath, &entries, perms, "aabbccdd1234")
	if err == nil {
		t.Fatal("expected path traversal error for absolute entry path, got nil")
	}
	if !strings.Contains(err.Error(), "path traversal") {
		t.Errorf("expected 'path traversal' in error, got: %v", err)
	}
}

func TestRejectsNestedTraversal(t *testing.T) {
	// Disguised traversal: a/b/../../../etc/passwd
	zipPath := createTraversalZIP(t, "a/b/../../../etc/passwd")
	perms := map[string][]string{"jenkinsci/bom": {"a/b/../../../etc"}}
	var entries []permissions.Entry
	err := permissions.Verify(discardLog, "jenkinsci/bom", zipPath, &entries, perms, "aabbccdd1234")
	if err == nil {
		t.Fatal("expected path traversal error for nested traversal, got nil")
	}
	if !strings.Contains(err.Error(), "path traversal") {
		t.Errorf("expected 'path traversal' in error, got: %v", err)
	}
}

func TestSucceedsWildcardPath(t *testing.T) {
	perms := readJSON(t, "../../test/fixtures-permissions-wildcard.json")
	var entries []permissions.Entry
	err := permissions.Verify(
		discardLog,
		"jenkinsci/bom",
		"../../test/fixtures-good-archive.zip",
		&entries,
		perms,
		"5055257e4d28adea76fc34fdde4e025347405bae",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
