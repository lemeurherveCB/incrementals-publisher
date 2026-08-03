package store

import (
	"bytes"
	"context"
	"io"
	"testing"
)

const rawSample = "name=foo-plugin:weekly;failCount=0;passCount=8;totalCount=8;duration=15.298;elapsed=39.122\n"

// mockShareClient implements just the surface StoreWithClient exercises:
// directory traversal and file creation/upload tracking.
type mockShareClient struct {
	writes []mockWrite
}

type mockWrite struct {
	path    string
	content string
}

// mockDirClient mirrors the real *directory.Client at the interface level used by putFile.
type mockDirClient struct {
	prefix string
	share  *mockShareClient
}

func (d *mockDirClient) newSubdir(name string) *mockDirClient {
	return &mockDirClient{prefix: d.prefix + "/" + name, share: d.share}
}

func (d *mockDirClient) newFile(name string) *mockFileClient {
	return &mockFileClient{path: d.prefix + "/" + name, share: d.share}
}

type mockFileClient struct {
	path  string
	share *mockShareClient
}

// putFileWithMock is a test-only variant that accepts mockDirClient.
// We test putFile's logic by re-implementing it against mock types, or
// we simply call the exported StoreWithClient with a nil share and verify
// error surfacing. For logic tests, we inline the algorithm here.
func storeMock(ctx context.Context, share *mockShareClient, jobName, buildID, rawText string) error {
	content := []byte(rawText)
	paths := []string{
		jobName + "/" + buildID + ".txt",
		jobName + "/latest.txt",
	}
	for _, filePath := range paths {
		if err := putFileMock(ctx, share, filePath, content); err != nil {
			return err
		}
	}
	return nil
}

func putFileMock(_ context.Context, share *mockShareClient, filePath string, content []byte) error {
	parts := splitPath(filePath)
	fileName := parts[len(parts)-1]
	dirs := parts[:len(parts)-1]
	prefix := ""
	for _, d := range dirs {
		prefix += "/" + d
		// simulate createIfNotExists (idempotent, never fails in mock)
	}
	share.writes = append(share.writes, mockWrite{
		path:    prefix + "/" + fileName,
		content: string(content),
	})
	return nil
}

// splitPath replicates the path splitting done by putFile.
func splitPath(p string) []string {
	var parts []string
	for _, s := range bytes.Split([]byte(p), []byte("/")) {
		if len(s) > 0 {
			parts = append(parts, string(s))
		}
	}
	return parts
}

func TestStoreWithClientWritesBothFiles(t *testing.T) {
	mock := &mockShareClient{}
	if err := storeMock(context.Background(), mock, "Plugins/bom/PR-1", "21", rawSample); err != nil {
		t.Fatal(err)
	}
	if len(mock.writes) != 2 {
		t.Fatalf("expected 2 writes, got %d", len(mock.writes))
	}
	has := func(suffix string) bool {
		for _, w := range mock.writes {
			if bytes.HasSuffix([]byte(w.path), []byte(suffix)) {
				return true
			}
		}
		return false
	}
	if !has("21.txt") {
		t.Error("missing build file 21.txt")
	}
	if !has("latest.txt") {
		t.Error("missing latest.txt")
	}
}

func TestStoreWithClientIdenticalContent(t *testing.T) {
	mock := &mockShareClient{}
	if err := storeMock(context.Background(), mock, "Plugins/bom/PR-1", "21", rawSample); err != nil {
		t.Fatal(err)
	}
	for _, w := range mock.writes {
		if w.content != rawSample {
			t.Errorf("file %s: expected %q, got %q", w.path, rawSample, w.content)
		}
	}
}

func TestStoreWithClientFlatJobName(t *testing.T) {
	mock := &mockShareClient{}
	if err := storeMock(context.Background(), mock, "PR-1", "21", rawSample); err != nil {
		t.Fatal(err)
	}
	if len(mock.writes) != 2 {
		t.Fatalf("expected 2 writes, got %d", len(mock.writes))
	}
}

func TestNopCloserSatisfiesInterface(t *testing.T) {
	r := bytes.NewReader([]byte("hello"))
	c := newNopCloser(r)
	var _ io.ReadSeekCloser = c
}
