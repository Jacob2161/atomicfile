# Changelog

This module follows semantic versioning. Before v1, a minor version may
change the API; each such change says what callers must do.

## v0.3.0 (2026-11-28)

- Requires Go 1.27, the newest Go release; Go 1.26 is no longer
  supported.
- CI runs staticcheck and govulncheck at the versions pinned in
  `tools/go.mod`, a module of its own that importers never see.

## v0.2.0 (2026-11-27)

This is a breaking change and ships as v0.2.0.

- `WriteFile` creates missing parent directories with mode 0700 instead of
  0755. To keep a directory others can read, create it first with
  `MkdirAll(directory, 0o755)` before calling `WriteFile`.
- CI also checks formatting, `go mod tidy -diff`, and `govulncheck`.

## v0.1.0 (2026-11-27)

First release.

- `WriteFile` replaces a file atomically: it writes a same-directory
  temporary file, syncs it, renames it over the target, and syncs the
  directory. An error wrapping `ErrNotDurable` came after the rename.
- `MkdirAll` creates a directory and its parents and syncs each new entry.
- `SyncDirectory` syncs a directory so created, renamed, and removed
  entries survive a crash.
- Unix-like systems only; on Windows every call returns `ErrUnsupported`.
