package atomicfile_test

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/jacob2161/atomicfile"
)

func ExampleWriteFile() {
	directory, err := os.MkdirTemp("", "atomicfile-example")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(directory)

	path := filepath.Join(directory, "settings.json")
	if err := atomicfile.WriteFile(path, []byte(`{"theme":"dark"}`), 0o600); err != nil {
		log.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(contents))
	// Output: {"theme":"dark"}
}

func BenchmarkWriteFile(b *testing.B) {
	path := filepath.Join(b.TempDir(), "benchmark")
	contents := make([]byte, 4096)
	b.SetBytes(int64(len(contents)))
	for b.Loop() {
		if err := atomicfile.WriteFile(path, contents, 0o600); err != nil {
			b.Fatal(err)
		}
	}
}
