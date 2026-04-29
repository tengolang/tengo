// Package main demonstrates using tar archives as virtual filesystems for
// Tengo module imports via Script.SetImportFS.
//
// A tarFS is built that satisfies fs.FS by indexing all regular files from a
// tar archive into memory on construction. The resulting value is passed
// directly to Script.SetImportFS so that Tengo's import() statement resolves
// modules from the archive rather than from disk.
//
// Two scenarios are shown back-to-back:
//
//  1. Single tar — one archive holds all modules.
//  2. Two tars   — modules are split across two archives and combined with
//     tengo.MultiFS, which tries each FS in order. The first archive takes
//     precedence when both contain the same path.
package main

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"path"
	"strings"
	"time"

	"github.com/tengolang/tengo/v3"
	"github.com/tengolang/tengo/v3/stdlib"
)

// ---------------------------------------------------------------------------
// tarFS — a read-only fs.FS backed by an in-memory snapshot of a tar archive
// ---------------------------------------------------------------------------

// tarFS indexes all regular files from a tar archive into memory so that
// arbitrary Open calls can be served without re-reading the archive.
type tarFS struct {
	files map[string][]byte // normalised VFS path → raw bytes
}

// newTarFS drains r, records every regular file entry, and returns a tarFS.
// Loading the entire archive upfront is the simplest approach because tar is a
// sequential format and fs.FS requires random-access Open semantics.
//
// For very large archives, consider keeping only an index of byte offsets and
// seeking through an io.ReadSeeker on demand instead.
func newTarFS(r io.Reader) (*tarFS, error) {
	t := &tarFS{files: make(map[string][]byte)}
	tr := tar.NewReader(r)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading tar header: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue // skip directories and other special entries
		}

		data, err := io.ReadAll(tr)
		if err != nil {
			return nil, fmt.Errorf("reading tar entry %q: %w", hdr.Name, err)
		}

		// Normalise the path: clean it, then strip any leading "./" that tar
		// tools sometimes add (e.g. "./lib/add.tengo" → "lib/add.tengo").
		name := strings.TrimPrefix(path.Clean(hdr.Name), "./")
		t.files[name] = data
	}

	return t, nil
}

// Open implements fs.FS. Only exact file paths and the root "." are handled;
// sub-directory paths without a matching file return fs.ErrNotExist.
func (t *tarFS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}

	// fs.FS requires Open(".") to succeed and return something directory-like.
	if name == "." {
		return &tarDir{name: "."}, nil
	}

	data, ok := t.files[name]
	if !ok {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	return &tarFile{
		name:   path.Base(name),
		reader: bytes.NewReader(data),
		size:   int64(len(data)),
	}, nil
}

// ---------------------------------------------------------------------------
// tarFile — fs.File for a regular file held in memory
// ---------------------------------------------------------------------------

type tarFile struct {
	name   string
	reader *bytes.Reader
	size   int64
}

func (f *tarFile) Read(b []byte) (int, error) { return f.reader.Read(b) }
func (f *tarFile) Close() error               { return nil }
func (f *tarFile) Stat() (fs.FileInfo, error) {
	return &tarFileInfo{name: f.name, size: f.size, isDir: false}, nil
}

// ---------------------------------------------------------------------------
// tarDir — minimal fs.File for a directory (only "." is ever returned)
// ---------------------------------------------------------------------------

type tarDir struct{ name string }

func (d *tarDir) Read([]byte) (int, error) { return 0, io.EOF }
func (d *tarDir) Close() error             { return nil }
func (d *tarDir) Stat() (fs.FileInfo, error) {
	return &tarFileInfo{name: d.name, isDir: true}, nil
}

// ---------------------------------------------------------------------------
// tarFileInfo — fs.FileInfo for both files and the root directory
// ---------------------------------------------------------------------------

type tarFileInfo struct {
	name  string
	size  int64
	isDir bool
}

func (i *tarFileInfo) Name() string { return i.name }
func (i *tarFileInfo) Size() int64  { return i.size }
func (i *tarFileInfo) Mode() fs.FileMode {
	if i.isDir {
		return fs.ModeDir | 0o555
	}
	return 0o444
}
func (i *tarFileInfo) ModTime() time.Time { return time.Time{} }
func (i *tarFileInfo) IsDir() bool        { return i.isDir }
func (i *tarFileInfo) Sys() any           { return nil }

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// buildTarBytes writes the given name/body pairs into a tar archive and
// returns the raw bytes. In a real application you would load the archive from
// disk instead:
//
//	data, err := os.ReadFile("modules.tar")
//	vfs, err  := newTarFS(bytes.NewReader(data))
func buildTarBytes(entries []struct{ name, body string }) ([]byte, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, e := range entries {
		if err := tw.WriteHeader(&tar.Header{
			Name: e.name,
			Mode: 0o644,
			Size: int64(len(e.body)),
		}); err != nil {
			return nil, err
		}
		if _, err := tw.Write([]byte(e.body)); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func mustTarFS(entries []struct{ name, body string }) *tarFS {
	data, err := buildTarBytes(entries)
	if err != nil {
		panic(fmt.Errorf("building tar: %w", err))
	}
	vfs, err := newTarFS(bytes.NewReader(data))
	if err != nil {
		panic(fmt.Errorf("indexing tar: %w", err))
	}
	return vfs
}

// runScript compiles and runs src with the given fs.FS, printing the result.
func runScript(src string, fsys fs.FS) {
	s := tengo.NewScript([]byte(src))
	s.SetImports(stdlib.GetModuleMap(stdlib.AllModuleNames()...))
	s.SetImportFS(fsys)
	if _, err := s.Run(); err != nil {
		fmt.Println("script error:", err)
	}
}

// ---------------------------------------------------------------------------
// main
// ---------------------------------------------------------------------------

func main() {
	// -----------------------------------------------------------------------
	// Scenario 1: single tar archive
	//
	// All three modules live in one archive. lib/mul.tengo uses a relative
	// import ("./add") which resolves to lib/add.tengo inside the same archive.
	// -----------------------------------------------------------------------
	fmt.Println("=== single tar ===")

	singleVFS := mustTarFS([]struct{ name, body string }{
		{
			"greet.tengo",
			`export func(name) { return "hello, " + name }`,
		},
		{
			"lib/add.tengo",
			`export func(a, b) { return a + b }`,
		},
		{
			// Relative import: import directory is set to "lib" when this file
			// is compiled, so import("./add") resolves to lib/add.tengo.
			"lib/mul.tengo",
			`add := import("./add")
export func(a, b) {
    result := 0
    for i := 0; i < b; i++ { result = add(result, a) }
    return result
}`,
		},
	})

	runScript(`
fmt   := import("fmt")
greet := import("greet")    // from the single archive
add   := import("lib/add")  // subdirectory module
mul   := import("lib/mul")  // uses import("./add") internally

fmt.println(greet("world"))  // hello, world
fmt.println(add(3, 4))       // 7
fmt.println(mul(6, 7))       // 42
`, singleVFS)

	// -----------------------------------------------------------------------
	// Scenario 2: two tar archives combined with MultiFS
	//
	// The base archive holds the core library (lib/add, lib/mul). The overlay
	// archive holds a different greet module that overrides the base's greet.
	// tengo.MultiFS tries the overlay first, so its greet.tengo wins.
	// -----------------------------------------------------------------------
	fmt.Println("\n=== two tars via MultiFS ===")

	baseVFS := mustTarFS([]struct{ name, body string }{
		{
			// This greet will be shadowed by the overlay.
			"greet.tengo",
			`export func(name) { return "hi, " + name }`,
		},
		{
			"lib/add.tengo",
			`export func(a, b) { return a + b }`,
		},
		{
			"lib/mul.tengo",
			`add := import("./add")
export func(a, b) {
    result := 0
    for i := 0; i < b; i++ { result = add(result, a) }
    return result
}`,
		},
	})

	overlayVFS := mustTarFS([]struct{ name, body string }{
		{
			// Same path as in baseVFS; MultiFS gives this priority because the
			// overlay is listed first.
			"greet.tengo",
			`export func(name) { return "hello, " + name }`,
		},
	})

	// overlayVFS is searched first; baseVFS fills in everything else.
	runScript(`
fmt   := import("fmt")
greet := import("greet")    // resolved from overlayVFS (takes precedence)
add   := import("lib/add")  // resolved from baseVFS
mul   := import("lib/mul")  // resolved from baseVFS, uses relative ./add

fmt.println(greet("world"))  // hello, world  (from overlay, not "hi, world")
fmt.println(add(3, 4))       // 7
fmt.println(mul(6, 7))       // 42
`, tengo.MultiFS{overlayVFS, baseVFS})
}
