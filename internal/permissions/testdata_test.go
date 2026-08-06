package permissions_test

// createTraversalZip creates a ZIP in t's temp dir containing a single entry
// whose name contains a path traversal sequence.
// Returns the path to the created file.

import (
	"archive/zip"
	"os"
	"testing"
)

func createTraversalZIP(t *testing.T, entryName string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "traversal-*.zip")
	if err != nil {
		t.Fatalf("creating temp ZIP: %v", err)
	}
	defer f.Close()

	w := zip.NewWriter(f)
	fw, err := w.Create(entryName)
	if err != nil {
		t.Fatalf("creating ZIP entry %q: %v", entryName, err)
	}
	_, _ = fw.Write([]byte("malicious content"))
	if err := w.Close(); err != nil {
		t.Fatalf("closing ZIP writer: %v", err)
	}
	return f.Name()
}
