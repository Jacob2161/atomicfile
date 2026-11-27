// Package atomicfile replaces files atomically and makes directory entries
// durable.
//
// WriteFile writes a same-directory temporary file, syncs it, and renames it
// over the target, so a reader sees either the whole old file or the whole new
// one, and the new contents survive a crash once WriteFile returns nil.
// SyncDirectory syncs a directory, which is what makes a created, renamed,
// linked, or removed entry in it survive a crash.
//
// Directory syncs need a Unix-like system. On Windows, WriteFile and
// SyncDirectory return ErrUnsupported without changing anything.
package atomicfile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
)

var (
	// ErrNotDurable is wrapped by a WriteFile error that came after the
	// rename: the new contents are in place, but the directory sync that
	// makes the rename survive a crash failed, so after a crash the old
	// contents may return.
	ErrNotDurable = errors.New("replaced, but the directory sync failed")
	// ErrUnsupported means the system cannot sync directories.
	ErrUnsupported = errors.New("directory syncs are not supported on " + runtime.GOOS)
)

// WriteFile replaces the file at path with contents, atomically, and syncs the
// file, its directory, and every directory it created before returning.
//
// The file gets mode, regardless of the process umask. Missing parent
// directories are created as MkdirAll creates them, with mode 0o700 (less the
// umask), so only the owner can list them; a caller that needs a directory
// others can read creates it first with MkdirAll and the mode it wants.
//
// An error that wraps ErrNotDurable came after the rename: the target holds
// the new contents. Any other error leaves the target as it was and removes
// the temporary file.
func WriteFile(path string, contents []byte, mode os.FileMode) (failure error) {
	if runtime.GOOS == "windows" {
		return ErrUnsupported
	}
	if path == "" {
		return errors.New("no path")
	}

	directory := filepath.Dir(path)
	if err := MkdirAll(directory, 0o700); err != nil {
		return err
	}
	temporaryFile, err := os.CreateTemp(directory, "."+filepath.Base(path)+".*")
	if err != nil {
		return fmt.Errorf("creating temporary file in %s: %w", directory, err)
	}
	temporaryPath := temporaryFile.Name()
	renamed := false
	defer func() {
		if renamed {
			return
		}
		if err := os.Remove(temporaryPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			failure = errors.Join(
				failure,
				fmt.Errorf("removing temporary file %s: %w", temporaryPath, err),
			)
		}
	}()
	if _, err := temporaryFile.Write(contents); err != nil {
		return errors.Join(err, temporaryFile.Close())
	}
	if err := temporaryFile.Chmod(mode); err != nil {
		return errors.Join(err, temporaryFile.Close())
	}
	if err := temporaryFile.Sync(); err != nil {
		return errors.Join(err, temporaryFile.Close())
	}
	if err := temporaryFile.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	renamed = true
	if err := SyncDirectory(directory); err != nil {
		return fmt.Errorf("%w: %w", ErrNotDurable, err)
	}
	return nil
}

// MkdirAll creates directory and its missing parents with mode (less the
// umask), as os.MkdirAll does, and syncs the parent of each directory it
// created, so the new entries survive a crash once it returns.
//
// It makes durable what it creates. A directory that already existed is taken
// as durable: when other goroutines or processes may be creating the same
// directories at once, or an earlier MkdirAll failed partway, create the
// hierarchy once, before the writers start, so its creator syncs it.
func MkdirAll(directory string, mode os.FileMode) error {
	if runtime.GOOS == "windows" {
		return ErrUnsupported
	}
	var missing []string
	for current := directory; ; current = filepath.Dir(current) {
		existing, err := os.Stat(current)
		if err == nil {
			if !existing.IsDir() {
				return &os.PathError{Op: "mkdir", Path: current, Err: syscall.ENOTDIR}
			}
			break
		}
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		missing = append(missing, current)
		if parent := filepath.Dir(current); parent == current {
			break
		}
	}
	if len(missing) == 0 {
		return nil
	}
	if err := os.MkdirAll(directory, mode); err != nil {
		return fmt.Errorf("creating directory %s: %w", directory, err)
	}
	// missing runs from the deepest directory up; each one's entry lives in
	// its parent.
	for _, created := range missing {
		if err := SyncDirectory(filepath.Dir(created)); err != nil {
			return fmt.Errorf("syncing the parent of %s: %w", created, err)
		}
	}
	return nil
}

// SyncDirectory syncs a directory, so the entries created, renamed, linked, or
// removed in it are durable. A failure to open, sync, or close it is returned.
func SyncDirectory(directory string) error {
	if runtime.GOOS == "windows" {
		return ErrUnsupported
	}
	handle, err := os.Open(directory)
	if err != nil {
		return err
	}
	return errors.Join(handle.Sync(), handle.Close())
}
