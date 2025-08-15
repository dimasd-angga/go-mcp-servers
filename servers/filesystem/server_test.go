package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewFilesystemServer_RequiresFSRoot(t *testing.T) {
	t.Setenv("FS_ROOT", "")
	if _, err := NewFilesystemServer(); err == nil {
		t.Fatal("expected error when FS_ROOT unset")
	}
}

func TestNewFilesystemServer_RejectsNonexistentRoot(t *testing.T) {
	t.Setenv("FS_ROOT", "/nonexistent-mcp-root-xyz")
	if _, err := NewFilesystemServer(); err == nil {
		t.Fatal("expected error for nonexistent root")
	}
}

func TestNewFilesystemServer_RejectsFileRoot(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FS_ROOT", file)
	if _, err := NewFilesystemServer(); err == nil {
		t.Fatal("expected error when FS_ROOT is a file")
	}
}

func TestNewFilesystemServer_RejectsInvalidMaxSize(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("FS_ROOT", dir)
	t.Setenv("FS_MAX_FILE_SIZE", "notanumber")
	if _, err := NewFilesystemServer(); err == nil {
		t.Fatal("expected error for non-numeric FS_MAX_FILE_SIZE")
	}
}

func TestNewFilesystemServer_HappyPath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("FS_ROOT", dir)
	t.Setenv("FS_MAX_FILE_SIZE", "")
	t.Setenv("FS_ALLOW_DELETE", "")
	fs, err := NewFilesystemServer()
	if err != nil {
		t.Fatalf("unexpected: %v", err)
	}
	// Symlink resolution may differ; check via filepath.EvalSymlinks of t.TempDir.
	wantRoot, _ := filepath.EvalSymlinks(dir)
	if fs.Root() != wantRoot {
		t.Errorf("root: want %q, got %q", wantRoot, fs.Root())
	}
	if fs.MaxSize() != 10*1024*1024 {
		t.Errorf("default max size wrong: %d", fs.MaxSize())
	}
	if fs.AllowDelete() {
		t.Errorf("delete should default off")
	}
}

func TestSafePath_RejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("FS_ROOT", dir)
	fs, _ := NewFilesystemServer()
	cases := []string{
		"../escape",
		"a/../../escape",
		"../../etc/passwd",
		"a/b/c/../../../../escape",
	}
	for _, p := range cases {
		if _, err := fs.safePath(p); err == nil {
			t.Errorf("expected reject %q", p)
		}
	}
}

func TestSafePath_AbsoluteTreatedAsRoot(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("FS_ROOT", dir)
	fs, _ := NewFilesystemServer()
	got, err := fs.safePath("/foo/bar")
	if err != nil {
		t.Fatal(err)
	}
	want, _ := filepath.EvalSymlinks(dir)
	want = filepath.Join(want, "foo/bar")
	if got != want {
		t.Errorf("want %q, got %q", want, got)
	}
}

func TestSafePath_HappyPath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("FS_ROOT", dir)
	fs, _ := NewFilesystemServer()
	got, err := fs.safePath("sub/file.txt")
	if err != nil {
		t.Fatal(err)
	}
	want, _ := filepath.EvalSymlinks(dir)
	want = filepath.Join(want, "sub/file.txt")
	if got != want {
		t.Errorf("want %q, got %q", want, got)
	}
}

func TestSafePath_PartialPrefixAttack(t *testing.T) {
	dir := t.TempDir()
	// Create a sibling with same prefix
	root := filepath.Join(dir, "root")
	sibling := filepath.Join(dir, "rootevil")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(sibling, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FS_ROOT", root)
	fs, err := NewFilesystemServer()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fs.safePath("../rootevil/secret"); err == nil {
		t.Error("must reject sibling with shared prefix")
	}
}
