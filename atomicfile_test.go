package atomicfile

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// TestWriteFileCreatesNew checks the shared writer's default file creation behavior.
func TestWriteFileCreatesNew(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := WriteFile(path, []byte("first"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}
	if string(written) != "first" {
		t.Fatalf("contents = %q, want %q", written, "first")
	}
	stat, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if stat.Mode().Perm() != 0o644 {
		t.Fatalf("mode = %v, want 0644", stat.Mode().Perm())
	}
}

// TestWriteFileOverwritesExisting replaces contents and permissions together.
func TestWriteFileOverwritesExisting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := WriteFile(path, []byte("old contents"), 0o644); err != nil {
		t.Fatalf("first WriteFile: %v", err)
	}
	if err := WriteFile(path, []byte("new"), 0o600); err != nil {
		t.Fatalf("second WriteFile: %v", err)
	}
	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}
	if string(written) != "new" {
		t.Fatalf("contents = %q, want %q", written, "new")
	}
	stat, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if stat.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, want 0600", stat.Mode().Perm())
	}
}

// TestWriteFileCreatesParentDirectory creates nested parents for existing callers.
func TestWriteFileCreatesParentDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "deeper", "unit.service")
	if err := WriteFile(path, []byte("body"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}
	if string(written) != "body" {
		t.Fatalf("contents = %q, want %q", written, "body")
	}
}

// TestWriteFileLeavesNoTemporaryFiles removes staging files after a
// successful replacement.
func TestWriteFileLeavesNoTemporaryFiles(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "target.txt")
	if err := WriteFile(path, []byte("done"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatalf("reading directory: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "target.txt" {
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		t.Fatalf("directory holds %v, want only target.txt", names)
	}
}

// TestDurablePrivateWrite preserves private directories and file modes while
// syncing the rename.
func TestDurablePrivateWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "private", "state.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	err := WriteFile(path, []byte(`{"revision":1}`), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	parent, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if parent.Mode().Perm() != 0o700 {
		t.Fatalf("parent mode = %v, want 0700", parent.Mode().Perm())
	}
	file, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if file.Mode().Perm() != 0o600 {
		t.Fatalf("file mode = %v, want 0600", file.Mode().Perm())
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != `{"revision":1}` {
		t.Fatalf("contents = %q, want %q", contents, `{"revision":1}`)
	}
}

// TestFailedReplacementCleansStaging retains the target and removes staging
// files when rename fails.
func TestFailedReplacementCleansStaging(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(directory, "target")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(target, []byte("replacement"), 0o600); err == nil {
		t.Fatal("replacing a directory must fail")
	}
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || !entries[0].IsDir() || entries[0].Name() != "target" {
		t.Fatalf("unexpected directory contents %v", entries)
	}
}

// TestConcurrentReplacementPublishesWholeFiles rejects torn reads while
// multiple writers replace the same path.
func TestConcurrentReplacementPublishesWholeFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state")
	first := bytes.Repeat([]byte("a"), 65_536)
	second := bytes.Repeat([]byte("b"), 65_536)
	var writers sync.WaitGroup
	failures := make(chan error, 2)
	for _, contents := range [][]byte{first, second} {
		writers.Go(func() {
			for range 20 {
				if err := WriteFile(path, contents, 0o600); err != nil {
					failures <- err
					return
				}
			}
			failures <- nil
		})
	}
	for range 100 {
		contents, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(contents, first) && !bytes.Equal(contents, second) {
			t.Fatal("reader received a partial replacement")
		}
	}
	writers.Wait()
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
}

// TestSyncDirectory syncs a directory and reports a missing one.
func TestSyncDirectory(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "entry"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := SyncDirectory(directory); err != nil {
		t.Fatalf("syncing a directory: %v", err)
	}
	if err := SyncDirectory(filepath.Join(directory, "missing")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("syncing a missing directory: %v, want os.ErrNotExist", err)
	}
}

// TestWriteFileCreatesPrivateParents writes under several missing
// directories, all of which are created with mode 0700, so a file written
// with a private mode is not listed to other users.
func TestWriteFileCreatesPrivateParents(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "a", "b", "c", "state.json")
	if err := WriteFile(path, []byte("nested"), 0o600); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil || string(contents) != "nested" {
		t.Fatalf("contents = %q, %v, want %q", contents, err, "nested")
	}
	for _, created := range []string{"a", "a/b", "a/b/c"} {
		directory, err := os.Stat(filepath.Join(root, created))
		if err != nil {
			t.Fatal(err)
		}
		if directory.Mode().Perm() != 0o700 {
			t.Errorf("%s mode = %v, want 0700", created, directory.Mode().Perm())
		}
	}
}

// TestMkdirAll creates nested directories with the given mode and accepts
// ones that exist.
func TestMkdirAll(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "a", "b")
	if err := MkdirAll(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	created, err := os.Stat(directory)
	if err != nil {
		t.Fatal(err)
	}
	if !created.IsDir() || created.Mode().Perm() != 0o700 {
		t.Fatalf("directory mode = %v, want a directory with 0700", created.Mode())
	}
	if err := MkdirAll(directory, 0o700); err != nil {
		t.Fatalf("an existing directory: %v", err)
	}
}

// TestMkdirAllRefusesAFile checks that a regular file is not taken for a
// directory.
func TestMkdirAllRefusesAFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := MkdirAll(path, 0o700); err == nil {
		t.Fatal("a regular file was accepted as a directory")
	}
}
