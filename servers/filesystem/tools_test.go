package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dimasd-angga/go-mcp-servers/shared/testutil"
	"github.com/mark3labs/mcp-go/client"
)

func setup(t *testing.T) (*client.Client, string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("FS_ROOT", dir)
	t.Setenv("FS_MAX_FILE_SIZE", "")
	t.Setenv("FS_ALLOW_DELETE", "")
	fs, err := NewFilesystemServer()
	if err != nil {
		t.Fatal(err)
	}
	c := testutil.NewInProcessClient(t, fs.MCP())
	resolved, _ := filepath.EvalSymlinks(dir)
	return c, resolved
}

func write(t *testing.T, root, name, content string) {
	t.Helper()
	full := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// ---------- read_file ----------

func TestReadFile_Happy(t *testing.T) {
	c, root := setup(t)
	write(t, root, "hi.txt", "hello")
	got := testutil.CallTool(t, c, "read_file", map[string]any{"path": "hi.txt"})
	if got != "hello" {
		t.Errorf("want %q, got %q", "hello", got)
	}
}

func TestReadFile_NotFound(t *testing.T) {
	c, _ := setup(t)
	r := testutil.CallToolRaw(t, c, "read_file", map[string]any{"path": "nope.txt"})
	if !r.IsError {
		t.Error("want IsError for missing file")
	}
}

func TestReadFile_RejectsDirectory(t *testing.T) {
	c, root := setup(t)
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	r := testutil.CallToolRaw(t, c, "read_file", map[string]any{"path": "sub"})
	if !r.IsError {
		t.Error("want IsError for directory")
	}
}

func TestReadFile_PathTraversal(t *testing.T) {
	c, _ := setup(t)
	r := testutil.CallToolRaw(t, c, "read_file", map[string]any{"path": "../../etc/passwd"})
	if !r.IsError {
		t.Error("must reject path traversal")
	}
}

func TestReadFile_TooLarge(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("FS_ROOT", dir)
	t.Setenv("FS_MAX_FILE_SIZE", "10")
	fs, err := NewFilesystemServer()
	if err != nil {
		t.Fatal(err)
	}
	c := testutil.NewInProcessClient(t, fs.MCP())
	write(t, dir, "big.txt", "this is more than ten bytes")
	r := testutil.CallToolRaw(t, c, "read_file", map[string]any{"path": "big.txt"})
	if !r.IsError {
		t.Error("want IsError for oversize file")
	}
}

// ---------- write_file ----------

func TestWriteFile_HappyAndOverwrite(t *testing.T) {
	c, root := setup(t)
	testutil.CallTool(t, c, "write_file", map[string]any{"path": "out.txt", "content": "v1"})
	if b, _ := os.ReadFile(filepath.Join(root, "out.txt")); string(b) != "v1" {
		t.Errorf("want v1, got %q", b)
	}
	testutil.CallTool(t, c, "write_file", map[string]any{"path": "out.txt", "content": "v2"})
	if b, _ := os.ReadFile(filepath.Join(root, "out.txt")); string(b) != "v2" {
		t.Errorf("want v2 after overwrite, got %q", b)
	}
}

func TestWriteFile_CreatesParents(t *testing.T) {
	c, root := setup(t)
	testutil.CallTool(t, c, "write_file", map[string]any{"path": "a/b/c.txt", "content": "deep"})
	if _, err := os.Stat(filepath.Join(root, "a/b/c.txt")); err != nil {
		t.Errorf("parent dirs not created: %v", err)
	}
}

func TestWriteFile_TooLarge(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("FS_ROOT", dir)
	t.Setenv("FS_MAX_FILE_SIZE", "5")
	fs, err := NewFilesystemServer()
	if err != nil {
		t.Fatal(err)
	}
	c := testutil.NewInProcessClient(t, fs.MCP())
	r := testutil.CallToolRaw(t, c, "write_file", map[string]any{"path": "x.txt", "content": "1234567890"})
	if !r.IsError {
		t.Error("want IsError for oversize write")
	}
}

func TestWriteFile_Traversal(t *testing.T) {
	c, _ := setup(t)
	r := testutil.CallToolRaw(t, c, "write_file", map[string]any{"path": "../escape.txt", "content": "x"})
	if !r.IsError {
		t.Error("must reject traversal")
	}
}

// ---------- append_file ----------

func TestAppendFile_HappyAndCreate(t *testing.T) {
	c, root := setup(t)
	testutil.CallTool(t, c, "append_file", map[string]any{"path": "a.txt", "content": "foo"})
	testutil.CallTool(t, c, "append_file", map[string]any{"path": "a.txt", "content": "bar"})
	b, _ := os.ReadFile(filepath.Join(root, "a.txt"))
	if string(b) != "foobar" {
		t.Errorf("want foobar, got %q", b)
	}
}

func TestAppendFile_ExceedsMax(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("FS_ROOT", dir)
	t.Setenv("FS_MAX_FILE_SIZE", "5")
	fs, err := NewFilesystemServer()
	if err != nil {
		t.Fatal(err)
	}
	c := testutil.NewInProcessClient(t, fs.MCP())
	testutil.CallTool(t, c, "append_file", map[string]any{"path": "a.txt", "content": "12345"})
	r := testutil.CallToolRaw(t, c, "append_file", map[string]any{"path": "a.txt", "content": "x"})
	if !r.IsError {
		t.Error("want IsError when append would exceed max")
	}
}

// ---------- delete_file ----------

func TestDeleteFile_DisabledByDefault(t *testing.T) {
	c, root := setup(t)
	write(t, root, "del.txt", "x")
	r := testutil.CallToolRaw(t, c, "delete_file", map[string]any{"path": "del.txt"})
	if !r.IsError {
		t.Error("delete must be disabled by default")
	}
	if _, err := os.Stat(filepath.Join(root, "del.txt")); err != nil {
		t.Error("file should still exist")
	}
}

func TestDeleteFile_EnabledHappy(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("FS_ROOT", dir)
	t.Setenv("FS_ALLOW_DELETE", "true")
	fs, err := NewFilesystemServer()
	if err != nil {
		t.Fatal(err)
	}
	c := testutil.NewInProcessClient(t, fs.MCP())
	write(t, dir, "del.txt", "x")
	testutil.CallTool(t, c, "delete_file", map[string]any{"path": "del.txt"})
	if _, err := os.Stat(filepath.Join(dir, "del.txt")); !os.IsNotExist(err) {
		t.Error("file should be gone")
	}
}

func TestDeleteFile_RefusesDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("FS_ROOT", dir)
	t.Setenv("FS_ALLOW_DELETE", "true")
	fs, err := NewFilesystemServer()
	if err != nil {
		t.Fatal(err)
	}
	c := testutil.NewInProcessClient(t, fs.MCP())
	if err := os.MkdirAll(filepath.Join(dir, "d"), 0o755); err != nil {
		t.Fatal(err)
	}
	r := testutil.CallToolRaw(t, c, "delete_file", map[string]any{"path": "d"})
	if !r.IsError {
		t.Error("must refuse directory delete")
	}
}

// ---------- list_directory ----------

func TestListDirectory_Flat(t *testing.T) {
	c, root := setup(t)
	write(t, root, "a.txt", "x")
	write(t, root, "b.txt", "y")
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := testutil.CallTool(t, c, "list_directory", map[string]any{"path": "."})
	for _, want := range []string{"a.txt", "b.txt", "sub/"} {
		if !strings.Contains(got, want) {
			t.Errorf("want %q in output, got: %s", want, got)
		}
	}
}

func TestListDirectory_Recursive(t *testing.T) {
	c, root := setup(t)
	write(t, root, "x.txt", "x")
	write(t, root, "a/b/c.txt", "y")
	got := testutil.CallTool(t, c, "list_directory", map[string]any{"path": ".", "recursive": true})
	for _, want := range []string{"x.txt", "a/", "a/b/", "a/b/c.txt"} {
		if !strings.Contains(got, want) {
			t.Errorf("want %q in recursive output, got: %s", want, got)
		}
	}
}

func TestListDirectory_RejectsFile(t *testing.T) {
	c, root := setup(t)
	write(t, root, "f.txt", "x")
	r := testutil.CallToolRaw(t, c, "list_directory", map[string]any{"path": "f.txt"})
	if !r.IsError {
		t.Error("want IsError when path is a file")
	}
}

// ---------- create_directory ----------

func TestCreateDirectory_HappyAndIdempotent(t *testing.T) {
	c, root := setup(t)
	testutil.CallTool(t, c, "create_directory", map[string]any{"path": "a/b/c"})
	if info, err := os.Stat(filepath.Join(root, "a/b/c")); err != nil || !info.IsDir() {
		t.Errorf("dir not created: %v", err)
	}
	// Idempotent
	testutil.CallTool(t, c, "create_directory", map[string]any{"path": "a/b/c"})
}

// ---------- move_file ----------

func TestMoveFile_Happy(t *testing.T) {
	c, root := setup(t)
	write(t, root, "src.txt", "data")
	testutil.CallTool(t, c, "move_file", map[string]any{"from": "src.txt", "to": "moved/dst.txt"})
	if _, err := os.Stat(filepath.Join(root, "src.txt")); !os.IsNotExist(err) {
		t.Error("source should be gone")
	}
	b, _ := os.ReadFile(filepath.Join(root, "moved/dst.txt"))
	if string(b) != "data" {
		t.Errorf("dst content wrong: %q", b)
	}
}

func TestMoveFile_RejectsTraversal(t *testing.T) {
	c, root := setup(t)
	write(t, root, "src.txt", "x")
	r := testutil.CallToolRaw(t, c, "move_file", map[string]any{"from": "src.txt", "to": "../out.txt"})
	if !r.IsError {
		t.Error("must reject destination escape")
	}
}

// ---------- search_files ----------

func TestSearchFiles_Glob(t *testing.T) {
	c, root := setup(t)
	write(t, root, "main.go", "x")
	write(t, root, "sub/helper.go", "y")
	write(t, root, "README.md", "z")
	got := testutil.CallTool(t, c, "search_files", map[string]any{"pattern": "*.go"})
	if !strings.Contains(got, "main.go") || !strings.Contains(got, "sub/helper.go") {
		t.Errorf("missing matches: %s", got)
	}
	if strings.Contains(got, "README.md") {
		t.Errorf("non-go file matched: %s", got)
	}
}

func TestSearchFiles_NoMatch(t *testing.T) {
	c, _ := setup(t)
	got := testutil.CallTool(t, c, "search_files", map[string]any{"pattern": "*.nonsense"})
	if !strings.Contains(got, "no files matched") {
		t.Errorf("expected no-match message, got: %s", got)
	}
}

// ---------- get_file_info ----------

func TestGetFileInfo_File(t *testing.T) {
	c, root := setup(t)
	write(t, root, "f.txt", "hello")
	got := testutil.CallTool(t, c, "get_file_info", map[string]any{"path": "f.txt"})
	for _, want := range []string{`"type": "file"`, `"size": 5`} {
		if !strings.Contains(got, want) {
			t.Errorf("want %q in %s", want, got)
		}
	}
}

func TestGetFileInfo_Directory(t *testing.T) {
	c, root := setup(t)
	if err := os.MkdirAll(filepath.Join(root, "d"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := testutil.CallTool(t, c, "get_file_info", map[string]any{"path": "d"})
	if !strings.Contains(got, `"type": "directory"`) {
		t.Errorf("want directory type, got %s", got)
	}
}

// ---------- find_in_files ----------

func TestFindInFiles_Happy(t *testing.T) {
	c, root := setup(t)
	write(t, root, "a.txt", "first line\nneedle here\nthird")
	write(t, root, "sub/b.txt", "no needle\nyes needle in line two")
	got := testutil.CallTool(t, c, "find_in_files", map[string]any{"pattern": "needle"})
	if !strings.Contains(got, "a.txt:2") {
		t.Errorf("want a.txt:2 match, got: %s", got)
	}
	if !strings.Contains(got, "sub/b.txt") {
		t.Errorf("want sub/b.txt match, got: %s", got)
	}
}

func TestFindInFiles_NoMatch(t *testing.T) {
	c, root := setup(t)
	write(t, root, "a.txt", "nothing here")
	got := testutil.CallTool(t, c, "find_in_files", map[string]any{"pattern": "needle"})
	if !strings.Contains(got, "no matches") {
		t.Errorf("expected no-match marker, got: %s", got)
	}
}
