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

- **#8 Array append / memory fix (d5/tengo #410)** ✓ done
  Pre-allocation applied across `os`, `text`, and `text_regexp` stdlib modules.

- **Sandbox-safe stdlib** ✓ done
  `sleep` moved from `times` to `os`. `text.sprintf` added as an I/O-free
  alternative to `fmt.sprintf`. `rand.seed()` replaced by `rand.new(seed?)`
  for isolated, non-interfering random state.

## Medium effort

- **#2 Version-based module directory**
  `~/.tengo/modules/<version>/` makes sense once `tengo install` (#3) exists; before that
  it adds friction without benefit. Do after #3.

- **#6 `tengo fmt`** ✓ done
  `format.Format(src)` in the `format` package; available as `tengo fmt`
  subcommand and standalone `tengo-fmt` binary. Tab-indented, spaces around
  operators, comment and blank-line preserving, idempotent.

- **#4 Template module** ✓ done
  `template.text` and `template.html` (inline strings) plus `text_files`/`html_files`
  (glob patterns) in `tengo-modules`.

- **#9 HTTP module** ✓ done
  `http.get`, `http.post`, `http.request` with response map in `tengo-modules`.

- **#15 Crypto module** ✓ done
  `crypto.sha256`, `sha512`, `hmac_sha256` (raw + hex), `aes_encrypt`/`aes_decrypt`
  (AES-GCM) in `tengo-modules`.

- **#11 Module method blacklisting** — reconsidered, won't do
  The sandbox concerns it was meant to address are now handled structurally:
  `sleep` is in `os`, `rand.seed()` is gone, `fmt.print*` can be omitted by
  not registering `fmt`. A generic filter adds complexity without clear benefit.

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
  Tengo can already be embedded in an HTTP handler. Full CGI support now has
  `os.stdin`/`os.stdout`/`os.stderr` (#14 done); next step is a thin CGI wrapper.
  FastCGI is more realistic than classic CGI.

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

- The plugin build constraint (same Go toolchain + same `tengo` version) is an infra
  question; a CI job that builds and publishes matching `tengo` binary + plugin `.so`
  pairs per release is the right long-term answer for #3.
