# atomicfile

[![Go Reference](https://pkg.go.dev/badge/github.com/jacob2161/atomicfile.svg)](https://pkg.go.dev/github.com/jacob2161/atomicfile)
[![CI](https://github.com/jacob2161/atomicfile/actions/workflows/ci.yml/badge.svg)](https://github.com/jacob2161/atomicfile/actions/workflows/ci.yml)

Replace a file atomically, so readers see the whole old file or the whole new one and the new contents survive a crash, and sync directories so created, renamed, and removed entries survive one too.

```bash
go get github.com/jacob2161/atomicfile
```

## Usage

```go
if err := atomicfile.WriteFile("/var/lib/app/settings.json", contents, 0o600); err != nil {
	return err
}
```

- `WriteFile` writes a same-directory temporary file, syncs it, renames it over the target, and syncs the directory.
- The file gets exactly the mode you pass, whatever the umask. Missing parent directories are created and synced too, as `MkdirAll` does; create shared hierarchies before concurrent writers start.
- An error wrapping `ErrNotDurable` came after the rename: the new contents are in place but may not survive a crash. Any other error leaves the target untouched.
- `SyncDirectory` is the missing half of most "atomic write" helpers: without it a rename can be lost in a crash.
- Unix-like systems only: on Windows it returns `ErrUnsupported` before changing anything.

Tested on the two newest Go releases. Licensed under the MIT license.
