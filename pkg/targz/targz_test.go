package targz

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/Roshick/manifest-maestro/pkg/filesystem"
	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompressAndExtract_Roundtrip(t *testing.T) {
	ctx := context.Background()

	// Create source filesystem with files
	srcFS := filesystem.New()
	require.NoError(t, srcFS.MkdirAll(srcFS.Join(srcFS.Root, "subdir")))

	writeFile(t, srcFS, srcFS.Join(srcFS.Root, "file1.txt"), "hello world")
	writeFile(t, srcFS, srcFS.Join(srcFS.Root, "subdir", "file2.txt"), "nested content")
	writeFile(t, srcFS, srcFS.Join(srcFS.Root, "subdir", "file3.yaml"), "key: value")

	// Compress
	var buf bytes.Buffer
	err := Compress(ctx, srcFS, srcFS.Root, "", &buf)
	require.NoError(t, err)
	assert.NotEmpty(t, buf.Bytes())

	// Extract
	dstFS := filesystem.New()
	err = Extract(ctx, dstFS, &buf, dstFS.Root)
	require.NoError(t, err)

	// Verify all files exist
	assertFileContent(t, dstFS, dstFS.Join(dstFS.Root, "file1.txt"), "hello world")
	assertFileContent(t, dstFS, dstFS.Join(dstFS.Root, "subdir", "file2.txt"), "nested content")
	assertFileContent(t, dstFS, dstFS.Join(dstFS.Root, "subdir", "file3.yaml"), "key: value")
}

func TestCompressAndExtract_WithSubPath(t *testing.T) {
	ctx := context.Background()

	srcFS := filesystem.New()
	writeFile(t, srcFS, srcFS.Join(srcFS.Root, "data.txt"), "test data")

	// Compress with a target sub-path
	var buf bytes.Buffer
	err := Compress(ctx, srcFS, srcFS.Root, "mychart", &buf)
	require.NoError(t, err)

	// Extract
	dstFS := filesystem.New()
	err = Extract(ctx, dstFS, &buf, dstFS.Root)
	require.NoError(t, err)

	// File should be under the sub-path
	assert.True(t, dstFS.Exists(dstFS.Join(dstFS.Root, "mychart", "data.txt")))
}

func TestCompress_NonExistentSource(t *testing.T) {
	ctx := context.Background()
	srcFS := filesystem.New()

	var buf bytes.Buffer
	err := Compress(ctx, srcFS, srcFS.Join(srcFS.Root, "nonexistent"), "", &buf)
	assert.Error(t, err)
}

func TestExtract_EmptyArchive(t *testing.T) {
	ctx := context.Background()
	dstFS := filesystem.New()

	err := Extract(ctx, dstFS, bytes.NewReader([]byte{}), dstFS.Root)
	assert.Error(t, err)
}

func TestCompressAndExtract_LargerPayload(t *testing.T) {
	ctx := context.Background()

	srcFS := filesystem.New()

	// Create multiple files to simulate a real chart
	files := map[string]string{
		"Chart.yaml":                    "apiVersion: v2\nname: test\nversion: 1.0.0",
		"values.yaml":                   "replicaCount: 1\nimage:\n  tag: latest",
		"templates/deployment.yaml":     "kind: Deployment\napiVersion: apps/v1",
		"templates/service.yaml":        "kind: Service\napiVersion: v1",
		"templates/_helpers.tpl":        "{{- define \"test.name\" -}}test{{- end -}}",
		"templates/tests/test-conn.yaml": "kind: Pod\napiVersion: v1",
	}

	for path, content := range files {
		fullPath := srcFS.Join(srcFS.Root, path)
		dir := srcFS.Dir(fullPath)
		require.NoError(t, srcFS.MkdirAll(dir))
		writeFile(t, srcFS, fullPath, content)
	}

	// Compress
	var buf bytes.Buffer
	err := Compress(ctx, srcFS, srcFS.Root, "", &buf)
	require.NoError(t, err)
	compressedSize := buf.Len()
	t.Logf("Compressed %d files into %d bytes", len(files), compressedSize)

	// Extract
	dstFS := filesystem.New()
	err = Extract(ctx, dstFS, &buf, dstFS.Root)
	require.NoError(t, err)

	// Verify all files
	for path, expectedContent := range files {
		assertFileContent(t, dstFS, dstFS.Join(dstFS.Root, path), expectedContent)
	}
}

func writeFile(t *testing.T, fs *filesystem.FileSystem, path string, content string) {
	t.Helper()
	f, err := fs.Create(path)
	require.NoError(t, err)
	_, err = f.Write([]byte(content))
	require.NoError(t, err)
	require.NoError(t, f.Close())
}

func assertFileContent(t *testing.T, fs *filesystem.FileSystem, path string, expected string) {
	t.Helper()
	assert.True(t, fs.Exists(path), "file should exist: %s", path)
	data, err := fs.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, expected, string(data))
}

func TestCompressFromBilly_Roundtrip(t *testing.T) {
	ctx := context.Background()

	// Create a billy memfs with some files
	billyFS := memfs.New()
	require.NoError(t, billyFS.MkdirAll("subdir", os.ModePerm))

	writeBillyFile(t, billyFS, "file1.txt", "hello billy")
	writeBillyFile(t, billyFS, "subdir/file2.txt", "nested billy content")

	// Compress from billy
	var buf bytes.Buffer
	err := CompressFromBilly(ctx, billyFS, &buf)
	require.NoError(t, err)
	assert.NotEmpty(t, buf.Bytes())

	// Extract into in-memory filesystem
	dstFS := filesystem.New()
	err = Extract(ctx, dstFS, &buf, dstFS.Root)
	require.NoError(t, err)

	// Verify files
	assertFileContent(t, dstFS, dstFS.Join(dstFS.Root, "file1.txt"), "hello billy")
	assertFileContent(t, dstFS, dstFS.Join(dstFS.Root, "subdir", "file2.txt"), "nested billy content")
}

func writeBillyFile(t *testing.T, billyFS billy.Filesystem, path string, content string) {
	t.Helper()
	f, err := billyFS.Create(path)
	require.NoError(t, err)
	_, err = f.Write([]byte(content))
	require.NoError(t, err)
	require.NoError(t, f.Close())
}


