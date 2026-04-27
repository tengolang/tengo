# Tengo Roadmap

## Quick wins

- **#1 Go version in `-version` output** ✓ done
  `runtime.Version()` included in all version string variants.

- **#14 stdin / stdout / stderr in the `os` module** ✓ done
  Added as `os.stdin`, `os.stdout`, `os.stderr` objects backed by `makeOSFile`.

- **#7 Sort module in stdlib** ✓ done
  `ints`, `floats`, `strings`, `reverse`, and `by(arr, fn)` added to stdlib.
  Also fixed a VM bug: `InteropFunction` calls that re-enter the VM via
  `RunCompiledFunction` were corrupting the stack (hoisted `sp`/`ip` locals
  not flushed before dispatch).

- **#13 DRY argument parsing** ✓ done
  `ArgCount`, `ArgString`, `ArgInt`, `ArgFloat`, `ArgBool`, `ArgBytes`, `ArgTime`
  exported from the `tengo` package in `args.go`.

## Medium effort

- **#2 Version-based module directory**
  `~/.tengo/modules/<version>/` makes sense once `tengo install` (#3) exists; before that
  it adds friction without benefit. Do after #3.

- **#6 `tengo fmt`**
  Requires the parser to emit a position-preserving AST and a printer. Non-trivial but the
  parser already tracks positions. Natural fit as a separate binary alongside `tengo-man`.

- **#4 Template module**
  Go's `text/template` and `html/template` as a plugin module in `tengo-modules`. Clean fit.

- **#9 HTTP module**
  Plugin module in `tengo-modules`. Start with an HTTP client; server-side needs more design.

- **#8 Array append / memory fix (d5/tengo #410)** ✓ done
  Pre-allocation applied across `os`, `text`, and `text_regexp` stdlib modules.

- **#15 Crypto module**
  Plugin module in `tengo-modules`: sha256, hmac, aes. Clearly not stdlib material,
  clearly useful.

- **#11 Module method blacklisting**
  A `FilteredModule` wrapper that strips specific keys from a `map[string]Object` before
  registration. Implementable without VM changes and directly useful for sandboxing.

## Needs design work

- **#3 `tengo install`**
  A package manager: manifest format, version resolution, arch/os handling for `.so` plugins
  (source modules need none of that). Significant standalone project but unlocks everything else.

- **#10 Memory allocation limit**
  `maxAllocs` counts VM object allocations, not bytes. Real byte-level limits would require
  hooking into the GC or a custom allocator. Not straightforward in Go.

- **#16 Error handling**
  d5 was right that try/catch adds overhead and Go-style error values are preferred. The
  existing `error` type + `is_error()` pattern works but is verbose. A `try(fn)` builtin
  that recovers panics may be the smallest useful addition. Related: d5/tengo #161.

- **#12 True goroutine concurrency**
  d5 rejected a PR due to ~10% overhead on the normal (non-concurrent) path from locking.
  The `coro` module covers cooperative concurrency. True goroutine concurrency is a large
  change with ongoing maintenance cost; low priority unless a concrete use case drives it.

- **#5 Server-side scripting / CGI**
  Tengo can already be embedded in an HTTP handler. Full CGI support needs #14
  (stdin/stdout/stderr) first, then a thin CGI wrapper. FastCGI is more realistic than
  classic CGI.

## Separate projects

- **#17 Language Server (LSP)**
  A real project, probably `tengolang/tengo-lsp`. Requires type inference that does not
  currently exist.

- **A Syntax highlighting**
  A `tengolang/tengo-syntax` repo with a TextMate grammar (covers VS Code, Zed, GitHub
  rendering). A VS Code extension already exists externally but is unmaintained.

- **#18 Language specification**
  A formal spec document. The tutorial rewrite and `tengo-man` have improved the
  situation, but a proper specification is a separate effort.

- **#19 d5/tengo issue #202**
  To be triaged.

## Notes

- **#11 method blacklisting** and **#14 stdin/stdout/stderr** are the most immediately
  relevant for anyone running Tengo in a sandboxed / user-script context.
- The plugin build constraint (same Go toolchain + same `tengo` version) is an infra
  question; a CI job that builds and publishes matching `tengo` binary + plugin `.so`
  pairs per release is the right long-term answer for #3.
